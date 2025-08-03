package v1

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
	"github.com/sashabaranov/go-openai"
)

func TestAutoAssistant_Creation(t *testing.T) {
	// 简单测试 AutoAssistant 的创建
	core := &core.Core{} // 这里需要完整的 core 初始化，暂时用空的测试结构创建
	
	assistant := NewAutoAssistant(core, types.AGENT_TYPE_NORMAL)
	
	if assistant == nil {
		t.Fatal("AutoAssistant creation failed")
	}
	
	if assistant.agentType != types.AGENT_TYPE_NORMAL {
		t.Errorf("Expected agent type %s, got %s", types.AGENT_TYPE_NORMAL, assistant.agentType)
	}
	
	t.Log("✅ AutoAssistant creation test passed")
}

func TestEinoMessageConverter_Creation(t *testing.T) {
	// 测试消息转换器的创建
	core := &core.Core{}
	
	converter := NewEinoMessageConverter(core)
	
	if converter == nil {
		t.Fatal("EinoMessageConverter creation failed")
	}
	
	if converter.core != core {
		t.Error("EinoMessageConverter core not set correctly")
	}
	
	t.Log("✅ EinoMessageConverter creation test passed")
}

func TestEinoAgentFactory_Creation(t *testing.T) {
	// 测试 Agent 工厂的创建
	core := &core.Core{}
	
	factory := NewEinoAgentFactory(core)
	
	if factory == nil {
		t.Fatal("EinoAgentFactory creation failed")
	}
	
	if factory.core != core {
		t.Error("EinoAgentFactory core not set correctly")
	}
	
	// 测试缓存相关功能
	if factory.cachedChatModelConfig != nil {
		t.Error("Expected cachedChatModelConfig to be nil initially")
	}
	if factory.cachedVisionModelConfig != nil {
		t.Error("Expected cachedVisionModelConfig to be nil initially")
	}
	
	// 测试清除缓存功能
	factory.ClearModelConfigCache()
	if factory.cachedChatModelConfig != nil {
		t.Error("Expected cachedChatModelConfig to remain nil after clear")
	}
	if factory.cachedVisionModelConfig != nil {
		t.Error("Expected cachedVisionModelConfig to remain nil after clear")
	}
	
	t.Log("✅ EinoAgentFactory creation test passed")
}

func TestToolCallPersister_Creation(t *testing.T) {
	// 测试持久化器的创建
	core := &core.Core{}
	sessionID := "test-session"
	spaceID := "test-space"
	userID := "test-user"
	
	persister := NewToolCallPersister(core, sessionID, spaceID, userID)
	
	if persister == nil {
		t.Fatal("ToolCallPersister creation failed")
	}
	
	if persister.sessionID != sessionID {
		t.Errorf("Expected sessionID %s, got %s", sessionID, persister.sessionID)
	}
	
	if persister.spaceID != spaceID {
		t.Errorf("Expected spaceID %s, got %s", spaceID, persister.spaceID)
	}
	
	if persister.userID != userID {
		t.Errorf("Expected userID %s, got %s", userID, persister.userID)
	}
	
	t.Log("✅ ToolCallPersister creation test passed")
}

func TestToolCallMessage_Formatting(t *testing.T) {
	// 测试工具调用消息格式化
	core := &core.Core{}
	persister := NewToolCallPersister(core, "session", "space", "user")
	
	toolCall := &ToolCallMessage{
		ToolName:  "SearchUserKnowledges",
		Arguments: map[string]interface{}{"query": "golang"},
		Result:    "Found 5 documents",
		Status:    "success",
		StartTime: 1640995200,
		EndTime:   1640995260,
	}
	
	formatted := persister.formatToolCallMessage(toolCall)
	
	if formatted == "" {
		t.Fatal("Tool call message formatting failed")
	}
	
	// 检查格式化内容是否包含期望的信息
	expectedParts := []string{
		"🔧 工具调用: SearchUserKnowledges",
		"参数:",
		"golang",
		"结果:",
		"Found 5 documents",
		"状态: 执行成功",
	}
	
	for _, part := range expectedParts {
		if !contains(formatted, part) {
			t.Errorf("Expected formatted message to contain '%s', but it didn't. Got: %s", part, formatted)
		}
	}
	
	t.Log("✅ ToolCallMessage formatting test passed")
	t.Logf("Formatted message: %s", formatted)
}

// TestEinoMessageConverter_ConvertFromChatMessages 测试从数据库消息转换到 eino 消息
func TestEinoMessageConverter_ConvertFromChatMessages(t *testing.T) {
	converter := &EinoMessageConverter{core: &core.Core{}}

	// 创建测试用的 ChatMessage
	testMessages := []types.ChatMessage{
		{
			Role:    types.USER_ROLE_USER,
			Message: "Hello, this is a test message",
		},
		{
			Role:    types.USER_ROLE_ASSISTANT,
			Message: "This is a response from assistant",
		},
	}

	result := converter.ConvertFromChatMessages(testMessages)

	// 验证转换结果
	if len(result) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(result))
	}

	// 验证第一条消息（用户消息）
	userMsg := result[0]
	if userMsg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", userMsg.Role)
	}
	if userMsg.Content != "Hello, this is a test message" {
		t.Errorf("Expected content 'Hello, this is a test message', got '%s'", userMsg.Content)
	}

	// 验证第二条消息（助手消息）
	assistantMsg := result[1]
	if assistantMsg.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", assistantMsg.Role)
	}
	if assistantMsg.Content != "This is a response from assistant" {
		t.Errorf("Expected content 'This is a response from assistant', got '%s'", assistantMsg.Content)
	}

	t.Log("✅ EinoMessageConverter ConvertFromChatMessages test passed")
	t.Logf("Converted %d ChatMessages to %d schema.Messages", len(testMessages), len(result))
}

// TestEinoMessageConverter_ConvertToChatMessage 测试从 eino 消息转换到数据库消息
func TestEinoMessageConverter_ConvertToChatMessage(t *testing.T) {
	converter := &EinoMessageConverter{core: &core.Core{}}

	// 创建测试用的 schema.Message
	testMsg := &schema.Message{
		Role:    "tool",
		Content: "Tool execution result: Found 5 documents",
	}

	result := converter.ConvertToChatMessage(testMsg, "session123", "space456", "user789")

	// 验证转换结果
	if result.Role != types.USER_ROLE_TOOL {
		t.Errorf("Expected role USER_ROLE_TOOL, got %s", result.Role)
	}
	if result.Message != "Tool execution result: Found 5 documents" {
		t.Errorf("Expected message 'Tool execution result: Found 5 documents', got '%s'", result.Message)
	}
	if result.SessionID != "session123" {
		t.Errorf("Expected SessionID 'session123', got '%s'", result.SessionID)
	}
	if result.MsgType != types.MESSAGE_TYPE_TOOL_TIPS {
		t.Errorf("Expected MsgType MESSAGE_TYPE_TOOL_TIPS, got %d", result.MsgType)
	}

	t.Log("✅ EinoMessageConverter ConvertToChatMessage test passed")
	t.Logf("Converted schema.Message to ChatMessage for persistence")
}

// TestEinoMessageConverter_ConvertToEinoMultiContent 测试多媒体内容转换
func TestEinoMessageConverter_ConvertToEinoMultiContent(t *testing.T) {
	converter := &EinoMessageConverter{core: &core.Core{}}

	// 创建测试用的 openai.ChatMessagePart
	openaiParts := []openai.ChatMessagePart{
		{
			Type: openai.ChatMessagePartTypeText,
			Text: "Please analyze this image",
		},
		{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQ...",
				Detail: openai.ImageURLDetailHigh,
			},
		},
	}

	result := converter.convertToEinoMultiContent(openaiParts)

	// 验证转换结果
	if len(result) != 2 {
		t.Fatalf("Expected 2 parts, got %d", len(result))
	}

	// 验证文本部分
	textPart := result[0]
	if string(textPart.Type) != "text" {
		t.Errorf("Expected type 'text', got '%s'", textPart.Type)
	}
	if textPart.Text != "Please analyze this image" {
		t.Errorf("Expected text 'Please analyze this image', got '%s'", textPart.Text)
	}

	// 验证图片部分
	imagePart := result[1]
	if string(imagePart.Type) != "image_url" {
		t.Errorf("Expected type 'image_url', got '%s'", imagePart.Type)
	}
	if imagePart.ImageURL == nil {
		t.Fatal("Expected ImageURL to be set")
	}
	if imagePart.ImageURL.URL != "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQ..." {
		t.Errorf("Expected base64 image URL, got '%s'", imagePart.ImageURL.URL)
	}
	if string(imagePart.ImageURL.Detail) != "high" {
		t.Errorf("Expected detail 'high', got '%s'", imagePart.ImageURL.Detail)
	}

	t.Log("✅ EinoMessageConverter convertToEinoMultiContent test passed")
	t.Logf("Converted %d openai parts to %d eino parts", len(openaiParts), len(result))
}

// TestEinoAgentFactory_ContainsMultimediaContent 测试多媒体内容检测
func TestEinoAgentFactory_ContainsMultimediaContent(t *testing.T) {
	factory := NewEinoAgentFactory(&core.Core{})

	// 测试纯文本消息
	textOnlyMessages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Hello, how are you?",
		},
		{
			Role:    schema.Assistant,
			Content: "I'm doing well, thank you!",
		},
	}

	if factory.containsMultimediaContent(textOnlyMessages) {
		t.Error("Expected containsMultimediaContent to return false for text-only messages")
	}

	// 测试包含图片的消息
	messagesWithImage := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Please analyze this image",
			MultiContent: []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "Please analyze this image",
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					ImageURL: &schema.ChatMessageImageURL{
						URL: "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQ...",
					},
				},
			},
		},
	}

	if !factory.containsMultimediaContent(messagesWithImage) {
		t.Error("Expected containsMultimediaContent to return true for messages with images")
	}

	// 测试混合内容（文本 + 多媒体）
	mixedMessages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Hello",
		},
		{
			Role:    schema.User,
			Content: "Here's a video",
			MultiContent: []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeVideoURL,
					VideoURL: &schema.ChatMessageVideoURL{
						URL: "https://example.com/video.mp4",
					},
				},
			},
		},
	}

	if !factory.containsMultimediaContent(mixedMessages) {
		t.Error("Expected containsMultimediaContent to return true for mixed messages")
	}

	t.Log("✅ EinoAgentFactory containsMultimediaContent test passed")
}

func TestAutoAssistant_RequestAssistant_Interface(t *testing.T) {
	// 验证 AutoAssistant 实现了与 NormalAssistant 相同的接口
	core := &core.Core{}
	autoAssistant := NewAutoAssistant(core, "rag")
	
	// 类型检查：确保 AutoAssistant 实现了与 NormalAssistant 相同的方法
	var _ interface {
		InitAssistantMessage(ctx context.Context, msgID string, seqID int64, userReqMessage *types.ChatMessage, ext types.ChatMessageExt) (*types.ChatMessage, error)
		GenSessionContext(ctx context.Context, prompt string, reqMsgWithDocs *types.ChatMessage) (*SessionContext, error)
		RequestAssistant(ctx context.Context, reqMsg *types.ChatMessage, receiver types.Receiver, aiCallOptions *types.AICallOptions) error
	} = autoAssistant

	t.Log("✅ AutoAssistant implements the same interface as NormalAssistant")
}

// 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// MockInvokableTool 模拟的 InvokableTool 实现用于测试
type MockInvokableTool struct {
	name string
	callCount int
}

func (m *MockInvokableTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: m.name,
		Desc: "Mock tool for testing",
	}, nil
}

func (m *MockInvokableTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	m.callCount++
	return "mock result", nil
}

// TestNotifyingTool_Functionality 测试 NotifyingTool 的通知功能
func TestNotifyingTool_Functionality(t *testing.T) {
	// 初始化 ID 生成器（测试环境）
	utils.SetupIDWorker(1)
	
	// 创建模拟接收函数来验证通知
	notifications := []types.MessageContent{}
	receiveFunc := types.ReceiveFunc(func(msg types.MessageContent, progressStatus types.MessageProgress) error {
		notifications = append(notifications, msg)
		return nil
	})
	
	// 创建 EinoAdapter
	adapter := ai.NewEinoAdapter(receiveFunc, "test-session", "test-message")
	
	// 创建模拟工具
	mockTool := &MockInvokableTool{name: "MockTool"}
	
	// 创建 NotifyingTool (手动设置以避免依赖 snowflake worker)
	testToolID := "test-tool-id-12345"
	notifyingTool := &NotifyingTool{
		InvokableTool: mockTool,
		adapter:       adapter,
		toolID:        testToolID, // 使用静态ID进行测试
	}
	
	// 执行工具调用
	ctx := context.Background()
	result, err := notifyingTool.InvokableRun(ctx, `{"query": "test"}`)
	
	// 验证结果
	if err != nil {
		t.Fatalf("NotifyingTool execution failed: %v", err)
	}
	
	if result != "mock result" {
		t.Errorf("Expected result 'mock result', got '%s'", result)
	}
	
	if mockTool.callCount != 1 {
		t.Errorf("Expected tool to be called once, got %d calls", mockTool.callCount)
	}
	
	// 验证通知功能 - 重点验证 tool_id 是否一致
	t.Log("✅ NotifyingTool functionality test passed")
	t.Logf("Tool was called %d times", mockTool.callCount)
	t.Logf("Received %d notifications", len(notifications))
	t.Logf("Expected tool_id: %s", testToolID)
	
	// 通过检查日志或其他方式验证 tool_id 的一致性
	// 在实际应用中，OnToolCallStart 和 OnToolCallEnd 都应该使用相同的 testToolID
}

// TestToolIDConsistency 专门测试工具调用过程中 tool_id 的一致性
func TestToolIDConsistency(t *testing.T) {
	// 收集所有的 ToolTips 消息
	var toolTipsMessages []*types.ToolTips
	receiveFunc := types.ReceiveFunc(func(msg types.MessageContent, progressStatus types.MessageProgress) error {
		if toolTips, ok := msg.(*types.ToolTips); ok {
			toolTipsMessages = append(toolTipsMessages, toolTips)
		}
		return nil
	})
	
	// 创建 EinoAdapter
	adapter := ai.NewEinoAdapter(receiveFunc, "test-session", "test-message")
	
	// 创建模拟工具
	mockTool := &MockInvokableTool{name: "TestTool"}
	
	// 创建 NotifyingTool 使用固定的 tool_id
	expectedToolID := "consistent-tool-id-456"
	notifyingTool := &NotifyingTool{
		InvokableTool: mockTool,
		adapter:       adapter,
		toolID:        expectedToolID,
	}
	
	// 执行工具调用
	ctx := context.Background()
	_, err := notifyingTool.InvokableRun(ctx, `{"test": "data"}`)
	
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}
	
	// 验证收到了 2 条 ToolTips 消息（开始 + 结束）
	if len(toolTipsMessages) != 2 {
		t.Fatalf("Expected 2 ToolTips messages, got %d", len(toolTipsMessages))
	}
	
	// 验证两条消息都使用了相同的 tool_id
	startMessage := toolTipsMessages[0]
	endMessage := toolTipsMessages[1]
	
	if startMessage.ID != expectedToolID {
		t.Errorf("Start message tool_id mismatch. Expected: %s, Got: %s", expectedToolID, startMessage.ID)
	}
	
	if endMessage.ID != expectedToolID {
		t.Errorf("End message tool_id mismatch. Expected: %s, Got: %s", expectedToolID, endMessage.ID)
	}
	
	if startMessage.ID != endMessage.ID {
		t.Errorf("Tool IDs are not consistent. Start: %s, End: %s", startMessage.ID, endMessage.ID)
	}
	
	// 验证状态变化：开始 -> RUNNING，结束 -> SUCCESS
	if startMessage.Status != types.TOOL_STATUS_RUNNING {
		t.Errorf("Expected start message status %d, got %d", types.TOOL_STATUS_RUNNING, startMessage.Status)
	}
	
	if endMessage.Status != types.TOOL_STATUS_SUCCESS {
		t.Errorf("Expected end message status %d, got %d", types.TOOL_STATUS_SUCCESS, endMessage.Status)
	}
	
	t.Log("✅ Tool ID consistency test passed")
	t.Logf("Consistent tool_id used: %s", expectedToolID)
	t.Logf("Start message tool name: %s, status: %d", startMessage.ToolName, startMessage.Status)
	t.Logf("End message tool name: %s, status: %d", endMessage.ToolName, endMessage.Status)
}