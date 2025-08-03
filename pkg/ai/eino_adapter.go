package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"

	"github.com/cloudwego/eino/schema"
)

// genSimpleID 生成简单的ID（测试用）
func genSimpleID() string {
	return strconv.FormatInt(time.Now().UnixNano()+int64(rand.Intn(1000)), 10)
}

// EinoToolCallRecorder 用于记录 eino 框架中的工具调用过程到现有聊天系统
type EinoToolCallRecorder struct {
	receiveFunc types.ReceiveFunc // 现有的消息接收函数
	sessionID   string            // 当前会话ID
	messageID   string            // 当前消息ID
}

// NewEinoToolCallRecorder 创建新的 eino 工具调用记录器
func NewEinoToolCallRecorder(receiveFunc types.ReceiveFunc, sessionID, messageID string) *EinoToolCallRecorder {
	return &EinoToolCallRecorder{
		receiveFunc: receiveFunc,
		sessionID:   sessionID,
		messageID:   messageID,
	}
}

// RecordToolTips 记录工具调用状态提示（通过 WebSocket 推送）
func (r *EinoToolCallRecorder) RecordToolTips(toolName, toolID, content string, status int) error {
	if r.receiveFunc == nil {
		return nil
	}

	toolTips := &types.ToolTips{
		ID:       toolID,
		ToolName: toolName,
		Status:   status,
		Content:  content,
	}

	return r.receiveFunc(toolTips, types.MESSAGE_PROGRESS_GENERATING)
}

// CreateMessageModifier 创建 eino MessageModifier，拦截和记录工具相关消息
func (r *EinoToolCallRecorder) CreateMessageModifier() func(context.Context, []*schema.Message) []*schema.Message {
	return func(ctx context.Context, messages []*schema.Message) []*schema.Message {
		// 遍历消息，找到工具调用相关的消息进行记录
		for _, msg := range messages {
			switch msg.Role {
			case schema.Tool:
				// 工具执行结果消息
				toolID := genSimpleID()
				r.RecordToolTips("tool_result", toolID, fmt.Sprintf("工具执行结果: %s", msg.Content), types.TOOL_STATUS_SUCCESS)

			case schema.Assistant:
				// 检查是否包含工具调用
				if len(msg.ToolCalls) > 0 {
					for _, toolCall := range msg.ToolCalls {
						toolID := genSimpleID()
						r.RecordToolTips(toolCall.Function.Name, toolID,
							fmt.Sprintf("准备调用工具: %s", toolCall.Function.Name),
							types.TOOL_STATUS_RUNNING)
					}
				}
			}
		}

		return messages
	}
}

// EinoToolCallInterceptor eino 工具调用拦截器
type EinoToolCallInterceptor struct {
	recorder *EinoToolCallRecorder
}

// NewEinoToolCallInterceptor 创建工具调用拦截器
func NewEinoToolCallInterceptor(recorder *EinoToolCallRecorder) *EinoToolCallInterceptor {
	return &EinoToolCallInterceptor{
		recorder: recorder,
	}
}

// InterceptToolCall 拦截工具调用过程
func (i *EinoToolCallInterceptor) InterceptToolCall(toolName string, toolCall *schema.ToolCall) func() {
	toolID := genSimpleID()

	// 记录工具调用开始
	i.recorder.RecordToolTips(toolName, toolID,
		fmt.Sprintf("开始执行工具: %s，参数: %s", toolName, toolCall.Function.Arguments),
		types.TOOL_STATUS_RUNNING)

	// 返回结束函数
	return func() {
		i.recorder.RecordToolTips(toolName, toolID,
			fmt.Sprintf("工具执行完成: %s", toolName),
			types.TOOL_STATUS_SUCCESS)
	}
}

// EinoAdapter eino 框架适配器，专门用于集成现有聊天系统
type EinoAdapter struct {
	recorder    *EinoToolCallRecorder
	interceptor *EinoToolCallInterceptor
}

// NewEinoAdapter 创建 eino 适配器
func NewEinoAdapter(receiveFunc types.ReceiveFunc, sessionID, messageID string) *EinoAdapter {
	recorder := NewEinoToolCallRecorder(receiveFunc, sessionID, messageID)
	return &EinoAdapter{
		recorder:    recorder,
		interceptor: NewEinoToolCallInterceptor(recorder),
	}
}

// GetMessageModifier 获取消息修改器，用于 eino agent 配置
func (a *EinoAdapter) GetMessageModifier() func(context.Context, []*schema.Message) []*schema.Message {
	return a.recorder.CreateMessageModifier()
}

// RecordToolCall 手动记录工具调用（用于自定义工具）
func (a *EinoAdapter) RecordToolCall(toolName, content string, status int) error {
	toolID := genSimpleID()
	return a.recorder.RecordToolTips(toolName, toolID, content, status)
}

// OnToolCallStart 工具调用开始时调用
func (a *EinoAdapter) OnToolCallStart(toolName string, args interface{}) error {
	// 从 args 中提取 tool_id，如果没有则生成新的
	toolMessageID := utils.GenUniqIDStr()

	argsJSON, _ := json.Marshal(args)
	return a.recorder.RecordToolTips(toolName, toolMessageID, fmt.Sprintf("开始执行: %s", string(argsJSON)), types.TOOL_STATUS_RUNNING)
}

// OnToolCallEnd 工具调用结束时调用
func (a *EinoAdapter) OnToolCallEnd(toolName string, result interface{}, err error) error {
	// 从 result 中提取 tool_id，如果没有则生成新的
	var toolID string
	if resultMap, ok := result.(map[string]interface{}); ok {
		if id, exists := resultMap["tool_id"]; exists {
			if idStr, ok := id.(string); ok {
				toolID = idStr
			}
		}
	}

	if toolID == "" {
		toolID = genSimpleID()
	}

	if err != nil {
		return a.recorder.RecordToolTips(toolName, toolID, fmt.Sprintf("执行失败: %v", err), types.TOOL_STATUS_FAILED)
	}

	resultJSON, _ := json.Marshal(result)
	return a.recorder.RecordToolTips(toolName, toolID, fmt.Sprintf("执行成功: %s", string(resultJSON)), types.TOOL_STATUS_SUCCESS)
}

// SaveToolCallToHistory 将工具调用保存为聊天历史记录
func (a *EinoAdapter) SaveToolCallToHistory(toolName string, toolCall *schema.ToolCall, result string) error {
	// TODO: 实现将工具调用记录保存到聊天历史中，对应数据库 chat_message 表
	return nil
}
