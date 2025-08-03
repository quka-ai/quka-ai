package v1

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// ToolCallState 工具调用状态
type ToolCallState struct {
	MessageID string
	ToolName  string
	StartTime time.Time
}

// EinoToolLifecycleCallback 工具调用生命周期回调管理器
type EinoToolLifecycleCallback struct {
	callbacks.HandlerBuilder

	persister       *ToolCallPersister
	parentMessage   *types.ChatMessage
	activeToolCalls map[string]*ToolCallState // tool_id -> state  
	mutex           sync.RWMutex
}

// NewEinoToolLifecycleCallback 创建工具调用生命周期回调管理器
func NewEinoToolLifecycleCallback(core *core.Core, sessionID, spaceID, userID string) *EinoToolLifecycleCallback {
	persister := NewToolCallPersister(core, sessionID, spaceID, userID)
	
	return &EinoToolLifecycleCallback{
		persister:       persister,
		activeToolCalls: make(map[string]*ToolCallState),
	}
}

// isToolComponent 检查是否为工具组件
func (c *EinoToolLifecycleCallback) isToolComponent(info *callbacks.RunInfo) bool {
	return info.Type == "tool"
}

// generateToolID 生成工具调用唯一ID
func (c *EinoToolLifecycleCallback) generateToolID(info *callbacks.RunInfo) string {
	// 使用组件名 + 时间戳生成唯一ID
	return info.Name + "_" + utils.GenUniqIDStr()
}

// OnStart 实现 eino callback - 工具调用开始
func (c *EinoToolLifecycleCallback) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if !c.isToolComponent(info) {
		return ctx
	}

	toolName := info.Name
	toolID := c.generateToolID(info)

	// 创建工具调用记录
	msgID, err := c.persister.SaveToolCallStart(ctx, toolName, input)
	if err != nil {
		slog.Error("failed to save tool call start", 
			slog.String("tool_name", toolName),
			slog.String("tool_id", msgID),
			slog.Any("error", err))
		return ctx
	}

	c.mutex.Lock()
	c.activeToolCalls[toolID] = &ToolCallState{
		MessageID: msgID,
		ToolName:  toolName,
		StartTime: time.Now(),
	}
	c.mutex.Unlock()

	slog.Debug("tool call started",
		slog.String("tool_name", toolName),
		slog.String("tool_id", toolID),
		slog.String("msg_id", msgID))

	return ctx
}

// OnEnd 实现 eino callback - 工具调用完成
func (c *EinoToolLifecycleCallback) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if !c.isToolComponent(info) {
		return ctx
	}

	toolID := c.generateToolID(info)

	c.mutex.Lock()
	toolState := c.activeToolCalls[toolID]
	delete(c.activeToolCalls, toolID)
	c.mutex.Unlock()

	if toolState != nil {
		// 更新工具调用完成记录
		if err := c.persister.SaveToolCallComplete(ctx, toolState.MessageID, output, true); err != nil {
			slog.Error("failed to save tool call complete",
				slog.String("tool_name", toolState.ToolName),
				slog.String("tool_id", toolID),
				slog.String("msg_id", toolState.MessageID),
				slog.Any("error", err))
		} else {
			slog.Debug("tool call completed",
				slog.String("tool_name", toolState.ToolName),
				slog.String("tool_id", toolID),
				slog.String("msg_id", toolState.MessageID))
		}
	} else {
		slog.Warn("tool call end without matching start",
			slog.String("tool_name", info.Name),
			slog.String("tool_id", toolID))
	}

	return ctx
}

// OnError 实现 eino callback - 工具调用出错
func (c *EinoToolLifecycleCallback) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if !c.isToolComponent(info) {
		return ctx
	}

	toolID := c.generateToolID(info)

	c.mutex.Lock()
	toolState := c.activeToolCalls[toolID]
	delete(c.activeToolCalls, toolID)
	c.mutex.Unlock()

	if toolState != nil {
		// 更新工具调用失败记录
		if updateErr := c.persister.SaveToolCallComplete(ctx, toolState.MessageID, err.Error(), false); updateErr != nil {
			slog.Error("failed to save tool call error",
				slog.String("tool_name", toolState.ToolName),
				slog.String("tool_id", toolID),
				slog.String("msg_id", toolState.MessageID),
				slog.Any("original_error", err),
				slog.Any("update_error", updateErr))
		} else {
			slog.Error("tool call failed",
				slog.String("tool_name", toolState.ToolName),
				slog.String("tool_id", toolID),
				slog.String("msg_id", toolState.MessageID),
				slog.Any("error", err))
		}
	} else {
		slog.Error("tool call error without matching start",
			slog.String("tool_name", info.Name),
			slog.String("tool_id", toolID),
			slog.Any("error", err))
	}

	return ctx
}