package srv

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func TestSetupAI_NilProvider(t *testing.T) {
	providers := []types.ModelConfig{
		{
			ID:        "test-1",
			ModelName: "test-model",
			ModelType: "chat",
			Provider:  nil, // This should not cause a panic
		},
		{
			ID:        "test-2",
			ModelName: "test-model-2",
			ModelType: "embedding",
			Provider:  nil, // This should not cause a panic
		},
	}

	usage := Usage{}

	// This should not panic
	ai, err := SetupAI(providers, []types.ModelProvider{}, usage)
	if err != nil {
		t.Fatalf("SetupAI failed: %v", err)
	}

	if ai == nil {
		t.Fatal("SetupAI returned nil AI")
	}

	// Should have empty drivers since all providers were nil
	if len(ai.chatDrivers) != 0 {
		t.Errorf("Expected 0 chat drivers, got %d", len(ai.chatDrivers))
	}
	if len(ai.embedDrivers) != 0 {
		t.Errorf("Expected 0 embed drivers, got %d", len(ai.embedDrivers))
	}
}

// MockChatModel 模拟 ChatModel 接口用于测试
type MockChatModel struct {
	modelName     string
	mockToolCalls []schema.ToolCall
	mockContent   string
	mockError     error
}

func (m *MockChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if m.mockError != nil {
		return nil, m.mockError
	}

	response := &schema.Message{
		Role:      schema.Assistant,
		Content:   m.mockContent,
		ToolCalls: m.mockToolCalls,
	}

	return response, nil
}

func (m *MockChatModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	// 对于测试，我们不需要实现真正的流式处理
	return nil, nil
}

func (m *MockChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	// 对于测试，返回自己的副本
	return m, nil
}

func (m *MockChatModel) Config() types.ModelConfig {
	return types.ModelConfig{
		ModelName: m.modelName,
	}
}

func TestAI_Summarize_Success(t *testing.T) {
	// 准备测试数据
	testDoc := "这是一篇关于人工智能发展的文章。文章讲述了AI技术在2024年的最新进展，包括大语言模型、计算机视觉和自然语言处理等领域的突破。文章发表于2024年3月15日，作者分析了这些技术对未来社会的影响。"

	// 模拟工具调用响应
	mockToolCall := schema.ToolCall{
		Function: schema.FunctionCall{
			Name: "summarize",
			Arguments: `{
				"tags": ["人工智能", "技术发展", "大语言模型"],
				"title": "2024年AI技术发展综述",
				"summary": "文章概述了2024年人工智能技术的最新进展，重点介绍了大语言模型、计算机视觉和自然语言处理领域的突破性发展。",
				"date_time": "2024-03-15"
			}`,
		},
	}

	// 创建模拟的enhance驱动
	mockEnhanceModel := &MockChatModel{
		modelName:     "test-enhance-model",
		mockToolCalls: []schema.ToolCall{mockToolCall},
	}

	// 创建AI实例
	ai := &AI{
		enhanceDrivers: map[string]types.ChatModel{
			"test": mockEnhanceModel,
		},
		enhanceDefault: mockEnhanceModel,
		usage: Usage{
			Enhance: "test",
		},
	}

	// 执行测试
	ctx := context.Background()
	result, err := ai.Summarize(ctx, &testDoc)

	// 验证结果
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	if result == nil {
		t.Fatal("Summarize returned nil result")
	}

	// 验证返回的数据
	if result.Model != "test-enhance-model" {
		t.Errorf("Expected model 'test-enhance-model', got '%s'", result.Model)
	}

	if result.Title != "2024年AI技术发展综述" {
		t.Errorf("Expected title '2024年AI技术发展综述', got '%s'", result.Title)
	}

	if len(result.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(result.Tags))
	}

	expectedTags := []string{"人工智能", "技术发展", "大语言模型"}
	for i, tag := range expectedTags {
		if i >= len(result.Tags) || result.Tags[i] != tag {
			t.Errorf("Expected tag[%d] '%s', got '%s'", i, tag, result.Tags[i])
		}
	}

	if result.Summary == "" {
		t.Error("Expected non-empty summary")
	}

	if result.DateTime != "2024-03-15" {
		t.Errorf("Expected datetime '2024-03-15', got '%s'", result.DateTime)
	}
}

func TestAI_Summarize_NoEnhanceAI(t *testing.T) {
	// 创建没有enhance驱动的AI实例
	ai := &AI{
		enhanceDrivers: map[string]types.ChatModel{},
		enhanceDefault: nil,
		usage:          Usage{},
	}

	testDoc := "test document"
	ctx := context.Background()
	result, err := ai.Summarize(ctx, &testDoc)

	// 验证错误
	if err == nil {
		t.Fatal("Expected error when enhance AI not available")
	}

	if result != nil {
		t.Error("Expected nil result when enhance AI not available")
	}

	expectedError := "enhance AI not available"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestAI_Summarize_NoToolCalls(t *testing.T) {
	testDoc := "test document"

	// 创建不返回工具调用的模拟模型
	mockEnhanceModel := &MockChatModel{
		modelName:     "test-model",
		mockToolCalls: nil, // 没有工具调用
		mockContent:   "This is a plain text response without tool calls",
	}

	ai := &AI{
		enhanceDrivers: map[string]types.ChatModel{
			"test": mockEnhanceModel,
		},
		enhanceDefault: mockEnhanceModel,
		usage: Usage{
			Enhance: "test",
		},
	}

	ctx := context.Background()
	result, err := ai.Summarize(ctx, &testDoc)

	// 验证错误
	if err == nil {
		t.Fatal("Expected error when no tool calls returned")
	}

	if result != nil {
		t.Error("Expected nil result when no tool calls returned")
	}
}

func TestAI_Chunk_Success(t *testing.T) {
	// 准备测试数据
	testDoc := "第一段：人工智能的定义和历史。人工智能（AI）是计算机科学的一个分支，致力于创建能够执行通常需要人类智能的任务的系统。\n\n第二段：机器学习的发展。机器学习作为AI的一个子领域，通过算法让计算机从数据中学习模式。\n\n第三段：深度学习的突破。深度学习使用神经网络来模拟人脑的学习过程，在图像识别和自然语言处理方面取得了重大突破。"

	// 模拟工具调用响应
	mockToolCall := schema.ToolCall{
		Function: schema.FunctionCall{
			Name: "chunk",
			Arguments: `{
				"tags": ["人工智能", "机器学习", "深度学习"],
				"title": "人工智能技术概述",
				"chunks": [
					"人工智能（AI）是计算机科学的一个分支，致力于创建能够执行通常需要人类智能的任务的系统。",
					"机器学习作为AI的一个子领域，通过算法让计算机从数据中学习模式。",
					"深度学习使用神经网络来模拟人脑的学习过程，在图像识别和自然语言处理方面取得了重大突破。"
				],
				"date_time": ""
			}`,
		},
	}

	// 创建模拟的enhance驱动
	mockEnhanceModel := &MockChatModel{
		modelName:     "test-enhance-model",
		mockToolCalls: []schema.ToolCall{mockToolCall},
	}

	// 创建AI实例
	ai := &AI{
		enhanceDrivers: map[string]types.ChatModel{
			"test": mockEnhanceModel,
		},
		enhanceDefault: mockEnhanceModel,
		usage: Usage{
			Enhance: "test",
		},
	}

	// 执行测试
	ctx := context.Background()
	result, err := ai.Chunk(ctx, &testDoc)

	// 验证结果
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if result == nil {
		t.Fatal("Chunk returned nil result")
	}

	// 验证返回的数据
	if result.Model != "test-enhance-model" {
		t.Errorf("Expected model 'test-enhance-model', got '%s'", result.Model)
	}

	if result.Title != "人工智能技术概述" {
		t.Errorf("Expected title '人工智能技术概述', got '%s'", result.Title)
	}

	if len(result.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(result.Tags))
	}

	expectedTags := []string{"人工智能", "机器学习", "深度学习"}
	for i, tag := range expectedTags {
		if i >= len(result.Tags) || result.Tags[i] != tag {
			t.Errorf("Expected tag[%d] '%s', got '%s'", i, tag, result.Tags[i])
		}
	}

	if len(result.Chunks) != 3 {
		t.Errorf("Expected 3 chunks, got %d", len(result.Chunks))
	}

	expectedChunks := []string{
		"人工智能（AI）是计算机科学的一个分支，致力于创建能够执行通常需要人类智能的任务的系统。",
		"机器学习作为AI的一个子领域，通过算法让计算机从数据中学习模式。",
		"深度学习使用神经网络来模拟人脑的学习过程，在图像识别和自然语言处理方面取得了重大突破。",
	}

	for i, chunk := range expectedChunks {
		if i >= len(result.Chunks) || result.Chunks[i] != chunk {
			t.Errorf("Expected chunk[%d] '%s', got '%s'", i, chunk, result.Chunks[i])
		}
	}

	if result.DateTime != "" {
		t.Errorf("Expected empty datetime, got '%s'", result.DateTime)
	}
}

func TestAI_Chunk_NoEnhanceAI(t *testing.T) {
	// 创建没有enhance驱动的AI实例
	ai := &AI{
		enhanceDrivers: map[string]types.ChatModel{},
		enhanceDefault: nil,
		usage:          Usage{},
	}

	testDoc := "test document"
	ctx := context.Background()
	result, err := ai.Chunk(ctx, &testDoc)

	// 验证错误
	if err == nil {
		t.Fatal("Expected error when enhance AI not available")
	}

	if result != nil {
		t.Error("Expected nil result when enhance AI not available")
	}

	expectedError := "enhance AI not available"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestAI_Chunk_NoToolCalls(t *testing.T) {
	testDoc := "test document"

	// 创建不返回工具调用的模拟模型
	mockEnhanceModel := &MockChatModel{
		modelName:     "test-model",
		mockToolCalls: nil, // 没有工具调用
		mockContent:   "This is a plain text response without tool calls",
	}

	ai := &AI{
		enhanceDrivers: map[string]types.ChatModel{
			"test": mockEnhanceModel,
		},
		enhanceDefault: mockEnhanceModel,
		usage: Usage{
			Enhance: "test",
		},
	}

	ctx := context.Background()
	result, err := ai.Chunk(ctx, &testDoc)

	// 验证错误
	if err == nil {
		t.Fatal("Expected error when no tool calls returned")
	}

	if result != nil {
		t.Error("Expected nil result when no tool calls returned")
	}
}

func TestAI_Summarize_InvalidToolCallArguments(t *testing.T) {
	testDoc := "test document"

	// 模拟无效的工具调用响应
	mockToolCall := schema.ToolCall{
		Function: schema.FunctionCall{
			Name:      "summarize",
			Arguments: `{"invalid": "json"`, // 无效的JSON
		},
	}

	mockEnhanceModel := &MockChatModel{
		modelName:     "test-model",
		mockToolCalls: []schema.ToolCall{mockToolCall},
	}

	ai := &AI{
		enhanceDrivers: map[string]types.ChatModel{
			"test": mockEnhanceModel,
		},
		enhanceDefault: mockEnhanceModel,
		usage: Usage{
			Enhance: "test",
		},
	}

	ctx := context.Background()
	result, err := ai.Summarize(ctx, &testDoc)

	// 验证错误
	if err == nil {
		t.Fatal("Expected error when tool call arguments are invalid")
	}

	if result != nil {
		t.Error("Expected nil result when tool call arguments are invalid")
	}
}

func TestAI_Chunk_InvalidToolCallArguments(t *testing.T) {
	testDoc := "test document"

	// 模拟无效的工具调用响应
	mockToolCall := schema.ToolCall{
		Function: schema.FunctionCall{
			Name:      "chunk",
			Arguments: `{"invalid": "json"`, // 无效的JSON
		},
	}

	mockEnhanceModel := &MockChatModel{
		modelName:     "test-model",
		mockToolCalls: []schema.ToolCall{mockToolCall},
	}

	ai := &AI{
		enhanceDrivers: map[string]types.ChatModel{
			"test": mockEnhanceModel,
		},
		enhanceDefault: mockEnhanceModel,
		usage: Usage{
			Enhance: "test",
		},
	}

	ctx := context.Background()
	result, err := ai.Chunk(ctx, &testDoc)

	// 验证错误
	if err == nil {
		t.Fatal("Expected error when tool call arguments are invalid")
	}

	if result != nil {
		t.Error("Expected nil result when tool call arguments are invalid")
	}
}
