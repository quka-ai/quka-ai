package v1

import (
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/quka-ai/quka-ai/pkg/types"
)

// MockReceiver 测试用的接收器
type MockReceiver struct {
	mock.Mock
}

func (m *MockReceiver) IsStream() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockReceiver) GetReceiveFunc() types.ReceiveFunc {
	args := m.Called()
	return args.Get(0).(types.ReceiveFunc)
}

func (m *MockReceiver) GetDoneFunc(callback func(msg *types.ChatMessage)) types.DoneFunc {
	args := m.Called(callback)
	return args.Get(0).(types.DoneFunc)
}

func (m *MockReceiver) RecvMessageInit(userReqMsg *types.ChatMessage, msgID string, seqID int64, ext types.ChatMessageExt) error {
	args := m.Called(userReqMsg, msgID, seqID, ext)
	return args.Error(0)
}

func TestEinoMessageLifecycleCallback_ComponentFilter(t *testing.T) {
	// 创建测试消息
	userMsg := &types.ChatMessage{
		ID:        "test-user-msg",
		SessionID: "test-session",
		SpaceID:   "test-space",
		UserID:    "test-user",
	}

	mockReceiver := &MockReceiver{}
	
	// 模拟 GetReceiveFunc 和 GetDoneFunc 调用
	mockReceiver.On("GetReceiveFunc").Return(func(msg types.MessageContent, progress types.MessageProgress) error {
		return nil
	})
	mockReceiver.On("GetDoneFunc", mock.AnythingOfType("func(*types.ChatMessage)")).Return(func(err error) error {
		return nil
	})

	callback := NewEinoMessageLifecycleCallback(nil, userMsg, types.ChatMessageExt{}, mockReceiver)

	// 测试 Agent 组件识别
	agentInfo := &callbacks.RunInfo{
		Name: "react.Agent",
		Type: "agent",
	}
	assert.True(t, callback.isAgentComponent(agentInfo))

	// 测试非 Agent 组件
	modelInfo := &callbacks.RunInfo{
		Name: "openai.ChatModel",
		Type: "model",
	}
	assert.False(t, callback.isAgentComponent(modelInfo))
}

func TestEinoToolLifecycleCallback_ComponentFilter(t *testing.T) {
	toolCallback := NewEinoToolLifecycleCallback(nil, "test-session", "test-space", "test-user")

	// 测试工具组件识别
	toolInfo := &callbacks.RunInfo{
		Name: "duckduckgo.Tool",
		Type: "tool",
	}
	assert.True(t, toolCallback.isToolComponent(toolInfo))

	// 测试非工具组件
	modelInfo := &callbacks.RunInfo{
		Name: "openai.ChatModel",
		Type: "model",
	}
	assert.False(t, toolCallback.isToolComponent(modelInfo))
}

func TestEnhancedCallback_Creation(t *testing.T) {
	userMsg := &types.ChatMessage{
		ID:        "test-user-msg",
		SessionID: "test-session",
		SpaceID:   "test-space",
		UserID:    "test-user",
	}

	mockReceiver := &MockReceiver{}
	
	// 模拟方法调用
	mockReceiver.On("GetReceiveFunc").Return(func(msg types.MessageContent, progress types.MessageProgress) error {
		return nil
	})
	mockReceiver.On("GetDoneFunc", mock.AnythingOfType("func(*types.ChatMessage)")).Return(func(err error) error {
		return nil
	})

	lifecycleCallback := NewEinoMessageLifecycleCallback(nil, userMsg, types.ChatMessageExt{}, mockReceiver)
	toolCallback := NewEinoToolLifecycleCallback(nil, "test-session", "test-space", "test-user")

	// 创建增强回调处理器
	handler := NewEnhancedEinoCallbackHandlers(
		"test-model",
		"test-msg-id",
		lifecycleCallback,
		toolCallback,
		nil, // responseHandler 可以为 nil 在测试中
	)

	// 验证返回的是正确的类型
	assert.NotNil(t, handler)
	
	// 验证实现了正确的接口
	_, ok := handler.(callbacks.Handler)
	assert.True(t, ok, "Enhanced callback should implement callbacks.Handler interface")
}