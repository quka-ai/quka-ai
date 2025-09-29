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
	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/butler"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/journal"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/rag"
	"github.com/quka-ai/quka-ai/pkg/ai/tools/duckduckgo"
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

// HandleAssistantEarlyError 通用的早期错误处理函数
// 用于在 RequestAssistant 准备阶段出现错误时，向前端发送失败响应
// 这确保了即使在消息初始化前出错，前端也能收到错误通知
func HandleAssistantEarlyError(err error, reqMsg *types.ChatMessage, receiver types.Receiver, errorPrefix string) error {
	if err == nil {
		return nil
	}

	slog.Error("Early error in RequestAssistant",
		slog.String("error", err.Error()),
		slog.String("prefix", errorPrefix),
		slog.String("session_id", reqMsg.SessionID),
		slog.String("message_id", reqMsg.ID))

	// 初始化错误响应消息
	if initErr := receiver.RecvMessageInit(types.ChatMessageExt{
		SpaceID:   reqMsg.SpaceID,
		SessionID: reqMsg.SessionID,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}); initErr != nil {
		slog.Error("failed to initialize error message",
			slog.String("init_error", initErr.Error()),
			slog.String("original_error", err.Error()))
		return fmt.Errorf("failed to initialize error message: %w (original: %s)", initErr, err.Error())
	}

	// 发送错误消息给前端
	receiveFunc := receiver.GetReceiveFunc()
	doneFunc := receiver.GetDoneFunc(nil)

	// 构建错误消息
	errorMsg := fmt.Sprintf("%s: %s", errorPrefix, err.Error())
	receiveFunc(&types.TextMessage{Text: errorMsg}, types.MESSAGE_PROGRESS_FAILED)
	doneFunc(err)

	return err
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
		return HandleAssistantEarlyError(err, reqMsg, receiver, "获取空间信息失败")
	}

	// 2. 准备提示词
	prompt := ai.BuildPrompt(space.BasePrompt, ai.MODEL_BASE_LANGUAGE_CN)
	prompt = receiver.VariableHandler().Do(prompt)

	// 3. 生成会话上下文
	sessionContext, err := a.GenSessionContext(ctx, prompt, reqMsg)
	if err != nil {
		return HandleAssistantEarlyError(err, reqMsg, receiver, "生成会话上下文失败")
	}

	// 4. 创建 AgentContext - 提取思考和搜索配置
	agentCtx := types.NewAgentContextWithOptions(
		ctx,
		reqMsg.SpaceID,
		reqMsg.UserID,
		reqMsg.SessionID,
		reqMsg.ID,
		reqMsg.Sequence,
		aiCallOptions.EnableThinking,
		aiCallOptions.EnableSearch,
		aiCallOptions.EnableKnowledge,
	)

	// 5. 将 MessageContext 转换为 eino 消息格式
	einoMessages := ai.ConvertMessageContextToEinoMessages(sessionContext.MessageContext)
	einoMessages = lo.Map(einoMessages, func(item *schema.Message, _ int) *schema.Message {
		item.Content = receiver.VariableHandler().Do(item.Content)
		return item
	})

	// adapter := ai.NewEinoAdapter(receiver, reqMsg.SessionID, reqMsg.ID)
	notifyToolWrapper := NewNotifyToolWrapper(a.core, reqMsg, receiver.Copy())

	factory := NewEinoAgentFactory(a.core)
	agent, modelConfig, err := factory.CreateAutoRagReActAgent(agentCtx, notifyToolWrapper, einoMessages)
	if err != nil {
		return HandleAssistantEarlyError(err, reqMsg, receiver, "创建AI代理失败")
	}

	// 从这里开始，错误处理交给具体的 handler 方法
	// 创建响应处理器（传入数据库写入函数）
	responseHandler := NewEinoResponseHandler(receiver, reqMsg)
	callbackHandler := NewCallbackHandlers(a.core, modelConfig.ModelName, responseHandler)

	// 10. 执行推理
	if receiver.IsStream() {
		// 流式处理
		if err = a.handleStreamResponse(agentCtx, agent, einoMessages, callbackHandler); err != nil {
			responseHandler.Receiver().GetDoneFunc(nil)(err)
			return err
		}
	} else {
		// 非流式处理
		return a.handleDirectResponse(agentCtx, agent, einoMessages, responseHandler)
	}
	return nil
}

type ReasoningContent struct {
	Extra struct {
		ReasoningContent string `json:"reasoning-content"` // 推理内容
	} `json:"extra"`
}

// handleStreamResponse 处理流式响应
func (a *AutoAssistant) handleStreamResponse(ctx context.Context, reactAgent *react.Agent, messages []*schema.Message, callbacksHandler callbacks.Handler) error {
	// reqMessage := streamHandler.reqMsg
	// initFunc := func(ctx context.Context) error {
	// 	// streamHandler.Init()
	// 	if err := streamHandler.Receiver().RecvMessageInit(types.ChatMessageExt{
	// 		SpaceID:   reqMessage.SpaceID,
	// 		SessionID: reqMessage.SessionID,
	// 		CreatedAt: time.Now().Unix(),
	// 		UpdatedAt: time.Now().Unix(),
	// 	}); err != nil {
	// 		slog.Error("failed to initialize receive message", slog.String("error", err.Error()))
	// 		return err
	// 	}

	// 	slog.Debug("AI message session created",
	// 		slog.String("msg_id", streamHandler.Receiver().MessageID()),
	// 		slog.String("session_id", reqMessage.SessionID))
	// 	return nil
	// }

	// 使用 eino agent 进行流式推理
	_, err := reactAgent.Stream(ctx, messages, agent.WithComposeOptions(
		compose.WithCallbacks(callbacksHandler, &LoggerCallback{}),
	))
	if err != nil {
		// initFunc(ctx)
		slog.Error("failed to start eino stream response", slog.Any("error", err))
		return err
	}

	// if err := streamHandler.HandleStreamResponse(ctx, result, initFunc); err != nil {
	// 	slog.Error("failed to handle stream response", slog.Any("error", err), slog.String("message_id", reqMessage.ID))
	// 	return err
	// }

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

	secretResult := nt.receiver.VariableHandler().Do(result)

	slog.Debug("tool call result", slog.Float64("duration", duration.Seconds()), slog.String("tool", toolName), slog.Any("error", err))

	resultJson := &types.ToolTips{
		ID:       nt.receiver.MessageID(),
		ToolName: toolName,
		Content:  result,
	}
	receiveFunc(resultJson, lo.If(err != nil, types.MESSAGE_PROGRESS_FAILED).Else(types.MESSAGE_PROGRESS_COMPLETE))
	doneFunc(nil)
	resultJson.Content = secretResult
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

// CreateAutoRagReActAgent
func (f *EinoAgentFactory) CreateAutoRagReActAgent(agentCtx *types.AgentContext, toolWrapper NotifyToolWrapper, messages []*schema.Message) (*react.Agent, *types.ModelConfig, error) {
	config := NewAgentConfig(agentCtx, toolWrapper, f.core, agentCtx.EnableThinking, messages)

	// 应用选项
	options := []AgentOption{
		&WithWebSearch{
			Enable: agentCtx.EnableWebSearch,
		}, // 支持网络搜索
		&WithRAG{
			Enable: agentCtx.EnableKnowledge,
		}, // 支持知识库搜索
	}

	for _, option := range options {
		if err := option.Apply(config); err != nil {
			slog.Warn("Failed to apply butler agent option", slog.Any("error", err))
		}
	}

	return f.CreateReActAgentWithConfig(config)
}

// CreateButlerReActAgent 创建包含Butler工具的ReAct Agent实例 (便捷方法)
func (f *EinoAgentFactory) CreateButlerReActAgent(agentCtx *types.AgentContext, toolWrapper NotifyToolWrapper, messages []*schema.Message, butlerAgent *butler.ButlerAgent) (*react.Agent, *types.ModelConfig, error) {
	config := NewAgentConfig(agentCtx, toolWrapper, f.core, agentCtx.EnableThinking, messages)

	// 应用选项
	options := []AgentOption{
		&WithWebSearch{
			Enable: agentCtx.EnableWebSearch,
		}, // 支持网络搜索
		&WithRAG{
			Enable: agentCtx.EnableKnowledge,
		}, // 支持知识库搜索
		NewWithButlerTools(butlerAgent), // 添加Butler专用工具
	}

	for _, option := range options {
		if err := option.Apply(config); err != nil {
			slog.Warn("Failed to apply butler agent option", slog.Any("error", err))
		}
	}

	return f.CreateReActAgentWithConfig(config)
}

// CreateJournalReActAgent 创建包含Journal工具的ReAct Agent实例 (便捷方法)
func (f *EinoAgentFactory) CreateJournalReActAgent(agentCtx *types.AgentContext, toolWrapper NotifyToolWrapper, messages []*schema.Message, journalAgent *journal.JournalAgent) (*react.Agent, *types.ModelConfig, error) {
	config := NewAgentConfig(agentCtx, toolWrapper, f.core, agentCtx.EnableThinking, messages)

	// 应用选项
	options := []AgentOption{
		&WithWebSearch{},                  // 支持网络搜索
		&WithRAG{},                        // 支持知识库搜索
		NewWithJournalTools(journalAgent), // 添加Journal专用工具
	}

	for _, option := range options {
		if err := option.Apply(config); err != nil {
			slog.Warn("Failed to apply journal agent option", slog.Any("error", err))
		}
	}

	return f.CreateReActAgentWithConfig(config)
}

// CreateCustomReActAgent 使用自定义选项创建ReAct Agent实例 (最灵活的方法)
func (f *EinoAgentFactory) CreateCustomReActAgent(agentCtx *types.AgentContext, toolWrapper NotifyToolWrapper, messages []*schema.Message, options ...AgentOption) (*react.Agent, *types.ModelConfig, error) {
	config := NewAgentConfig(agentCtx, toolWrapper, f.core, agentCtx.EnableThinking, messages)

	// 应用所有选项
	for _, option := range options {
		if err := option.Apply(config); err != nil {
			slog.Warn("Failed to apply custom agent option", slog.Any("error", err))
		}
	}

	return f.CreateReActAgentWithConfig(config)
}

func GetToolCallingModel(agentCtx *types.AgentContext, modelConfig types.ModelConfig) (model.ToolCallingChatModel, error) {
	// 根据模型的思考支持类型调整EnableThinking设置
	enableThinking := agentCtx.EnableThinking
	switch modelConfig.ThinkingSupport {
	case types.ThinkingSupportForced:
		enableThinking = true // 强制思考
	case types.ThinkingSupportNone:
		enableThinking = false // 不支持思考
		// ThinkingSupportOptional的情况保持原始设置
	}

	if strings.Contains(strings.ToLower(modelConfig.ModelName), "qwen") {
		// 创建 Qwen 模型
		chatModel, err := qwen.NewChatModel(agentCtx, &qwen.ChatModelConfig{
			APIKey:         modelConfig.Provider.ApiKey,
			BaseURL:        modelConfig.Provider.ApiUrl,
			Model:          modelConfig.ModelName,
			Timeout:        5 * time.Minute,
			EnableThinking: &enableThinking,
		})
		if err != nil {
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
			"enable_thinking": enableThinking,
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
		}
	}
	return false
}

// ClearModelConfigCache 清除模型配置缓存，用于配置更新后重新获取
func (f *EinoAgentFactory) ClearModelConfigCache() {
	f.cachedChatModelConfig = nil
	f.cachedVisionModelConfig = nil
}

// AgentOption Agent选项接口，用于配置Agent的各个方面
type AgentOption interface {
	Apply(config *AgentConfig) error
}

// AgentConfig Agent配置结构
type AgentConfig struct {
	AgentCtx    *types.AgentContext
	ToolWrapper NotifyToolWrapper
	Core        *core.Core
	Messages    []*schema.Message

	// 可扩展的配置字段
	Tools         []tool.BaseTool
	ModelOverride *types.ModelConfig // 可选：覆盖默认模型配置
	CustomPrompts []string           // 可选：自定义系统提示词
	MaxIterations int                // 可选：最大工具调用迭代次数

	EnableThinking bool
}

// NewAgentConfig 创建默认的Agent配置
func NewAgentConfig(agentCtx *types.AgentContext, toolWrapper NotifyToolWrapper, core *core.Core, enableThinking bool, messages []*schema.Message) *AgentConfig {
	return &AgentConfig{
		AgentCtx:       agentCtx,
		ToolWrapper:    toolWrapper,
		Core:           core,
		Messages:       messages,
		Tools:          []tool.BaseTool{},
		MaxIterations:  2, // 默认最大迭代次数
		EnableThinking: enableThinking,
	}
}

// === 工具相关选项 ===

// WithWebSearch 添加网络搜索工具选项
type WithWebSearch struct {
	Enable bool
}

func (o *WithWebSearch) Apply(config *AgentConfig) error {
	if !o.Enable {
		return nil
	}

	duckduckgoTool, err := duckduckgo.NewTool(config.AgentCtx, ddg.RegionCN)
	if err != nil {
		slog.Warn("Failed to create DuckDuckGo tool", slog.String("error", err.Error()))
		return nil
	}

	notifyingDDGTool := config.ToolWrapper.Wrap(duckduckgoTool)
	config.Tools = append(config.Tools, notifyingDDGTool)
	return nil
}

// WithRAG 添加RAG知识库工具选项
type WithRAG struct {
	Enable bool
}

func (o *WithRAG) Apply(config *AgentConfig) error {
	if !o.Enable {
		return nil
	}

	ragTool := rag.NewRagTool(config.Core, config.AgentCtx.SpaceID, config.AgentCtx.UserID, config.AgentCtx.SessionID, config.AgentCtx.MessageID, config.AgentCtx.MessageSequence)
	notifyingRagTool := config.ToolWrapper.Wrap(ragTool)
	config.Tools = append(config.Tools, notifyingRagTool)
	return nil
}

// WithButlerTools 添加Butler工具选项
type WithButlerTools struct {
	ButlerAgent *butler.ButlerAgent
}

func NewWithButlerTools(butlerAgent *butler.ButlerAgent) *WithButlerTools {
	return &WithButlerTools{ButlerAgent: butlerAgent}
}

func (o *WithButlerTools) Apply(config *AgentConfig) error {
	butlerTools := butler.GetButlerTools(config.Core, config.AgentCtx.UserID, o.ButlerAgent)

	for _, butlerTool := range butlerTools {
		notifyingButlerTool := config.ToolWrapper.Wrap(butlerTool)
		config.Tools = append(config.Tools, notifyingButlerTool)
	}

	return nil
}

// WithJournalTools 添加Journal工具选项
type WithJournalTools struct {
	JournalAgent *journal.JournalAgent
}

func NewWithJournalTools(journalAgent *journal.JournalAgent) *WithJournalTools {
	return &WithJournalTools{JournalAgent: journalAgent}
}

func (o *WithJournalTools) Apply(config *AgentConfig) error {
	journalTools := journal.GetJournalTools(config.Core, config.AgentCtx.SpaceID, config.AgentCtx.UserID, o.JournalAgent)

	for _, journalTool := range journalTools {
		notifyingJournalTool := config.ToolWrapper.Wrap(journalTool)
		config.Tools = append(config.Tools, notifyingJournalTool)
	}

	return nil
}

// === 模型相关选项 ===

// WithModelOverride 覆盖默认模型配置
type WithModelOverride struct {
	ModelConfig *types.ModelConfig
}

func NewWithModelOverride(modelConfig *types.ModelConfig) *WithModelOverride {
	return &WithModelOverride{ModelConfig: modelConfig}
}

func (o *WithModelOverride) Apply(config *AgentConfig) error {
	config.ModelOverride = o.ModelConfig
	return nil
}

// === 行为相关选项 ===

// WithMaxIterations 设置最大工具调用迭代次数
type WithMaxIterations struct {
	MaxIterations int
}

func NewWithMaxIterations(maxIterations int) *WithMaxIterations {
	return &WithMaxIterations{MaxIterations: maxIterations}
}

func (o *WithMaxIterations) Apply(config *AgentConfig) error {
	config.MaxIterations = o.MaxIterations
	return nil
}

// WithCustomPrompts 添加自定义系统提示词
type WithCustomPrompts struct {
	Prompts []string
}

func NewWithCustomPrompts(prompts ...string) *WithCustomPrompts {
	return &WithCustomPrompts{Prompts: prompts}
}

func (o *WithCustomPrompts) Apply(config *AgentConfig) error {
	config.CustomPrompts = append(config.CustomPrompts, o.Prompts...)
	return nil
}

// CreateReActAgentWithConfig 使用AgentConfig创建ReAct Agent (新的通用方法)
func (f *EinoAgentFactory) CreateReActAgentWithConfig(config *AgentConfig) (*react.Agent, *types.ModelConfig, error) {
	var err error

	var chatModel types.ChatModel
	// 如果有模型覆盖配置，则使用它
	if config.ModelOverride != nil {
		if chatModel, err = srv.SetupAIDriver(context.Background(), *config.ModelOverride); err != nil {
			return nil, nil, err
		}
	} else {
		// 检查消息中是否包含多媒体内容，决定使用哪种模型
		if f.containsMultimediaContent(config.Messages) {
			chatModel = config.Core.Srv().AI().GetVisionAI()
		} else {
			chatModel = config.Core.Srv().AI().GetChatAI(config.EnableThinking)
		}
	}

	toolsConfig := compose.ToolsNodeConfig{
		Tools: config.Tools,
	}

	// 使用配置中的StreamChecker，如果没有则使用默认的
	// streamChecker := func(ctx context.Context, modelOutput *schema.StreamReader[*schema.Message]) (bool, error) {
	// 	defer modelOutput.Close()
	// 	for {
	// 		res, err := modelOutput.Recv()
	// 		if err != nil {
	// 			return false, nil
	// 		}

	// 		if res.ResponseMeta.FinishReason == "tool_calls" {
	// 			return true, nil
	// 		}
	// 	}
	// }

	// 创建 ReAct Agent
	agentConfig := &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      toolsConfig,
		MessageModifier:  nil, // 禁用 MessageModifier，使用工具内部通知机制
		//StreamToolCallChecker: streamChecker,
		// MaxStep: 2,
	}

	agent, err := react.NewAgent(config.AgentCtx, agentConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create ReAct agent: %w", err)
	}

	modelConfig := chatModel.Config()
	return agent, &modelConfig, nil
}

// EinoResponseHandler 处理 eino Agent 的响应
type EinoResponseHandler struct {
	_receiveFunc types.ReceiveFunc
	_doneFunc    types.DoneFunc
	_receiver    types.Receiver
	reqMsg       *types.ChatMessage
}

// NewEinoResponseHandler 创建响应处理器
func NewEinoResponseHandler(receiver types.Receiver, reqMsg *types.ChatMessage) *EinoResponseHandler {
	return &EinoResponseHandler{
		_receiver: receiver,
		reqMsg:    reqMsg,
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
func (h *EinoResponseHandler) HandleStreamResponse(ctx context.Context, stream *schema.StreamReader[*model.CallbackOutput], needToCreateMessage func(ctx context.Context) error) error {
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

		startThinking    = sync.Once{}
		hasThinking      = false
		finishedThinking = sync.Once{}
		// toolCalls []*schema.ToolCall // 使用 eino 原生结构
	)

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

		_msg, err := stream.Recv()
		msg := _msg.Message
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
			if hasThinking {
				finishedThinking.Do(func() {
					strs.WriteString("</think>")
				})
			}
			strs.WriteString(msg.Content)

			// 处理隐藏标记
			if machedMarks && strings.Contains(msg.Content, "]") {
				preStr := strs.String()
				resultStr := h._receiver.VariableHandler().Undo(preStr)
				if preStr != resultStr {
					strs.Reset()
					strs.WriteString(resultStr)
				}
				maybeMarks = false
				machedMarks = false
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

func NewCallbackHandlers(core *core.Core, modelName string, streamHandler *EinoResponseHandler) callbacks.Handler {
	reqMessage := streamHandler.reqMsg
	initFunc := func(ctx context.Context) error {
		defer slog.Debug("AI message session created",
			slog.String("msg_id", streamHandler.Receiver().MessageID()),
			slog.String("session_id", reqMessage.SessionID))
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

		return nil
	}

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
			raw, _ := json.Marshal(input)
			fmt.Println("onStart", string(raw))
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
				initFunc(ctx)
				streamHandler.GetDoneFunc(nil)(err)
			}
			return ctx
		},
		OnEndWithStreamOutput: func(ctx context.Context, runInfo *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
			// 处理流式输出
			infoRaw, _ := json.Marshal(runInfo)
			slog.Debug("eino callback stream output", slog.String("info", string(infoRaw)), slog.String("model_name", modelName), slog.String("message_id", reqMessage.ID))

			if output == nil || runInfo.Name != react.ModelNodeName {
				if runInfo.Name != react.ModelNodeName {
					recvUnknown, err := output.Recv()
					if err != nil {
						slog.Error("failed to recv unknown stream output", slog.Any("error", err), slog.String("message_id", reqMessage.ID))
					} else {
						raw, err := json.Marshal(recvUnknown)
						if err != nil {
							fmt.Println("uuuuu error", err)
						} else {
							fmt.Println("uuuuu", string(raw))
						}
					}
				}
				return ctx
			}

			if err := streamHandler.HandleStreamResponse(ctx, output, initFunc); err != nil {
				slog.Error("failed to handle stream response", slog.Any("error", err), slog.String("message_id", reqMessage.ID))
				return ctx
			}

			return ctx
		},
	}).Handler()
	// .Tool(&callbackhelper.ToolCallbackHandler{})
}
