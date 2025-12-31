package vision

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type mockVisionModel struct{}

func (m *mockVisionModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return &schema.Message{
		Role:    schema.Assistant,
		Content: "This is a test image analysis result.",
	}, nil
}

func (m *mockVisionModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (m *mockVisionModel) BindTools(tools []*schema.ToolInfo) error {
	return nil
}

func (m *mockVisionModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *mockVisionModel) Config() types.ModelConfig {
	return types.ModelConfig{
		ModelName: "test-vision-model",
	}
}

func TestVisionTool(t *testing.T) {
	ctx := context.Background()
	mock := &mockVisionModel{}

	tool, err := NewTool(ctx, mock)
	if err != nil {
		t.Fatalf("Failed to create vision tool: %v", err)
	}

	info, err := tool.Info(ctx)
	if err != nil {
		t.Fatalf("Failed to get tool info: %v", err)
	}

	if info.Name != "vision" {
		t.Errorf("Expected tool name 'vision', got '%s'", info.Name)
	}

	if info.Desc == "" {
		t.Error("Tool description should not be empty")
	}

	// Test that batch processing is mentioned in description
	if !strings.Contains(info.Desc, "批量") {
		t.Error("Tool description should mention batch processing capability")
	}
}

func TestVisionToolSingleImage(t *testing.T) {
	ctx := context.Background()
	mock := &mockVisionModel{}

	tool, err := NewTool(ctx, mock)
	if err != nil {
		t.Fatalf("Failed to create vision tool: %v", err)
	}

	// Test single image request (using array with one element)
	singleRequest := VisionRequest{
		ImageURLs: []string{"https://example.com/image.jpg"},
		Question:  "What's in this image?",
	}

	argsJSON, err := json.Marshal(singleRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	result, err := tool.InvokableRun(ctx, string(argsJSON))
	if err != nil {
		t.Fatalf("Failed to run vision tool: %v", err)
	}

	if result != "This is a test image analysis result." {
		t.Errorf("Unexpected result: %s", result)
	}
}

func TestVisionToolBatchProcessing(t *testing.T) {
	ctx := context.Background()
	mock := &mockVisionModel{}

	tool, err := NewTool(ctx, mock)
	if err != nil {
		t.Fatalf("Failed to create vision tool: %v", err)
	}

	// Test batch request with multiple images
	batchRequest := VisionRequest{
		ImageURLs: []string{
			"https://example.com/image1.jpg",
			"https://example.com/image2.jpg",
			"https://example.com/image3.jpg",
		},
		Question: "What's in these images?",
	}

	argsJSON, err := json.Marshal(batchRequest)
	if err != nil {
		t.Fatalf("Failed to marshal batch request: %v", err)
	}

	result, err := tool.InvokableRun(ctx, string(argsJSON))
	if err != nil {
		t.Fatalf("Failed to run vision tool with batch: %v", err)
	}

	// Verify batch result contains multiple image sections
	if !strings.Contains(result, "Image 1 Analysis") || !strings.Contains(result, "Image 2 Analysis") {
		t.Errorf("Batch result should contain multiple image analyses: %s", result)
	}
}

func TestVisionToolEmptyURLs(t *testing.T) {
	ctx := context.Background()
	mock := &mockVisionModel{}

	tool, err := NewTool(ctx, mock)
	if err != nil {
		t.Fatalf("Failed to create vision tool: %v", err)
	}

	// Test empty URLs array
	emptyRequest := VisionRequest{
		ImageURLs: []string{},
		Question:  "What's in this image?",
	}

	argsJSON, err := json.Marshal(emptyRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	_, err = tool.InvokableRun(ctx, string(argsJSON))
	if err == nil {
		t.Error("Expected error for empty image_urls")
	}

	if !strings.Contains(err.Error(), "at least one URL") {
		t.Errorf("Expected 'at least one URL' error, got: %v", err)
	}
}
