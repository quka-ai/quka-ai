package v1

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/safe"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/types/protocol"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// EinoMessageLifecycleCallback Eino 消息生命周期回调管理器
type EinoMessageLifecycleCallback struct {
	callbacks.HandlerBuilder // 嵌入 eino HandlerBuilder

	core           *core.Core
	userReqMessage *types.ChatMessage
	aiMessage      *types.ChatMessage // 代表一次完整的 AI 响应会话，在 Agent OnStart 中创建

	modelConfig *types.ModelConfig
	handler     *EinoResponseHandler
	mutex       sync.RWMutex
}

// 消息生命周期范围说明:
// - aiMessage 代表一次完整的 RequestAssistant() 调用产生的 AI 响应
// - aiMessage 的内容是基于工具调用结果生成的最终回答，但不直接包含工具调用过程
// - Agent OnStart -> 创建 aiMessage (MESSAGE_PROGRESS_GENERATING)
// - Agent OnEnd -> 完成 aiMessage (MESSAGE_PROGRESS_COMPLETE)
// - 工具调用会创建独立的工具消息记录，用于展示执行过程，但最终结果体现在 aiMessage 中

// NewEinoMessageLifecycleCallback 创建 Eino 消息生命周期回调管理器
func NewEinoMessageLifecycleCallback(core *core.Core, userReqMsg *types.ChatMessage, ext types.ChatMessageExt, receiver types.Receiver) *EinoMessageLifecycleCallback {
	return &EinoMessageLifecycleCallback{
		core:           core,
		userReqMessage: userReqMsg,
	}
}

// isAgentComponent 检查是否为 Agent 组件
func (c *EinoMessageLifecycleCallback) isAgentComponent(info *callbacks.RunInfo) bool {
	// eino ReAct Agent 的组件名通常是 "react.Agent"
	return info.Name == "react.Agent" || info.Type == "agent"
}

// OnStart 实现 eino callback - 在 Agent 开始时创建消息
func (c *EinoMessageLifecycleCallback) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	// 只在 Agent 组件开始时创建消息（Agent 级别代表整个对话会话）
	if !c.isAgentComponent(info) {
		return ctx
	}

	// 创建 assistant 消息记录，代表一次完整的 AI 响应会话
	// (替代原来在 RequestAssistant 中的 InitAssistantMessage 调用)
	msgID := utils.GenUniqIDStr()

	// 获取正确的消息序号（在会话中的顺序序号）
	seqID, err := c.core.Plugins.GetChatSessionSeqID(ctx, c.userReqMessage.SpaceID, c.userReqMessage.SessionID)
	if err != nil {
		slog.Error("failed to get chat message sequence in Agent OnStart callback", slog.Any("error", err))
		return ctx
	}

	msgExt := types.ChatMessageExt{
		SpaceID:   c.userReqMessage.SpaceID,
		SessionID: c.userReqMessage.SessionID,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	aiMessage, err := initAssistantMessage(ctx, c.core, msgID, seqID, c.userReqMessage, msgExt)
	if err != nil {
		slog.Error("failed to init assistant message in Agent OnStart callback", slog.Any("error", err))
		return ctx
	}

	c.mutex.Lock()
	c.aiMessage = aiMessage
	c.mutex.Unlock()

	// 发送初始化消息通知
	DefaultMessager(protocol.GenIMTopic(c.userReqMessage.SessionID), c.core.Srv().Tower()).
		PublishMessage(types.WS_EVENT_ASSISTANT_INIT, chatMsgToTextMsg(c.aiMessage))

	slog.Debug("AI message session created",
		slog.String("msg_id", msgID),
		slog.String("session_id", c.userReqMessage.SessionID))

	return ctx
}

func (c *EinoMessageLifecycleCallback) OnEndWithStreamOutput(ctx context.Context, runInfo *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
	// 处理流式输出
	if output == nil {
		return ctx
	}

	infoRaw, _ := json.Marshal(runInfo)
	slog.Debug("eino callback stream output", slog.String("info", string(infoRaw)), slog.String("model_name", c.modelConfig.ModelName), slog.String("message_id", c.userReqMessage.ID))

	go safe.Run(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
		defer cancel()
		if err := c.handler.HandleStreamResponse(ctx, output); err != nil {
			slog.Error("failed to handle stream response", slog.Any("error", err), slog.String("model_name", c.modelConfig.ModelName), slog.String("message_id", c.userReqMessage.ID))
			return
		}
	})

	return ctx
}

// OnEnd 实现 eino callback - 在 Agent 结束时完成消息
func (c *EinoMessageLifecycleCallback) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	// 只在 Agent 组件结束时完成消息（整个对话会话结束）
	if !c.isAgentComponent(info) {
		return ctx
	}

	c.mutex.RLock()
	aiMessage := c.aiMessage
	c.mutex.RUnlock()

	if aiMessage == nil {
		slog.Warn("aiMessage is nil in Agent OnEnd callback, session may not have been properly initialized")
		return ctx
	}

	// 更新消息状态为完成 (替代原来的 handler.doneFunc)
	// 此时整个 AI 响应会话已完成，包括所有工具调用和最终回答
	if err := c.core.Store().ChatMessageStore().UpdateMessageCompleteStatus(
		ctx, aiMessage.SessionID, aiMessage.ID, int32(types.MESSAGE_PROGRESS_COMPLETE)); err != nil {
		slog.Error("failed to update message complete status in Agent OnEnd callback", slog.Any("error", err))
		return ctx
	}

	c.handler.doneFunc(nil)

	slog.Debug("AI message session completed",
		slog.String("msg_id", aiMessage.ID),
		slog.String("session_id", aiMessage.SessionID))

	return ctx
}

// OnError 实现 eino callback - 在 Agent 出错时处理消息失败
func (c *EinoMessageLifecycleCallback) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	// 只在 Agent 组件出错时处理消息失败（整个对话会话失败）
	if !c.isAgentComponent(info) {
		return ctx
	}

	c.mutex.RLock()
	aiMessage := c.aiMessage
	c.mutex.RUnlock()

	if aiMessage != nil {
		// 更新消息状态为失败
		if updateErr := c.core.Store().ChatMessageStore().UpdateMessageCompleteStatus(
			ctx, aiMessage.SessionID, aiMessage.ID, int32(types.MESSAGE_PROGRESS_FAILED)); updateErr != nil {
			slog.Error("failed to update message failed status in Agent OnError callback",
				slog.Any("original_error", err),
				slog.Any("update_error", updateErr))
		}

		slog.Error("AI message session failed",
			slog.String("msg_id", aiMessage.ID),
			slog.String("session_id", aiMessage.SessionID),
			slog.Any("error", err))
	}

	// 直接调用已保存的 doneFunc（错误场景）
	c.handler.doneFunc(err)

	return ctx
}
