package v1

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func TestAutoAssistant_Creation(t *testing.T) {
	// 简单测试 AutoAssistant 的创建
	core := &core.Core{} // 这里需要完整的 core 初始化，暂时用空的测试结构创建

	assistant := NewAutoAssistant(core, types.AGENT_TYPE_AUTO)

	if assistant == nil {
		t.Fatal("AutoAssistant creation failed")
	}

	if assistant.agentType != types.AGENT_TYPE_AUTO {
		t.Errorf("Expected agent type %s, got %s", types.AGENT_TYPE_AUTO, assistant.agentType)
	}

	t.Log("✅ AutoAssistant creation test passed")
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

	persister := NewToolCallSaver(core, sessionID, spaceID, userID)

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

// TestEinoMessageConverter_ConvertFromChatMessages 测试从数据库消息转换到 eino 消息
func TestEinoMessageConverter_ConvertFromChatMessages(t *testing.T) {
	// 创建测试用的 ChatMessage
	testMessages := []*types.MessageContext{
		{
			Role:    types.USER_ROLE_USER,
			Content: "Hello, this is a test message",
		},
		{
			Role:    types.USER_ROLE_ASSISTANT,
			Content: "This is a response from assistant",
		},
	}

	result := ai.ConvertMessageContextToEinoMessages(testMessages)

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

// TestHiddenSyntaxDetection 测试 $hidden[...] 语法检测逻辑
func TestHiddenSyntaxDetection(t *testing.T) {
	tests := []struct {
		name            string
		streamChunks    []string // 模拟流式接收的消息块
		expectedMatched bool     // 是否应该匹配到 $hidden[
		expectedFinal   string   // 最终经过 Undo 处理后的预期结果（如果匹配的话）
		description     string
	}{
		{
			name: "完整的密码隐藏场景",
			streamChunks: []string{
				"根据",
				"您提供的知识",
				"库记录",
				"，`",
				"tools@edgen",
				"ext.com` 的",
				"邮箱密码已被系统脱敏处理",
				"，具体信息如下：\n\n-",
				" **邮箱地址**：`",
				"tools@edgen",
				"ext.com`\n-",
				" **密码**：",
				"`$hidden[g",
				"wG45",
				"s3mhV",
				"]`\n- **",
				"登录要求**：需",
			},
			expectedMatched: true,
			description:     "模拟真实场景中流式接收包含 $hidden[...] 的密码",
		},
		{
			name: "多个$符号但不匹配hidden语法",
			streamChunks: []string{
				"价格是 $",
				"100，折扣",
				" $20",
				"，总共 $",
				"80",
			},
			expectedMatched: false,
			description:     "多个 $ 符号但都不是 $hidden[ 语法",
		},
		{
			name: "包含$hidden但分多次接收",
			streamChunks: []string{
				"密码：",
				"$",
				"h",
				"i",
				"d",
				"d",
				"e",
				"n",
				"[",
				"test123",
				"]",
			},
			expectedMatched: true,
			description:     "$hidden[ 被拆分成多个字符逐个接收",
		},
		{
			name: "混合场景：普通$和hidden语法",
			streamChunks: []string{
				"价格 $50，",
				"密码 $hidden[abc",
				"123]，结束",
			},
			expectedMatched: true,
			description:     "既有普通的 $ 又有 $hidden[ 语法",
		},
		{
			name: "$后面跟其他文本",
			streamChunks: []string{
				"这是 $test",
				" 文本",
			},
			expectedMatched: false,
			description:     "$ 后面不是 hidden[",
		},
		{
			name: "不完整的hidden语法",
			streamChunks: []string{
				"密码：$hidd",
				"en but no bracket",
			},
			expectedMatched: false,
			description:     "有 $hidd 但不完整，不构成 $hidden[",
		},
		{
			name: "单个消息块包含完整$hidden[前缀",
			streamChunks: []string{
				"关于",
				"邮箱",
				" `tools@ed",
				"genext.com`",
				" 的密码，根据",
				"您的记忆库记录",
				"，相关信息如下：\n\n- **",
				"邮箱账号**：`tools",
				"@edgenext",
				".com`  \n- **密码",
				"**：`$hidden[I",
				"8lWBBl",
				"f4R]",
				"`  \n- **",
			},
			expectedMatched: true,
			description:     "第二次真实场景：一个消息块中包含 **：`$hidden[I",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟变量
			var (
				maybeMarks  bool
				machedMarks bool
				strs        strings.Builder
			)

			// 模拟 flush 函数
			flushResponse := func() {
				if strs.Len() > 0 {
					t.Logf("      [FLUSH] 发送内容: %q (长度=%d)", strs.String(), strs.Len())
					strs.Reset()
				}
			}

			// 模拟流式处理每个消息块
			for i, chunk := range tt.streamChunks {
				t.Logf("    [%d] 收到消息块: %q", i, chunk)

				// 模拟检测 $ 符号的逻辑
				if !maybeMarks {
					if strings.Contains(chunk, "$") {
						maybeMarks = true
						if strs.Len() != 0 {
							flushResponse()
						}
						t.Logf("      检测到 $，设置 maybeMarks=true")
					}
				}

				// 写入内容（模拟 line 1021: strs.WriteString(msg.Content)）
				strs.WriteString(chunk)
				t.Logf("      写入后 strs: %q", strs.String())

				// 在写入后检查是否形成 $hidden[ 语法（模拟 line 1024-1057）
				if maybeMarks && !machedMarks {
					currentStr := strs.String()
					if strings.Contains(currentStr, "$hidden[") {
						machedMarks = true
					} else {
						// 检查所有 $ 位置
						dollarIdx := 0
						hasValidCandidate := false
						for {
							idx := strings.Index(currentStr[dollarIdx:], "$")
							if idx == -1 {
								break
							}
							dollarIdx += idx
							remainingStr := currentStr[dollarIdx:]
							remainingLen := len(remainingStr)

							if remainingLen < len("$hidden[") {
								hasValidCandidate = true
								break
							}

							dollarIdx++
						}

						if !hasValidCandidate {
							maybeMarks = false
						}
					}
				}

				// 模拟遇到 ] 时的处理（这里简化，不真正调用 Undo）
				if machedMarks && strings.Contains(chunk, "]") {
					// 在真实场景中会调用 h._receiver.VariableHandler().Undo(preStr)
					// 这里我们只验证状态
					break
				}
			}

			// 验证结果
			if machedMarks != tt.expectedMatched {
				t.Errorf("%s:\n  期望 machedMarks=%v, 实际=%v\n  最终字符串: %q",
					tt.description, tt.expectedMatched, machedMarks, strs.String())
			} else {
				t.Logf("✅ %s: 通过 (machedMarks=%v, 内容长度=%d)",
					tt.description, machedMarks, strs.Len())
			}

			// 额外的调试信息
			if machedMarks {
				t.Logf("   检测到 $hidden[ 语法，缓冲区内容: %q", strs.String())
			}
		})
	}
}

// TestHiddenSyntaxEdgeCases 测试边界情况
func TestHiddenSyntaxEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldMatch bool
	}{
		{"空字符串", "", false},
		{"只有$", "$", false},
		{"$h", "$h", false},
		{"$hi", "$hi", false},
		{"$hid", "$hid", false},
		{"$hidd", "$hidd", false},
		{"$hidde", "$hidde", false},
		{"$hidden", "$hidden", false},
		{"$hidden[", "$hidden[", true},
		{"$hidden[]", "$hidden[]", true},
		{"$hidden[password]", "$hidden[password]", true},
		{"text $hidden[secret] more", "text $hidden[secret] more", true},
		{"$hidden $hidden[test]", "$hidden $hidden[test]", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strings.Contains(tt.input, "$hidden[")
			if result != tt.shouldMatch {
				t.Errorf("输入 %q: 期望匹配=%v, 实际=%v", tt.input, tt.shouldMatch, result)
			}
		})
	}
}

// MockInvokableTool 模拟的 InvokableTool 实现用于测试
type MockInvokableTool struct {
	name      string
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
