package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	ddg "github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
	openailibs "github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	callbackhelper "github.com/cloudwego/eino/utils/callbacks"
	"github.com/samber/lo"
	goopenai "github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/rag"
	"github.com/quka-ai/quka-ai/pkg/ai/tools/duckduckgo"
	"github.com/quka-ai/quka-ai/pkg/mark"
	"github.com/quka-ai/quka-ai/pkg/safe"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// AutoAssistant 基于 eino 框架的智能助手
// 保持与 NormalAssistant 相同的接口，但内部使用 eino ReAct Agent
type AutoAssistant struct {
	core      *core.Core
	agentType string
}

// NewAutoAssistant 创建新的 Auto Assistant 实例
func NewAutoAssistant(core *core.Core, agentType string) *AutoAssistant {
	return &AutoAssistant{
		core:      core,
		agentType: agentType,
	}
}

// InitAssistantMessage 初始化助手消息 - 与 NormalAssistant 保持兼容
func (a *AutoAssistant) InitAssistantMessage(ctx context.Context, msgID string, seqID int64, userReqMessage *types.ChatMessage, ext types.ChatMessageExt) (*types.ChatMessage, error) {
	// 直接调用 ai.go 中的 initAssistantMessage 函数
	return initAssistantMessage(ctx, a.core, msgID, seqID, userReqMessage, ext)
}

// GenSessionContext 生成会话上下文 - 与 NormalAssistant 保持兼容
func (a *AutoAssistant) GenSessionContext(ctx context.Context, prompt string, reqMsgWithDocs *types.ChatMessage) (*SessionContext, error) {
	// 直接调用 ai.go 中的函数
	return GenChatSessionContextAndSummaryIfExceedsTokenLimit(ctx, a.core, prompt, reqMsgWithDocs, normalGenMessageCondition, types.GEN_CONTEXT)
}

// RequestAssistant 基于 eino 框架的智能助手请求处理
// 实现与 NormalAssistant 相同的接口，但内部使用 eino ReAct Agent
func (a *AutoAssistant) RequestAssistant(ctx context.Context, reqMsg *types.ChatMessage, receiver types.Receiver, aiCallOptions *types.AICallOptions) error {
	// 1. 获取空间信息
	space, err := a.core.Store().SpaceStore().GetSpace(ctx, reqMsg.SpaceID)
	if err != nil {
		return err
	}

	// 2. 准备提示词
	var prompt string
	if aiCallOptions.Docs == nil || len(aiCallOptions.Docs.Refs) == 0 {
		aiCallOptions.Docs = &types.RAGDocs{}
		prompt = lo.If(space.BasePrompt != "", space.BasePrompt).Else(ai.GENERATE_PROMPT_TPL_NONE_CONTENT_CN)
	} else {
		prompt = ai.BuildRAGPrompt(ai.GENERATE_PROMPT_TPL_CN, ai.NewDocs(aiCallOptions.Docs.Docs), a.core.Srv().AI())
	}

	// 3. 生成会话上下文
	sessionContext, err := a.GenSessionContext(ctx, prompt, reqMsg)
	if err != nil {
		return handleAndNotifyAssistantFailed(a.core, receiver, reqMsg, err)
	}

	// 4. 创建 AgentContext - 提取思考和搜索配置
	enableThinking := aiCallOptions.EnableThinking
	enableWebSearch := aiCallOptions.EnableSearch

	agentCtx := types.NewAgentContextWithOptions(
		ctx,
		reqMsg.SpaceID,
		reqMsg.UserID,
		reqMsg.SessionID,
		reqMsg.ID,
		enableThinking,
		enableWebSearch,
	)

	// 5. 直接将 MessageContext 转换为 eino 消息格式
	einoMessages := make([]*schema.Message, 0, len(sessionContext.MessageContext))
	for _, msgCtx := range sessionContext.MessageContext {
		einoMsg := &schema.Message{
			Content: msgCtx.Content,
		}

		// 转换角色
		switch msgCtx.Role {
		case types.USER_ROLE_SYSTEM:
			einoMsg.Role = schema.System
		case types.USER_ROLE_USER:
			einoMsg.Role = schema.User
		case types.USER_ROLE_ASSISTANT:
			einoMsg.Role = schema.Assistant
		case types.USER_ROLE_TOOL:
			einoMsg.Role = schema.Tool
		default:
			einoMsg.Role = schema.User
		}

		// 处理多媒体内容
		if len(msgCtx.MultiContent) > 0 {
			einoMsg.MultiContent = make([]schema.ChatMessagePart, len(msgCtx.MultiContent))
			for i, part := range msgCtx.MultiContent {
				einoMsg.MultiContent[i] = schema.ChatMessagePart{
					Type: schema.ChatMessagePartType(part.Type),
					Text: part.Text,
				}

				// 转换 ImageURL
				if part.ImageURL != nil {
					einoMsg.MultiContent[i].ImageURL = &schema.ChatMessageImageURL{
						URL:    part.ImageURL.URL,
						Detail: schema.ImageURLDetail(part.ImageURL.Detail),
					}
				}
			}
		}

		einoMessages = append(einoMessages, einoMsg)
	}

	// 6. 🔥 创建 Eino 消息生命周期回调管理器（替代直接的消息初始化）
	lifecycleCallback := NewEinoMessageLifecycleCallback(a.core, reqMsg, types.ChatMessageExt{}, receiver)

	// 7. 🔥 创建工具调用生命周期回调管理器
	toolCallback := NewEinoToolLifecycleCallback(a.core, reqMsg.SessionID, reqMsg.SpaceID, reqMsg.UserID)

	// 保留原有的 receiveFunc 和 doneFunc 作为备用（将在 lifecycleCallback 中使用）
	receiveFunc := receiver.GetReceiveFunc()
	doneFunc := receiver.GetDoneFunc(func(recvMsgInfo *types.ChatMessage) {
		if recvMsgInfo == nil {
			return
		}
		// set chat session pin
		go safe.Run(func() {
			if len(aiCallOptions.Docs.Refs) == 0 {
				return
			}
			if err := createChatSessionKnowledgePin(a.core, recvMsgInfo, aiCallOptions.Docs); err != nil {
				slog.Error("Failed to create chat session knowledge pins", slog.String("session_id", recvMsgInfo.SessionID), slog.String("error", err.Error()))
			}
		})
	})

	// 🔥 关键：使用增强的适配器支持工具调用记录
	enhancedAdapter := NewEnhancedEinoAdapter(receiveFunc, reqMsg.SessionID, reqMsg.ID)

	// 7. 创建 Agent（使用增强适配器）
	factory := NewEinoAgentFactory(a.core)
	agent, modelConfig, err := factory.CreateReActAgent(agentCtx, enhancedAdapter.EinoAdapter, einoMessages)
	if err != nil {
		return handleAndNotifyAssistantFailed(a.core, receiver, reqMsg, err)
	}

	// 8. 执行推理并处理响应

	// 构建 marks 映射（用于特殊语法处理）
	marks := make(map[string]string)
	if aiCallOptions != nil && aiCallOptions.Docs != nil {
		for _, v := range aiCallOptions.Docs.Docs {
			if v.SW == nil {
				continue
			}
			for fake, real := range v.SW.Map() {
				marks[fake] = real
			}
		}
	}

	// 创建响应处理器（传入数据库写入函数）
	responseHandler := NewEinoResponseHandler(receiveFunc, doneFunc, enhancedAdapter.EinoAdapter, marks)

	// 9. 🔥 使用增强的 Eino Callback Handlers（集成消息和工具生命周期管理）
	callbackHandler := NewEnhancedEinoCallbackHandlers(
		modelConfig.ModelName,
		reqMsg.ID,
		lifecycleCallback,
		toolCallback,
		responseHandler,
	)

	// 10. 执行推理
	if receiver.IsStream() {
		// 流式处理
		return a.handleStreamResponse(agentCtx, agent, einoMessages, callbackHandler)
	} else {
		// 非流式处理
		return a.handleDirectResponse(agentCtx, agent, einoMessages, responseHandler, doneFunc)
	}
}

type UsageAndReasoningColumns struct {
	ResponseMeta schema.ResponseMeta `json:"response_meta"`
}

type ReasoningContent struct {
	Extra struct {
		ReasoningContent string `json:"reasoning-content"` // 推理内容
	} `json:"extra"`
}

// handleStreamResponse 处理流式响应
func (a *AutoAssistant) handleStreamResponse(ctx context.Context, reactAgent *react.Agent, messages []*schema.Message, callbacksHandler callbacks.Handler) error {
	// 使用 eino agent 进行流式推理
	result, err := reactAgent.Stream(ctx, messages, agent.WithComposeOptions(
		// compose.WithCallbacks()
		// compose.WithCallbacks(&LoggerCallback{}),
		compose.WithCallbacks(callbacksHandler, &LoggerCallback{}),
	))
	if err != nil {
		slog.Error("failed to start eino stream response", slog.Any("error", err))
		return err
	}

	defer result.Close()

	for {
		_, err := result.Recv()
		if err != nil {
			break
		}
	}
	return nil
}

// handleDirectResponse 处理非流式响应
func (a *AutoAssistant) handleDirectResponse(ctx context.Context, agent *react.Agent, messages []*schema.Message, handler *EinoResponseHandler, done types.DoneFunc) error {
	// 使用 eino agent 进行推理
	result, err := agent.Generate(ctx, messages)
	if err != nil {
		if done != nil {
			done(err)
		}
		return err
	}

	if err = handler.receiveFunc(&types.TextMessage{Text: result.Content}, types.MESSAGE_PROGRESS_GENERATING); err != nil {
		return err
	}

	// 完成处理
	if done != nil {
		return done(nil)
	}
	return nil
}

// EinoMessageConverter 消息转换器，处理 schema.Message 和数据库记录的转换
type EinoMessageConverter struct {
	core *core.Core
}

// NewEinoMessageConverter 创建消息转换器
func NewEinoMessageConverter(core *core.Core) *EinoMessageConverter {
	return &EinoMessageConverter{core: core}
}

// ConvertFromChatMessages 将数据库中的 ChatMessage 转换为 eino schema.Message
func (c *EinoMessageConverter) ConvertFromChatMessages(chatMessages []types.ChatMessage) []*schema.Message {
	messages := make([]*schema.Message, 0, len(chatMessages))

	for _, msg := range chatMessages {
		einoMsg := &schema.Message{
			Content: msg.Message,
		}

		// 转换角色
		switch msg.Role {
		case types.USER_ROLE_SYSTEM:
			einoMsg.Role = schema.System
		case types.USER_ROLE_USER:
			einoMsg.Role = schema.User
		case types.USER_ROLE_ASSISTANT:
			einoMsg.Role = schema.Assistant
		case types.USER_ROLE_TOOL:
			einoMsg.Role = schema.Tool
		default:
			einoMsg.Role = schema.User
		}

		// 处理多媒体内容 - 从 ChatMessageAttach 转换为 schema.ChatMessagePart
		if len(msg.Attach) > 0 {
			// 使用 core 中的文件存储服务来下载文件
			multiContent := msg.Attach.ToMultiContent(msg.Message, c.core.FileStorage())
			einoMsg.MultiContent = c.convertToEinoMultiContent(multiContent)
		}

		messages = append(messages, einoMsg)
	}

	return messages
}

// ConvertToChatMessage 将 schema.Message 转换为数据库 ChatMessage 格式（用于持久化）
func (c *EinoMessageConverter) ConvertToChatMessage(msg *schema.Message, sessionID, spaceID, userID string) *types.ChatMessage {
	chatMsg := &types.ChatMessage{
		SessionID: sessionID,
		SpaceID:   spaceID,
		UserID:    userID,
		Message:   msg.Content,
		SendTime:  time.Now().Unix(),
		Complete:  types.MESSAGE_PROGRESS_COMPLETE,
	}

	// 转换角色
	switch msg.Role {
	case schema.System:
		chatMsg.Role = types.USER_ROLE_SYSTEM
	case schema.User:
		chatMsg.Role = types.USER_ROLE_USER
	case schema.Assistant:
		chatMsg.Role = types.USER_ROLE_ASSISTANT
	case schema.Tool:
		chatMsg.Role = types.USER_ROLE_TOOL
	default:
		chatMsg.Role = types.USER_ROLE_USER
	}

	// 根据角色设置消息类型
	switch msg.Role {
	case schema.Tool:
		chatMsg.MsgType = types.MESSAGE_TYPE_TOOL_TIPS
	default:
		chatMsg.MsgType = types.MESSAGE_TYPE_TEXT
	}

	return chatMsg
}

// convertToEinoMultiContent 将 goopenai.ChatMessagePart 转换为 schema.ChatMessagePart
func (c *EinoMessageConverter) convertToEinoMultiContent(openaiParts []goopenai.ChatMessagePart) []schema.ChatMessagePart {
	einoParts := make([]schema.ChatMessagePart, len(openaiParts))

	for i, part := range openaiParts {
		einoParts[i] = schema.ChatMessagePart{
			Type: schema.ChatMessagePartType(part.Type),
			Text: part.Text,
		}

		// 转换 ImageURL
		if part.ImageURL != nil {
			einoParts[i].ImageURL = &schema.ChatMessageImageURL{
				URL:    part.ImageURL.URL,
				Detail: schema.ImageURLDetail(part.ImageURL.Detail),
			}
		}

		// TODO: 如果需要支持其他多媒体类型（音频、视频、文件），可以在这里添加
		// 例如：
		// if part.AudioURL != nil {
		//     einoParts[i].AudioURL = &schema.ChatMessageAudioURL{...}
		// }
	}

	return einoParts
}

// === 工具内部通知机制 ===

// NotifyingTool 工具包装器，在工具执行前后发送实时通知
type NotifyingTool struct {
	tool.InvokableTool
	core    *core.Core
	adapter *ai.EinoAdapter
	toolID  string // 为每个工具实例分配唯一ID
}

// NewNotifyingTool 创建带通知功能的工具包装器
func NewNotifyingTool(baseTool tool.InvokableTool, adapter *ai.EinoAdapter) *NotifyingTool {
	return &NotifyingTool{
		InvokableTool: baseTool,
		adapter:       adapter,
		toolID:        utils.GenUniqIDStr(), // 生成唯一的工具实例ID
	}
}

// InvokableRun 执行工具调用，并在执行前后发送通知
func (nt *NotifyingTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	fmt.Println("Invoked tool:", argumentsInJSON)
	// 获取工具信息
	toolInfo, err := nt.InvokableTool.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get tool info: %w", err)
	}
	toolName := toolInfo.Name

	// 🔥 工具调用开始通知
	if err := nt.adapter.OnToolCallStart(toolName, map[string]interface{}{
		"input":   argumentsInJSON,
		"tool_id": nt.toolID,
	}); err != nil {
		slog.Error("Failed to send tool start notification",
			slog.String("tool", toolName),
			slog.String("tool_id", nt.toolID),
			slog.Any("error", err))
	}

	// 执行实际工具
	startTime := time.Now()
	result, err := nt.InvokableTool.InvokableRun(ctx, argumentsInJSON, opts...)
	duration := time.Since(startTime)

	// 🔥 工具调用结束通知
	if err != nil {
		nt.adapter.OnToolCallEnd(toolName, map[string]interface{}{
			"error":    err.Error(),
			"duration": duration.String(),
			"tool_id":  nt.toolID,
		}, err)
	} else {
		nt.adapter.OnToolCallEnd(toolName, map[string]interface{}{
			"result":   result,
			"duration": duration.String(),
			"tool_id":  nt.toolID,
		}, nil)
	}

	return result, err
}

// === 增强的 eino 适配器，支持实时工具调用通知 ===

// EnhancedEinoAdapter 增强的 eino 适配器，支持实时工具调用通知
type EnhancedEinoAdapter struct {
	*ai.EinoAdapter
}

// NewEnhancedEinoAdapter 创建增强的 eino 适配器
func NewEnhancedEinoAdapter(receiveFunc types.ReceiveFunc, sessionID, messageID string) *EnhancedEinoAdapter {
	baseAdapter := ai.NewEinoAdapter(receiveFunc, sessionID, messageID)

	return &EnhancedEinoAdapter{
		EinoAdapter: baseAdapter,
	}
}

func (e *EnhancedEinoAdapter) extractToolName(info *callbacks.RunInfo) string {
	if info != nil && info.Name != "" {
		return info.Name
	}
	return "unknown_tool"
}

// EinoAgentFactory 创建和配置 eino Agent 的工厂
type EinoAgentFactory struct {
	core *core.Core
	// 模型配置缓存，避免重复查库
	cachedChatModelConfig   *types.ModelConfig
	cachedVisionModelConfig *types.ModelConfig
}

// NewEinoAgentFactory 创建 Agent 工厂
func NewEinoAgentFactory(core *core.Core) *EinoAgentFactory {
	return &EinoAgentFactory{core: core}
}

// CreateReActAgent 创建 ReAct Agent 实例
func (f *EinoAgentFactory) CreateReActAgent(agentCtx *types.AgentContext, adapter *ai.EinoAdapter, messages []*schema.Message) (*react.Agent, *types.ModelConfig, error) {
	// 检查消息中是否包含多媒体内容，决定使用哪种模型
	needVisionModel := f.containsMultimediaContent(messages)

	var modelConfig *types.ModelConfig
	var err error

	if needVisionModel {
		// 获取视觉模型配置
		modelConfig, err = f.getVisionModelConfig(agentCtx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get vision model config: %w", err)
		}
	} else {
		// 获取聊天模型配置
		modelConfig, err = f.getChatModelConfig(agentCtx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get chat model config: %w", err)
		}
	}

	chatModel, err := f.GetToolCallingModel(agentCtx, modelConfig)
	if err != nil {
		return nil, nil, err
	}

	// 创建工具配置
	tools, err := f.createTools(agentCtx, adapter)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create tools: %w", err)
	}

	toolsConfig := compose.ToolsNodeConfig{
		Tools: tools,
	}

	// 创建 ReAct Agent
	agentConfig := &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      toolsConfig,
		// 🔥 禁用 MessageModifier，改为使用工具内部通知机制
		MessageModifier: nil,
		StreamToolCallChecker: func(ctx context.Context, modelOutput *schema.StreamReader[*schema.Message]) (bool, error) {
			defer modelOutput.Close()
			for {
				res, err := modelOutput.Recv()
				if err != nil {
					return false, nil
				}

				if res.ResponseMeta.FinishReason == "tool_calls" {
					return true, nil
				}
			}
		},
	}

	agent, err := react.NewAgent(agentCtx, agentConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create ReAct agent: %w", err)
	}

	return agent, modelConfig, nil
}

func (f *EinoAgentFactory) GetToolCallingModel(agentCtx *types.AgentContext, modelConfig *types.ModelConfig) (model.ToolCallingChatModel, error) {
	if strings.Contains(strings.ToLower(modelConfig.ModelName), "qwen") {
		// 创建 OpenAI 模型
		chatModel, err := qwen.NewChatModel(agentCtx, &qwen.ChatModelConfig{
			APIKey:         modelConfig.Provider.ApiKey,
			BaseURL:        modelConfig.Provider.ApiUrl,
			Model:          modelConfig.ModelName,
			Timeout:        5 * time.Minute,
			EnableThinking: &agentCtx.EnableThinking,
		})
		if err != nil {
			// 如果创建失败，尝试使用 goopenai 库创建模型
			return nil, fmt.Errorf("failed to create qwen chat model: %w", err)
		}
		return chatModel, nil
	} else if strings.Contains(strings.ToLower(modelConfig.ModelName), "deepseek") {
		chatModel, err := deepseek.NewChatModel(agentCtx, &deepseek.ChatModelConfig{
			APIKey:  modelConfig.Provider.ApiKey,
			BaseURL: modelConfig.Provider.ApiUrl,
			Model:   modelConfig.ModelName,
			Timeout: 5 * time.Minute,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create deepseek chat model: %w", err)
		}
		return chatModel, nil
	}

	// 创建 OpenAI 模型
	chatModel, err := openai.NewChatModel(agentCtx, &openai.ChatModelConfig{
		APIKey:  modelConfig.Provider.ApiKey,
		BaseURL: modelConfig.Provider.ApiUrl,
		Model:   modelConfig.ModelName,
		Timeout: 5 * time.Minute,
		ExtraFields: map[string]any{
			"enable_thinking": agentCtx.EnableThinking,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create openai chat model: %w", err)
	}
	return chatModel, nil
}

// getChatModelConfig 获取聊天模型配置，带缓存功能避免重复查库
func (f *EinoAgentFactory) getChatModelConfig(agentCtx *types.AgentContext) (*types.ModelConfig, error) {
	// 如果已经有缓存，直接返回
	if f.cachedChatModelConfig != nil {
		return f.cachedChatModelConfig, nil
	}

	// 使用 GetActiveModelConfig 获取当前激活的聊天模型配置
	modelConfig, err := f.core.GetActiveModelConfig(agentCtx, types.AI_USAGE_CHAT)
	if err != nil {
		return nil, fmt.Errorf("failed to get active chat model config: %w", err)
	}

	// 缓存配置
	f.cachedChatModelConfig = modelConfig
	return modelConfig, nil
}

// getVisionModelConfig 获取视觉模型配置，带缓存功能避免重复查库
func (f *EinoAgentFactory) getVisionModelConfig(agentCtx *types.AgentContext) (*types.ModelConfig, error) {
	// 如果已经有缓存，直接返回
	if f.cachedVisionModelConfig != nil {
		return f.cachedVisionModelConfig, nil
	}

	// 使用 GetActiveModelConfig 获取当前激活的视觉模型配置
	modelConfig, err := f.core.GetActiveModelConfig(agentCtx, types.AI_USAGE_VISION)
	if err != nil {
		return nil, fmt.Errorf("failed to get active vision model config: %w", err)
	}

	// 缓存配置
	f.cachedVisionModelConfig = modelConfig
	return modelConfig, nil
}

// containsMultimediaContent 检查消息中是否包含多媒体内容（图片、音频、视频等）
func (f *EinoAgentFactory) containsMultimediaContent(messages []*schema.Message) bool {
	for _, msg := range messages {
		if len(msg.MultiContent) > 0 {
			return true
			// for _, part := range msg.MultiContent {
			// 	// 检查是否包含非文本内容
			// 	switch part.Type {
			// 	case schema.ChatMessagePartTypeImageURL,
			// 		 schema.ChatMessagePartTypeAudioURL,
			// 		 schema.ChatMessagePartTypeVideoURL,
			// 		 schema.ChatMessagePartTypeFileURL:
			// 		return true
			// 	}
			// }
		}
	}
	return false
}

// ClearModelConfigCache 清除模型配置缓存，用于配置更新后重新获取
func (f *EinoAgentFactory) ClearModelConfigCache() {
	f.cachedChatModelConfig = nil
	f.cachedVisionModelConfig = nil
}

// createTools 创建可用工具列表
func (f *EinoAgentFactory) createTools(agentCtx *types.AgentContext, adapter *ai.EinoAdapter) ([]tool.BaseTool, error) {
	var tools []tool.BaseTool

	// 根据 EnableWebSearch 标志决定是否添加 DuckDuckGo 搜索工具
	if agentCtx.EnableWebSearch {
		duckduckgoTool, err := duckduckgo.NewTool(agentCtx, ddg.RegionCN)
		if err != nil {
			slog.Warn("Failed to create DuckDuckGo tool", slog.String("error", err.Error()))
		} else {
			// 🔥 使用 NotifyingTool 包装 DuckDuckGo 工具
			notifyingDDGTool := NewNotifyingTool(duckduckgoTool, adapter)
			tools = append(tools, notifyingDDGTool)
		}
	}

	// 添加 RAG 知识库搜索工具
	ragTool := rag.NewRagTool(f.core, agentCtx.SpaceID, agentCtx.UserID, agentCtx.SessionID, agentCtx.MessageID)
	// 🔥 使用 NotifyingTool 包装 RAG 工具
	notifyingRagTool := NewNotifyingTool(ragTool, adapter)
	tools = append(tools, notifyingRagTool)

	// TODO: 这里可以添加更多工具
	// - 文件处理工具
	// - 计算工具等

	return tools, nil
}

// EinoResponseHandler 处理 eino Agent 的响应
type EinoResponseHandler struct {
	receiveFunc types.ReceiveFunc
	doneFunc    types.DoneFunc
	adapter     *ai.EinoAdapter
	marks       map[string]string // 特殊语法标记处理
}

// NewEinoResponseHandler 创建响应处理器
func NewEinoResponseHandler(receiveFunc types.ReceiveFunc, doneFunc types.DoneFunc, adapter *ai.EinoAdapter, marks map[string]string) *EinoResponseHandler {
	return &EinoResponseHandler{
		receiveFunc: receiveFunc,
		doneFunc:    doneFunc,
		adapter:     adapter,
		marks:       marks,
	}
}

// HandleStreamResponse 处理 eino Agent 的流式响应，返回 ResponseChoice 通道以兼容现有接口
func (h *EinoResponseHandler) HandleStreamResponse(ctx context.Context, stream *schema.StreamReader[*model.CallbackOutput]) error {
	ticker := time.NewTicker(time.Millisecond * 500)

	defer func() {
		ticker.Stop()
	}()

	var (
		once      = sync.Once{}
		strs      = strings.Builder{}
		messageID string
		mu        sync.Mutex

		maybeMarks  bool
		machedMarks bool
		needToMarks = len(h.marks) > 0

		startThinking    = sync.Once{}
		hasThinking      = false
		finishedThinking = sync.Once{}
		// toolCalls []*schema.ToolCall // 使用 eino 原生结构
	)

	flushResponse := func() {
		mu.Lock()
		defer mu.Unlock()
		if strs.Len() > 0 {
			if err := h.receiveFunc(&types.TextMessage{
				Text: strs.String(),
			}, types.MESSAGE_PROGRESS_GENERATING); err != nil {
				slog.Error("failed to call receiveFunc for database write", slog.Any("error", err))
			}
			strs.Reset()
		}
	}

	// 定时刷新协程
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if maybeMarks {
					continue
				}
				flushResponse()
			}
		}
	}()

	// 处理 eino stream
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := stream.Recv()
		if err != nil && err != io.EOF {
			return err
		}

		if err == io.EOF {
			flushResponse()
			return nil
		}

		raw, err := json.Marshal(msg)
		fmt.Println("111", string(raw))
		// if res == nil {
		// 	continue
		// }
		thinkingContent, existThinking := openailibs.GetReasoningContent(msg.Message)
		// 处理内容
		if msg.Message.Content == "" && !existThinking {
			continue
		}

		// 处理特殊语法标记
		if needToMarks {
			if !maybeMarks {
				if strings.Contains(msg.Message.Content, "$") {
					maybeMarks = true
					if strs.Len() != 0 {
						flushResponse()
					}
				}
			} else if maybeMarks && strs.Len() >= 10 {
				if strings.Contains(strs.String(), "$hidden[") {
					machedMarks = true
				} else {
					maybeMarks = false
				}
			}
		}

		// 处理思考内容
		if existThinking {
			if thinkingContent == "" {
				thinkingContent = msg.Message.Content
			}
			startThinking.Do(func() {
				hasThinking = true
				if strings.Contains(thinkingContent, "\n") {
					thinkingContent = ""
				}
				strs.WriteString("<think>")
			})
			strs.WriteString(strings.ReplaceAll(thinkingContent, "\n", "</br>"))
		} else {
			finishedThinking.Do(func() {
				if !hasThinking {
					return
				}
				strs.WriteString("</think>")
			})
			strs.WriteString(msg.Message.Content)

			// 处理隐藏标记
			if machedMarks && strings.Contains(msg.Message.Content, "]") {
				text, replaced := mark.ResolveHidden(strs.String(), func(fakeValue string) string {
					real := h.marks[fakeValue]
					return real
				}, false)
				if replaced {
					strs.Reset()
					strs.WriteString(text)
					maybeMarks = false
					machedMarks = false
				}
			}
		}

		once.Do(func() {
			// eino 消息 ID 可能需要从其他地方获取
			if msg.Message.Extra != nil {
				if id, ok := msg.Message.Extra["id"].(string); ok {
					messageID = id
				}
			}
			if messageID == "" {
				messageID = fmt.Sprintf("eino-%d", time.Now().UnixNano())
			}
		})
	}
}

// handleToolCalls 处理工具调用（持久化）
func (h *EinoResponseHandler) handleToolCalls(ctx context.Context, toolCalls []*schema.ToolCall) {
	for _, toolCall := range toolCalls {
		// 1. 发送 WebSocket 通知（实时显示）
		if err := h.adapter.RecordToolCall(toolCall.Function.Name, toolCall.Function.Arguments, types.TOOL_STATUS_RUNNING); err != nil {
			slog.Error("failed to record tool call via adapter", slog.Any("error", err))
		}

		// 2. 持久化到数据库（需要创建 ToolCallPersister 实例）
		// 注意：这里需要从 EinoResponseHandler 中访问必要的上下文信息
		// 暂时记录日志，实际实现需要传入更多上下文
		slog.Info("eino tool call detected",
			slog.String("tool_name", toolCall.Function.Name),
			slog.String("arguments", toolCall.Function.Arguments),
			slog.String("tool_id", toolCall.ID))
	}
}

// convertToolCallsToOpenAI 将 eino ToolCall 转换为兼容格式（用于 DeepContinue）
func (h *EinoResponseHandler) convertToolCallsToOpenAI(toolCalls []*schema.ToolCall) []*goopenai.ToolCall {
	result := make([]*goopenai.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		result[i] = &goopenai.ToolCall{
			ID:   tc.ID,
			Type: goopenai.ToolTypeFunction,
			Function: goopenai.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

// SendedCounterImpl 实现 SendedCounter 接口
type SendedCounterImpl struct {
	count int
	mutex sync.Mutex
}

func (s *SendedCounterImpl) Add(n []byte) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.count += len(n)
}

func (s *SendedCounterImpl) Get() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.count
}

// ToolCallMessage 工具调用消息结构
type ToolCallMessage struct {
	ToolName  string      `json:"tool_name"`
	Arguments interface{} `json:"arguments"`
	Result    interface{} `json:"result,omitempty"`
	Status    string      `json:"status"` // "running", "success", "failed"
	StartTime int64       `json:"start_time"`
	EndTime   int64       `json:"end_time,omitempty"`
}

// ToolCallPersister 工具调用持久化器，将工具调用过程保存到数据库作为聊天记录
type ToolCallPersister struct {
	core      *core.Core
	sessionID string
	spaceID   string
	userID    string
}

// NewToolCallPersister 创建工具调用持久化器
func NewToolCallPersister(core *core.Core, sessionID, spaceID, userID string) *ToolCallPersister {
	return &ToolCallPersister{
		core:      core,
		sessionID: sessionID,
		spaceID:   spaceID,
		userID:    userID,
	}
}

// SaveToolCallStart 保存工具调用开始记录
func (p *ToolCallPersister) SaveToolCallStart(ctx context.Context, toolName string, args interface{}) (string, error) {
	// 生成工具调用消息ID
	toolCallMsgID := utils.GenUniqIDStr()

	// 创建工具调用记录
	toolCallMsg := ToolCallMessage{
		ToolName:  toolName,
		Arguments: args,
		Status:    "running",
		StartTime: time.Now().Unix(),
	}

	// 创建聊天消息
	chatMsg := p.createToolCallMessage(toolCallMsgID, &toolCallMsg)

	// 保存到数据库
	if err := p.core.Store().ChatMessageStore().Create(ctx, chatMsg); err != nil {
		slog.Error("Failed to save tool call start record",
			slog.String("tool_name", toolName),
			slog.String("error", err.Error()))
		return "", err
	}

	slog.Debug("Tool call start record saved",
		slog.String("tool_name", toolName),
		slog.String("msg_id", toolCallMsgID))

	return toolCallMsgID, nil
}

// SaveToolCallComplete 更新工具调用完成记录
func (p *ToolCallPersister) SaveToolCallComplete(ctx context.Context, toolCallMsgID string, result interface{}, success bool) error {
	// 获取原始消息
	originalMsg, err := p.core.Store().ChatMessageStore().GetOne(ctx, toolCallMsgID)
	if err != nil {
		slog.Error("Failed to get original tool call message",
			slog.String("msg_id", toolCallMsgID),
			slog.String("error", err.Error()))
		return err
	}

	if originalMsg == nil {
		return fmt.Errorf("tool call message not found: %s", toolCallMsgID)
	}

	// 解析原始工具调用信息
	var toolCallMsg ToolCallMessage
	if err := json.Unmarshal([]byte(originalMsg.Message), &toolCallMsg); err != nil {
		// 如果解析失败，尝试从消息文本中提取信息
		toolCallMsg = ToolCallMessage{
			ToolName:  "unknown",
			StartTime: originalMsg.SendTime,
		}
	}

	// 更新工具调用信息
	toolCallMsg.Result = result
	toolCallMsg.EndTime = time.Now().Unix()
	if success {
		toolCallMsg.Status = "success"
	} else {
		toolCallMsg.Status = "failed"
	}

	// 生成新的消息内容
	newMessage := p.formatToolCallMessage(&toolCallMsg)

	// 更新数据库记录
	if err := p.core.Store().ChatMessageStore().RewriteMessage(ctx, p.spaceID, p.sessionID, toolCallMsgID, []byte(newMessage), int32(types.MESSAGE_PROGRESS_COMPLETE)); err != nil {
		slog.Error("Failed to update tool call complete record",
			slog.String("msg_id", toolCallMsgID),
			slog.String("error", err.Error()))
		return err
	}

	slog.Debug("Tool call complete record updated",
		slog.String("msg_id", toolCallMsgID),
		slog.Bool("success", success))

	return nil
}

// createToolCallMessage 创建工具调用消息格式
func (p *ToolCallPersister) createToolCallMessage(msgID string, toolCall *ToolCallMessage) *types.ChatMessage {
	message := p.formatToolCallMessage(toolCall)

	// 生成序列号 - 通过session获取当前最新消息的seq然后+1
	seqID, err := p.core.Plugins.GetChatSessionSeqID(context.Background(), p.spaceID, p.sessionID)
	if err != nil {
		// 如果获取失败，回退到原来的随机生成方式
		seqID = utils.GenUniqID()
		slog.Warn("Failed to get chat session seq ID, using random ID instead",
			slog.String("space_id", p.spaceID),
			slog.String("session_id", p.sessionID),
			slog.String("error", err.Error()))
	}

	return &types.ChatMessage{
		ID:        msgID,
		SpaceID:   p.spaceID,
		SessionID: p.sessionID,
		UserID:    p.userID,
		Role:      types.USER_ROLE_TOOL,
		Message:   message,
		MsgType:   types.MESSAGE_TYPE_TEXT,
		SendTime:  toolCall.StartTime,
		Complete:  types.MESSAGE_PROGRESS_GENERATING, // 开始时状态为生成中
		Sequence:  seqID,
	}
}

// formatToolCallMessage 格式化工具调用消息内容
func (p *ToolCallPersister) formatToolCallMessage(toolCall *ToolCallMessage) string {
	// 创建用户友好的显示格式
	content := fmt.Sprintf("🔧 工具调用: %s", toolCall.ToolName)

	// 添加参数信息
	if toolCall.Arguments != nil {
		argsJSON, _ := json.Marshal(toolCall.Arguments)
		content += fmt.Sprintf("\n参数: %s", string(argsJSON))
	}

	// 添加结果信息
	if toolCall.Result != nil {
		resultJSON, _ := json.Marshal(toolCall.Result)
		content += fmt.Sprintf("\n结果: %s", string(resultJSON))
	}

	// 添加状态信息
	statusMap := map[string]string{
		"running": "执行中...",
		"success": "执行成功",
		"failed":  "执行失败",
	}
	if statusText, exists := statusMap[toolCall.Status]; exists {
		content += fmt.Sprintf("\n状态: %s", statusText)
	}

	return content
}

func NewCallbackHandlers(modelName, reqMessageID string, handler *EinoResponseHandler) callbacks.Handler {
	return callbackhelper.NewHandlerHelper().ChatModel(&callbackhelper.ModelCallbackHandler{
		OnEnd: func(ctx context.Context, runInfo *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
			res := model.ConvCallbackOutput(output)
			if res.TokenUsage != nil {
				// 记录 Token 使用情况
				go process.NewRecordChatUsageRequest(modelName, types.USAGE_SUB_TYPE_CHAT, reqMessageID, &goopenai.Usage{
					TotalTokens:      res.TokenUsage.TotalTokens,
					PromptTokens:     res.TokenUsage.PromptTokens,
					CompletionTokens: res.TokenUsage.CompletionTokens,
				})
			}
			handler.doneFunc(nil)
			return ctx
		},
		OnError: func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context {
			if err != nil {
				slog.Error("eino callback error", slog.Any("error", err), slog.String("model_name", modelName), slog.String("message_id", reqMessageID))
				// 记录错误信息
				handler.doneFunc(err)
			}
			return ctx
		},
		OnEndWithStreamOutput: func(ctx context.Context, runInfo *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
			// 处理流式输出
			if output == nil {
				return ctx
			}

			infoRaw, _ := json.Marshal(runInfo)
			slog.Debug("eino callback stream output", slog.String("info", string(infoRaw)), slog.String("model_name", modelName), slog.String("message_id", reqMessageID))

			go safe.Run(func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
				defer cancel()
				if err := handler.HandleStreamResponse(ctx, output); err != nil {
					slog.Error("failed to handle stream response", slog.Any("error", err), slog.String("model_name", modelName), slog.String("message_id", reqMessageID))
					return
				}
			})

			return ctx
		},
	}).Handler()
}
