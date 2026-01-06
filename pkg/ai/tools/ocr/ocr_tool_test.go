package ocr

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/quka-ai/quka-ai/pkg/ai/baidu"
)

type mockOCR struct{}

func (m *mockOCR) ProcessOCR(ctx context.Context, fileData []byte) (*baidu.OCRProcessResult, error) {
	return &baidu.OCRProcessResult{
		Title:        "Test Document",
		MarkdownText: "# Test Document\n\nThis is a test OCR result.",
		Images:       []string{},
		Model:        "test-model",
	}, nil
}

func (m *mockOCR) Lang() string {
	return "CN"
}

func TestOCRTool(t *testing.T) {
	ctx := context.Background()
	mock := &mockOCR{}

	tool, err := NewTool(ctx, mock)
	if err != nil {
		t.Fatalf("Failed to create OCR tool: %v", err)
	}

	info, err := tool.Info(ctx)
	if err != nil {
		t.Fatalf("Failed to get tool info: %v", err)
	}

	if info.Name != "ocr" {
		t.Errorf("Expected tool name 'ocr', got '%s'", info.Name)
	}

	if info.Desc == "" {
		t.Error("Tool description should not be empty")
	}

	// Test that batch processing is mentioned in description
	if !strings.Contains(info.Desc, "批量") {
		t.Error("Tool description should mention batch processing capability")
	}
}

func TestOCRToolSingleImage(t *testing.T) {
	ctx := context.Background()
	mock := &mockOCR{}

	tool, err := NewTool(ctx, mock)
	if err != nil {
		t.Fatalf("Failed to create OCR tool: %v", err)
	}

	// Test single image request (using array with one element)
	singleRequest := OCRRequest{
		ImageURLs: []string{
			"https://example.com/image.jpg",
		},
	}

	argsJSON, err := json.Marshal(singleRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Note: This will fail in actual execution because we can't download from example.com
	// But it tests the structure and parameter handling
	_, err = tool.InvokableRun(ctx, string(argsJSON))
	if err == nil {
		t.Error("Expected error when downloading from example.com")
	}

	// Verify error is about download, not parameter parsing
	if !strings.Contains(err.Error(), "download") {
		t.Errorf("Expected download error, got: %v", err)
	}
}

func TestOCRToolBatchProcessing(t *testing.T) {
	ctx := context.Background()
	mock := &mockOCR{}

	tool, err := NewTool(ctx, mock)
	if err != nil {
		t.Fatalf("Failed to create OCR tool: %v", err)
	}

	// Test batch request with multiple images
	batchRequest := OCRRequest{
		ImageURLs: []string{
			"https://example.com/image1.jpg",
			"https://example.com/image2.jpg",
			"https://example.com/image3.jpg",
		},
	}

	argsJSON, err := json.Marshal(batchRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Note: This will fail in actual execution because we can't download from example.com
	// But it tests the structure and parameter handling
	_, err = tool.InvokableRun(ctx, string(argsJSON))
	if err == nil {
		t.Error("Expected error when downloading from example.com")
	}

	// Verify error is about download, not parameter parsing
	if !strings.Contains(err.Error(), "download") {
		t.Errorf("Expected download error, got: %v", err)
	}
}

func TestOCRToolEmptyURLs(t *testing.T) {
	ctx := context.Background()
	mock := &mockOCR{}

	tool, err := NewTool(ctx, mock)
	if err != nil {
		t.Fatalf("Failed to create OCR tool: %v", err)
	}

	// Test empty URLs array
	emptyRequest := OCRRequest{
		ImageURLs: []string{},
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
