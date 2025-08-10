package v1

import (
	"context"
	"database/sql"
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
		prompt = lo.If(space.BasePrompt != "", space.BasePrompt).Else(ai.GENERATE_PROMPT_TPL_NONE_CONTENT_CN) + ai.APPEND_PROMPT_CN
	} else {
		prompt = ai.BuildRAGPrompt(ai.GENERATE_PROMPT_TPL_CN, ai.NewDocs(aiCallOptions.Docs.Docs), a.core.Srv().AI())
	}

	// 3. 生成会话上下文
	sessionContext, err := a.GenSessionContext(ctx, prompt, reqMsg)
	if err != nil {
		return err
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
		reqMsg.Sequence,
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

		if len(msgCtx.ToolCalls) > 0 {
			einoMsg.ToolCalls = lo.Map(msgCtx.ToolCalls, func(item goopenai.ToolCall, _ int) schema.ToolCall {
				return schema.ToolCall{
					Type: string(item.Type),
					Function: schema.FunctionCall{
						Name:      item.Function.Name,
						Arguments: item.Function.Arguments,
					},
				}
			})
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

	// adapter := ai.NewEinoAdapter(receiver, reqMsg.SessionID, reqMsg.ID)
	notifyToolWrapper := NewNotifyToolWrapper(a.core, reqMsg, receiver.Copy())

	factory := NewEinoAgentFactory(a.core)
	agent, modelConfig, err := factory.CreateReActAgent(agentCtx, notifyToolWrapper, einoMessages)
	if err != nil {
		return err
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
	responseHandler := NewEinoResponseHandler(receiver, reqMsg, marks)
	callbackHandler := NewCallbackHandlers(a.core, modelConfig.ModelName, reqMsg)

	// 10. 执行推理
	if receiver.IsStream() {
		// 流式处理
		return a.handleStreamResponse(agentCtx, agent, einoMessages, responseHandler, callbackHandler)
	} else {
		// 非流式处理
		return a.handleDirectResponse(agentCtx, agent, einoMessages, responseHandler)
	}
}

type ReasoningContent struct {
	Extra struct {
		ReasoningContent string `json:"reasoning-content"` // 推理内容
	} `json:"extra"`
}

// handleStreamResponse 处理流式响应
func (a *AutoAssistant) handleStreamResponse(ctx context.Context, reactAgent *react.Agent, messages []*schema.Message, streamHandler *EinoResponseHandler, callbacksHandler callbacks.Handler) error {
	reqMessage := streamHandler.reqMsg
	initFunc := func(ctx context.Context) error {
		// streamHandler.Init()
		if err := streamHandler.Receiver().RecvMessageInit(types.ChatMessageExt{
			SpaceID:   reqMessage.SpaceID,
			SessionID: reqMessage.SessionID,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		}); err != nil {
			slog.Error("failed to initialize receive message", slog.String("error", err.Error()))
			return err
		}

		slog.Debug("AI message session created",
			slog.String("msg_id", streamHandler.Receiver().MessageID()),
			slog.String("session_id", reqMessage.SessionID))
		return nil
	}

	// 使用 eino agent 进行流式推理
	result, err := reactAgent.Stream(ctx, messages, agent.WithComposeOptions(
		// compose.WithCallbacks()
		// compose.WithCallbacks(&LoggerCallback{}),
		compose.WithCallbacks(callbacksHandler, &LoggerCallback{}),
	))
	if err != nil {
		initFunc(ctx)
		streamHandler.GetDoneFunc(nil)(err)
		slog.Error("failed to start eino stream response", slog.Any("error", err))
		return err
	}

	if err := streamHandler.HandleStreamResponse(ctx, result, initFunc); err != nil {
		slog.Error("failed to handle stream response", slog.Any("error", err), slog.String("message_id", reqMessage.ID))
		return err
	}

	return nil
}

// handleDirectResponse 处理非流式响应
func (a *AutoAssistant) handleDirectResponse(ctx context.Context, agent *react.Agent, messages []*schema.Message, handler *EinoResponseHandler) error {
	// 使用 eino agent 进行推理
	done := handler.GetDoneFunc(nil)
	result, err := agent.Generate(ctx, messages)
	if err != nil {
		if done != nil {
			done(err)
		}
		return err
	}

	if err = handler.GetReceiveFunc()(&types.TextMessage{Text: result.Content}, types.MESSAGE_PROGRESS_GENERATING); err != nil {
		return err
	}
	return done(nil)
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
	core     *core.Core
	receiver types.Receiver
	reqMsg   *types.ChatMessage
	// saver    *ToolCallSaver
	toolID string // 为每个工具实例分配唯一ID
}

// // NewNotifyingTool 创建带通知功能的工具包装器
// func NewNotifyingTool(baseTool tool.InvokableTool, receiver types.Receiver) *NotifyingTool {
// 	return &NotifyingTool{
// 		InvokableTool: baseTool,
// 		receiver:      receiver,
// 		toolID:        utils.GenUniqIDStr(), // 生成唯一的工具实例ID
// 	}
// }

type NotifyToolWrapper interface {
	Wrap(baseTool tool.InvokableTool) *NotifyingTool
}

func NewNotifyToolWrapper(core *core.Core, reqMsg *types.ChatMessage, receiver types.Receiver) NotifyToolWrapper {
	return &NotifyingTool{
		core:     core,
		reqMsg:   reqMsg,
		receiver: receiver,
		toolID:   utils.GenUniqIDStr(), // 生成唯一的工具实例ID
	}
}

func (nt *NotifyingTool) Wrap(baseTool tool.InvokableTool) *NotifyingTool {
	c := *nt
	c.InvokableTool = baseTool
	return &c
}

// InvokableRun 执行工具调用，并在执行前后发送通知
func (nt *NotifyingTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 获取工具信息
	toolInfo, err := nt.InvokableTool.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get tool info: %w", err)
	}
	toolName := toolInfo.Name

	slog.Debug("invoke tool", slog.String("tool_name", toolName), slog.String("tool_args", argumentsInJSON))

	if err = nt.receiver.RecvMessageInit(types.ChatMessageExt{
		SessionID: nt.reqMsg.SessionID,
		SpaceID:   nt.reqMsg.SpaceID,
		ToolName:  toolName,
		ToolArgs: sql.NullString{
			String: argumentsInJSON,
			Valid:  true,
		},
	}); err != nil {
		return "", err
	}

	toolTips := &types.ToolTips{
		ID:       nt.receiver.MessageID(),
		ToolName: toolName,
		Content:  fmt.Sprintf("Using tool: %s", toolName),
	}

	receiveFunc := nt.receiver.GetReceiveFunc()
	doneFunc := nt.receiver.GetDoneFunc(nil)
	receiveFunc(toolTips, types.MESSAGE_PROGRESS_GENERATING)

	// 执行实际工具
	startTime := time.Now()
	result, err := nt.InvokableTool.InvokableRun(ctx, argumentsInJSON, opts...)
	duration := time.Since(startTime)
	if err != nil {
		doneFunc(err)
		return err.Error(), nil
	}

	slog.Debug("tool call result", slog.Float64("duration", duration.Seconds()), slog.String("tool", toolName), slog.Any("error", err))

	resultJson := &types.ToolTips{
		ID:       nt.receiver.MessageID(),
		ToolName: toolName,
		Content:  result,
	}
	receiveFunc(resultJson, lo.If(err != nil, types.MESSAGE_PROGRESS_FAILED).Else(types.MESSAGE_PROGRESS_COMPLETE))
	doneFunc(nil)
	return string(resultJson.Bytes()), err
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
func (f *EinoAgentFactory) CreateReActAgent(agentCtx *types.AgentContext, toolWrapper NotifyToolWrapper, messages []*schema.Message) (*react.Agent, *types.ModelConfig, error) {
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

	chatModel, err := GetToolCallingModel(agentCtx, *modelConfig)
	if err != nil {
		return nil, nil, err
	}

	// 创建工具配置
	tools, err := f.createTools(agentCtx, toolWrapper)
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

func GetToolCallingModel(agentCtx *types.AgentContext, modelConfig types.ModelConfig) (model.ToolCallingChatModel, error) {
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
func (f *EinoAgentFactory) createTools(agentCtx *types.AgentContext, toolWrapper NotifyToolWrapper) ([]tool.BaseTool, error) {
	var tools []tool.BaseTool

	// 根据 EnableWebSearch 标志决定是否添加 DuckDuckGo 搜索工具
	if agentCtx.EnableWebSearch {
		duckduckgoTool, err := duckduckgo.NewTool(agentCtx, ddg.RegionCN)
		if err != nil {
			slog.Warn("Failed to create DuckDuckGo tool", slog.String("error", err.Error()))
		} else {
			// 🔥 使用 NotifyingTool 包装 DuckDuckGo 工具
			notifyingDDGTool := toolWrapper.Wrap(duckduckgoTool)
			tools = append(tools, notifyingDDGTool)
		}
	}

	// 添加 RAG 知识库搜索工具
	ragTool := rag.NewRagTool(f.core, agentCtx.SpaceID, agentCtx.UserID, agentCtx.SessionID, agentCtx.MessageID, agentCtx.MessageSequence)
	// 🔥 使用 NotifyingTool 包装 RAG 工具
	notifyingRagTool := toolWrapper.Wrap(ragTool)
	tools = append(tools, notifyingRagTool)

	// TODO: 这里可以添加更多工具
	// - 文件处理工具
	// - 计算工具等

	return tools, nil
}

// EinoResponseHandler 处理 eino Agent 的响应
type EinoResponseHandler struct {
	_receiveFunc types.ReceiveFunc
	_doneFunc    types.DoneFunc
	_receiver    types.Receiver
	reqMsg       *types.ChatMessage
	// adapter     *ai.EinoAdapter
	marks map[string]string // 特殊语法标记处理
}

// NewEinoResponseHandler 创建响应处理器
func NewEinoResponseHandler(receiver types.Receiver, reqMsg *types.ChatMessage, marks map[string]string) *EinoResponseHandler {
	return &EinoResponseHandler{
		_receiver: receiver,
		reqMsg:    reqMsg,
		// receiveFunc: receiveFunc,
		//doneFunc:    doneFunc,
		// adapter:     adapter,
		marks: marks,
	}
}

func (h *EinoResponseHandler) Receiver() types.Receiver {
	return h._receiver
}

func (h *EinoResponseHandler) Init() {
	h._receiver = h._receiver.Copy()
	h._receiveFunc = nil
	h._doneFunc = nil
}

func (h *EinoResponseHandler) GetReceiveFunc() types.ReceiveFunc {
	if h._receiveFunc == nil {
		h._receiveFunc = h._receiver.GetReceiveFunc()
	}
	return h._receiveFunc
}

func (h *EinoResponseHandler) GetDoneFunc(callback func(msg *types.ChatMessage)) types.DoneFunc {
	if h._doneFunc == nil {
		h._doneFunc = h._receiver.GetDoneFunc(callback)
	}
	return h._doneFunc
}

// HandleStreamResponse 处理 eino Agent 的流式响应，返回 ResponseChoice 通道以兼容现有接口
func (h *EinoResponseHandler) HandleStreamResponse(ctx context.Context, stream *schema.StreamReader[*schema.Message], needToCreateMessage func(ctx context.Context) error) error {
	ticker := time.NewTicker(time.Millisecond * 300)

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

	// nop handler
	receiveFunc := h.GetReceiveFunc()
	doneFunc := h.GetDoneFunc(nil)

	flushResponse := func() {
		mu.Lock()
		defer mu.Unlock()
		if strs.Len() > 0 {
			if err := receiveFunc(&types.TextMessage{
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

	isFirstChunk := true

	// 处理 eino stream
	for {
		select {
		case <-ctx.Done():
			doneFunc(ctx.Err())
			return ctx.Err()
		default:
		}

		msg, err := stream.Recv()
		// raw, _ := json.Marshal(msg)
		// fmt.Println("ttttt", string(raw), "eeee", err)
		if err != nil && err != io.EOF {
			doneFunc(err)
			return err
		}

		if isFirstChunk {
			isFirstChunk = false
			if len(msg.ToolCalls) == 0 { // 工具调用不创建message消息
				if err := needToCreateMessage(ctx); err != nil {
					return err
				}
				// receiveFunc = h.GetReceiveFunc()
				// doneFunc = h.GetDoneFunc(nil)
			}
		}

		if err == io.EOF || msg.ResponseMeta.FinishReason != "" {
			flushResponse()
			doneFunc(nil)
			return nil
		}

		thinkingContent, existThinking := openailibs.GetReasoningContent(msg)
		// 处理内容
		if msg.Content == "" && !existThinking {
			continue
		}

		// 处理特殊语法标记
		if needToMarks {
			if !maybeMarks {
				if strings.Contains(msg.Content, "$") {
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
				thinkingContent = msg.Content
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
			strs.WriteString(msg.Content)

			// 处理隐藏标记
			if machedMarks && strings.Contains(msg.Content, "]") {
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
			if msg.Extra != nil {
				if id, ok := msg.Extra["id"].(string); ok {
					messageID = id
				}
			}
			if messageID == "" {
				messageID = fmt.Sprintf("eino-%d", time.Now().UnixNano())
			}
		})
	}
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

// ToolCallSaver 工具调用持久化器，将工具调用过程保存到数据库作为聊天记录
type ToolCallSaver struct {
	core      *core.Core
	sessionID string
	spaceID   string
	userID    string
}

// NewToolCallSaver 创建工具调用持久化器
func NewToolCallSaver(core *core.Core, sessionID, spaceID, userID string) *ToolCallSaver {
	return &ToolCallSaver{
		core:      core,
		sessionID: sessionID,
		spaceID:   spaceID,
		userID:    userID,
	}
}

// SaveToolCallStart 保存工具调用开始记录
func (p *ToolCallSaver) SaveToolCallStart(ctx context.Context, toolName, args string) (string, error) {
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
	// 生成序列号 - 通过session获取当前最新消息的seq然后+1
	seqID, err := p.core.Plugins.GetChatSessionSeqID(context.Background(), p.spaceID, p.sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get session sequence id, %w", err)
	}

	chatMsg := &types.ChatMessage{
		ID:        toolCallMsgID,
		SpaceID:   p.spaceID,
		SessionID: p.sessionID,
		UserID:    p.userID,
		Role:      types.USER_ROLE_TOOL,
		Message:   "",
		MsgType:   types.MESSAGE_TYPE_TEXT,
		SendTime:  toolCallMsg.StartTime,
		Complete:  types.MESSAGE_PROGRESS_GENERATING, // 开始时状态为生成中
		Sequence:  seqID,
	}

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
func (p *ToolCallSaver) SaveToolCallComplete(ctx context.Context, toolCallMsgID, args, result string, success bool) error {
	// // 获取原始消息
	// originalMsg, err := p.core.Store().ChatMessageStore().GetOne(ctx, toolCallMsgID)
	// if err != nil {
	// 	slog.Error("Failed to get original tool call message",
	// 		slog.String("msg_id", toolCallMsgID),
	// 		slog.String("error", err.Error()))
	// 	return err
	// }

	// if originalMsg == nil {
	// 	return fmt.Errorf("tool call message not found: %s", toolCallMsgID)
	// }

	// // 解析原始工具调用信息
	// var toolCallMsg ToolCallMessage
	// if err := json.Unmarshal([]byte(originalMsg.Message), &toolCallMsg); err != nil {
	// 	// 如果解析失败，尝试从消息文本中提取信息
	// 	toolCallMsg = ToolCallMessage{
	// 		ToolName:  "unknown",
	// 		StartTime: originalMsg.SendTime,
	// 	}
	// }

	// // 更新工具调用信息
	// toolCallMsg.Result = result
	// toolCallMsg.EndTime = time.Now().Unix()
	// if success {
	// 	toolCallMsg.Status = "success"
	// } else {
	// 	toolCallMsg.Status = "failed"
	// }

	// 生成新的消息内容
	// newMessage := p.formatToolCallMessage(&toolCallMsg)

	messageStatus := lo.If(success, types.MESSAGE_PROGRESS_COMPLETE).Else(types.MESSAGE_PROGRESS_FAILED)

	resultWithArgs := map[string]json.RawMessage{
		"args":   json.RawMessage(args),
		"result": json.RawMessage(result),
	}

	raw, _ := json.Marshal(resultWithArgs)

	// 更新数据库记录
	if err := p.core.Store().ChatMessageStore().RewriteMessage(ctx, p.spaceID, p.sessionID, toolCallMsgID, raw, messageStatus); err != nil {
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

func NewCallbackHandlers(core *core.Core, modelName string, reqMessage *types.ChatMessage) callbacks.Handler {
	return callbackhelper.NewHandlerHelper().ChatModel(&callbackhelper.ModelCallbackHandler{
		OnStart: func(ctx context.Context, runInfo *callbacks.RunInfo, input *model.CallbackInput) context.Context {
			// if err := initFunc(ctx); err != nil {
			// 	ctx, cancel := context.WithCancel(ctx)
			// 	cancel()
			// 	return ctx
			// }
			// latestMessage := input.Messages[len(input.Messages)-1]
			// 创建 assistant 消息记录，代表一次完整的 AI 响应会话
			// (替代原来在 RequestAssistant 中的 InitAssistantMessage 调用)
			return ctx
		},
		OnEnd: func(ctx context.Context, runInfo *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
			res := model.ConvCallbackOutput(output)
			if res.TokenUsage != nil {
				// 记录 Token 使用情况
				go process.NewRecordChatUsageRequest(modelName, types.USAGE_SUB_TYPE_CHAT, reqMessage.ID, &goopenai.Usage{
					TotalTokens:      res.TokenUsage.TotalTokens,
					PromptTokens:     res.TokenUsage.PromptTokens,
					CompletionTokens: res.TokenUsage.CompletionTokens,
				})
			}
			return ctx
		},
		OnError: func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context {
			if err != nil {
				slog.Error("eino callback error", slog.Any("error", err), slog.String("model_name", modelName), slog.String("message_id", reqMessage.Message))
				// 记录错误信息
				// streamHandler.GetDoneFunc(nil)(err)
			}
			return ctx
		},
		OnEndWithStreamOutput: func(ctx context.Context, runInfo *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
			// 处理流式输出
			if output == nil {
				return ctx
			}
			infoRaw, _ := json.Marshal(runInfo)
			slog.Debug("eino callback stream output", slog.String("info", string(infoRaw)), slog.String("model_name", modelName), slog.String("message_id", reqMessage.ID))

			return ctx
		},
	}).Handler()
	// .Tool(&callbackhelper.ToolCallbackHandler{})
}
