package ai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	ddg "github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/joho/godotenv"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/ai/tools/duckduckgo"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func createPrompt() *prompt.DefaultChatTemplate {
	return prompt.FromMessages(schema.FString,
		schema.SystemMessage(ai.GENERATE_PROMPT_TPL_CN),
		schema.MessagesPlaceholder("chat_history", false))
}

func TestBaseChat(t *testing.T) {
	// 加载 .env 文件
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Logf("Warning: Could not load .env file: %v", err)
	}

	// 从环境变量获取 OpenAI 配置，使用 TEST_ 前缀的配置
	apiKey := os.Getenv("TEST_OPENAI_API_KEY")
	baseURL := os.Getenv("TEST_OPENAI_ENDPOINT")
	model := os.Getenv("TEST_OPENAI_MODEL")

	if apiKey == "" {
		t.Skip("TEST_OPENAI_API_KEY not set, skipping test")
	}

	if model == "" {
		model = "gpt-3.5-turbo" // 默认模型
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// 创建 OpenAI 聊天模型
	toolableChatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	duckduckgoTool, err := duckduckgo.NewTool(ctx, ddg.RegionCN)
	if err != nil {
		t.Fatal(err)
	}

	tools := compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{duckduckgoTool},
	}

	ra, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: toolableChatModel,
		ToolsConfig:      tools,

		MessageModifier: func(ctx context.Context, input []*schema.Message) []*schema.Message {
			inputMessages, _ := json.Marshal(input)
			fmt.Println("input message:", string(inputMessages))
			return input
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	resp, err := ra.Stream(ctx, []*schema.Message{schema.UserMessage("看下golang 1.25的新特性")}, agent.WithComposeOptions())
	if err != nil {
		t.Fatal(err)
	}

	result, _ := json.Marshal(resp)
	t.Log("result", string(result))
}

// TestEinoWithToolCallRecording 测试 eino 集成工具调用记录
func TestEinoWithToolCallRecording(t *testing.T) {
	// 加载 .env 文件
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Logf("Warning: Could not load .env file: %v", err)
	}

	// 从环境变量获取 OpenAI 配置
	apiKey := os.Getenv("TEST_OPENAI_API_KEY")
	baseURL := os.Getenv("TEST_OPENAI_ENDPOINT")
	model := os.Getenv("TEST_OPENAI_MODEL")

	if apiKey == "" {
		t.Skip("TEST_OPENAI_API_KEY not set, skipping test")
	}

	if model == "" {
		model = "gpt-3.5-turbo"
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// 模拟现有聊天系统的接收函数
	var recordedMessages []string
	var recordedToolTips []string
	
	receiveFunc := func(msg types.MessageContent, progressStatus types.MessageProgress) error {
		switch msg.Type() {
		case types.MESSAGE_TYPE_TEXT:
			content := string(msg.Bytes())
			recordedMessages = append(recordedMessages, content)
			t.Logf("📝 记录消息: %s", content)
		case types.MESSAGE_TYPE_TOOL_TIPS:
			content := string(msg.Bytes())
			recordedToolTips = append(recordedToolTips, content)
			t.Logf("🔧 工具提示: %s", content)
		}
		return nil
	}

	// 创建 eino 适配器
	sessionID := "test-session-123"
	messageID := "test-message-456"
	einoAdapter := ai.NewEinoAdapter(receiveFunc, sessionID, messageID)

	// 创建 OpenAI 聊天模型
	toolableChatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 创建工具
	duckduckgoTool, err := duckduckgo.NewTool(ctx, ddg.RegionCN)
	if err != nil {
		t.Fatal(err)
	}

	tools := compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{duckduckgoTool},
	}

	// 创建 ReAct Agent，使用我们的 MessageModifier 来记录工具调用
	ra, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: toolableChatModel,
		ToolsConfig:      tools,
		MessageModifier:  einoAdapter.GetMessageModifier(), // 使用适配器的 MessageModifier
	})

	if err != nil {
		t.Fatal(err)
	}

	// 手动记录开始
	einoAdapter.RecordToolCall("agent_start", "开始处理用户请求", types.TOOL_STATUS_RUNNING)

	// 执行对话
	resp, err := ra.Stream(ctx, []*schema.Message{schema.UserMessage("搜索一下 golang 1.23 的新特性")}, agent.WithComposeOptions())
	if err != nil {
		t.Fatal(err)
	}

	// 手动记录结束
	einoAdapter.RecordToolCall("agent_complete", "请求处理完成", types.TOOL_STATUS_SUCCESS)

	// 输出结果
	t.Logf("🎯 Agent 响应: %v", resp)
	t.Logf("📊 记录的消息数量: %d", len(recordedMessages))
	t.Logf("🔧 记录的工具提示数量: %d", len(recordedToolTips))

	// 验证工具调用被正确记录
	if len(recordedToolTips) == 0 {
		t.Log("⚠️  没有记录到工具调用，可能是因为 AI 没有使用工具")
	}
}
