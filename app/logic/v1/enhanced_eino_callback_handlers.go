package v1

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	goopenai "github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/types"
)

// NewEnhancedEinoCallbackHandlers 创建增强的 Eino 回调处理器
// 集成消息生命周期管理、工具调用管理和响应处理
func NewEnhancedEinoCallbackHandlers(
	modelName, reqMessageID string,
	lifecycleCallback *EinoMessageLifecycleCallback,
	toolCallback *EinoToolLifecycleCallback,
	responseHandler *EinoResponseHandler,
) callbacks.Handler {

	// 🔥 创建组合回调处理器，同时处理多种类型的回调
	return &EnhancedCallback{
		lifecycleCallback: lifecycleCallback,
		toolCallback:      toolCallback,
		modelName:         modelName,
		reqMessageID:      reqMessageID,
		responseHandler:   responseHandler,
	}
}

// EnhancedCallback 增强的回调处理器，实现完整的 callbacks.Handler 接口
type EnhancedCallback struct {
	callbacks.HandlerBuilder // 嵌入默认实现

	lifecycleCallback *EinoMessageLifecycleCallback
	toolCallback      *EinoToolLifecycleCallback
	modelName         string
	reqMessageID      string
	responseHandler   *EinoResponseHandler
}

// OnStart 统一的 OnStart 处理
func (c *EnhancedCallback) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	// 根据组件类型分发到不同的处理器
	if c.lifecycleCallback.isAgentComponent(info) {
		ctx = c.lifecycleCallback.OnStart(ctx, info, input)
	}
	
	if c.toolCallback.isToolComponent(info) {
		ctx = c.toolCallback.OnStart(ctx, info, input)
	}
	
	return ctx
}

// OnEnd 统一的 OnEnd 处理
func (c *EnhancedCallback) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	// Agent 消息生命周期处理
	if c.lifecycleCallback.isAgentComponent(info) {
		ctx = c.lifecycleCallback.OnEnd(ctx, info, output)
	}
	
	// Tool 调用处理
	if c.toolCallback.isToolComponent(info) {
		ctx = c.toolCallback.OnEnd(ctx, info, output)
	}
	
	// ChatModel token 统计处理
	if info.Type == "model" {
		if modelOutput, ok := output.(*model.CallbackOutput); ok {
			res := model.ConvCallbackOutput(modelOutput)
			if res.TokenUsage != nil {
				go process.NewRecordChatUsageRequest(c.modelName, types.USAGE_SUB_TYPE_CHAT, c.reqMessageID, &goopenai.Usage{
					TotalTokens:      res.TokenUsage.TotalTokens,
					PromptTokens:     res.TokenUsage.PromptTokens,
					CompletionTokens: res.TokenUsage.CompletionTokens,
				})
			}
		}
	}
	
	return ctx
}

// OnError 统一的 OnError 处理
func (c *EnhancedCallback) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	// Agent 消息生命周期处理
	if c.lifecycleCallback.isAgentComponent(info) {
		ctx = c.lifecycleCallback.OnError(ctx, info, err)
	}
	
	// Tool 调用处理
	if c.toolCallback.isToolComponent(info) {
		ctx = c.toolCallback.OnError(ctx, info, err)
	}
	
	// ChatModel 错误处理
	if info.Type == "model" {
		slog.Error("eino ChatModel callback error",
			slog.Any("error", err),
			slog.String("model_name", c.modelName),
			slog.String("message_id", c.reqMessageID))
	}
	
	return ctx
}

// OnEndWithStreamOutput 流式输出处理
func (c *EnhancedCallback) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	if info.Type == "model" && output != nil {
		infoRaw, _ := json.Marshal(info)
		slog.Debug("eino callback stream output",
			slog.String("info", string(infoRaw)),
			slog.String("model_name", c.modelName),
			slog.String("message_id", c.reqMessageID))

		// 暂时跳过流式处理的类型转换，因为类型系统较复杂
		// TODO: 需要进一步研究 eino 的类型系统来正确处理流式输出
		slog.Debug("stream output callback received, skipping detailed processing for now")
	}
	
	return ctx
}

// OnStartWithStreamInput 流式输入处理 - 实现接口要求
func (c *EnhancedCallback) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	// 目前不需要特殊处理流式输入，使用默认实现
	if input != nil {
		defer input.Close()
	}
	return ctx
}