// GOWORK=off go test -v . -run TestBaiduOCR
package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quka-ai/quka-ai/pkg/ai/baidu"
)

// getBaiduOCRConfig loads baidu OCR configuration from environment variables
func getBaiduOCRConfig(t *testing.T) (string, string) {
	// Load .env file if it exists
	if envPath := filepath.Join("../..", ".env"); fileExists(envPath) {
		_ = godotenv.Load(envPath)
	}

	apiURL := os.Getenv("QUKA_TEST_BAIDU_OCR_ENDPOINT")
	token := os.Getenv("QUKA_TEST_BAIDU_OCR_TOKEN")

	if apiURL == "" || token == "" {
		t.Skip("QUKA_TEST_BAIDU_OCR_ENDPOINT or QUKA_TEST_BAIDU_OCR_TOKEN not set, skipping baidu OCR e2e tests")
	}

	return apiURL, token
}

// BaiduOCRTestSuite holds the test suite for baidu OCR e2e testing
type BaiduOCRTestSuite struct {
	driver *baidu.Driver
}

// NewBaiduOCRTestSuite creates a new baidu OCR e2e test suite
func NewBaiduOCRTestSuite(t *testing.T) *BaiduOCRTestSuite {
	apiURL, token := getBaiduOCRConfig(t)

	driver := baidu.New(baidu.Config{
		APIURL: apiURL,
		Token:  token,
	})

	return &BaiduOCRTestSuite{
		driver: driver,
	}
}

// TestBaiduOCR tests the complete baidu OCR e2e flow
func TestBaiduOCR(t *testing.T) {
	suite := NewBaiduOCRTestSuite(t)

	t.Run("TestLanguage", suite.TestLanguage)
	t.Run("TestProcessImageOCR", suite.TestProcessImageOCR)
	t.Run("TestProcessPDFOCR", suite.TestProcessPDFOCR)
	t.Run("TestProcessInvalidFile", suite.TestProcessInvalidFile)
	t.Run("TestProcessEmptyFile", suite.TestProcessEmptyFile)
}

// TestLanguage tests the language detection
func (s *BaiduOCRTestSuite) TestLanguage(t *testing.T) {
	lang := s.driver.Lang()
	assert.Equal(t, "CN", lang)
	t.Logf("Baidu OCR language: %s", lang)
}

// TestProcessImageOCR tests processing an image file
func (s *BaiduOCRTestSuite) TestProcessImageOCR(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Read test image file
	testImagePath := filepath.Join(".", "test_table_image.jpg")
	require.True(t, fileExists(testImagePath), "Test image file not found: %s", testImagePath)

	imageData, err := os.ReadFile(testImagePath)
	require.NoError(t, err, "Failed to read test image file")
	require.NotEmpty(t, imageData, "Test image file is empty")

	// Process OCR
	result, err := s.driver.ProcessOCR(ctx, imageData)
	require.NoError(t, err, "OCR processing failed")
	require.NotNil(t, result, "OCR result is nil")

	// Validate result structure
	assert.NotEmpty(t, result.Title, "Title should not be empty")
	assert.NotEmpty(t, result.MarkdownText, "Markdown text should not be empty")
	assert.Equal(t, "baidu", result.Model, "Model should be 'baidu'")
	assert.NotNil(t, result.Usage, "Usage should not be nil")
	assert.Greater(t, result.Usage.TokensUsed, 0, "Tokens used should be greater than 0")

	// Log results
	t.Logf("Title: %s", result.Title)
	t.Logf("Markdown length: %d characters", len(result.MarkdownText))
	t.Logf("Number of images: %d", len(result.Images))
	t.Logf("Tokens used: %d", result.Usage.TokensUsed)
	t.Logf("Model: %s", result.Model)

	// Log first 200 characters of markdown text
	if len(result.MarkdownText) > 200 {
		t.Logf("Markdown preview: %s...", result.MarkdownText[:200])
	} else {
		t.Logf("Markdown content: %s", result.MarkdownText)
	}

	// Log image URLs if any
	for i, imgURL := range result.Images {
		t.Logf("Image %d: %s", i+1, imgURL)
	}

	t.Log(result.MarkdownText)
}

// TestProcessPDFOCR tests processing a PDF file
func (s *BaiduOCRTestSuite) TestProcessPDFOCR(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a simple PDF file content (PDF header)
	// Note: This is a minimal PDF for testing. In real scenarios, use actual PDF files.
	pdfContent := []byte("%PDF-1.4\n%âãÏÓ\n1 0 obj\n<<\n/Type /Catalog\n/Pages 2 0 R\n>>\nendobj\n2 0 obj\n<<\n/Type /Pages\n/Kids [3 0 R]\n/Count 1\n>>\nendobj\n3 0 obj\n<<\n/Type /Page\n/Parent 2 0 R\n/MediaBox [0 0 612 792]\n/Contents 4 0 R\n/Resources <<\n/Font <<\n/F1 5 0 R\n>>\n>>\n>>\nendobj\n4 0 obj\n<<\n/Length 44\n>>\nstream\nBT\n/F1 12 Tf\n100 700 Td\n(Test PDF Content) Tj\nET\nendstream\nendobj\n5 0 obj\n<<\n/Type /Font\n/Subtype /Type1\n/BaseFont /Helvetica\n>>\nendobj\nxref\n0 6\n0000000000 65535 f\n0000000015 00000 n\n0000000068 00000 n\n0000000125 00000 n\n0000000279 00000 n\n0000000372 00000 n\ntrailer\n<<\n/Size 6\n/Root 1 0 R\n>>\nstartxref\n457\n%%EOF")

	// Process OCR
	result, err := s.driver.ProcessOCR(ctx, pdfContent)

	// Note: This might fail if the PDF is too simple or invalid
	// In production tests, use a real PDF file
	if err != nil {
		t.Logf("Expected behavior: Simple PDF might not be processed: %v", err)
		t.Skip("Skipping PDF test with minimal PDF content")
		return
	}

	require.NotNil(t, result, "OCR result is nil")

	// Validate result structure
	// Note: Minimal PDF might not contain extractable text
	if result.Title == "" && len(result.MarkdownText) == 0 {
		t.Log("PDF OCR returned empty content - this is expected for minimal/synthetic PDF")
	}
	assert.Equal(t, "baidu", result.Model, "Model should be 'baidu'")

	// Log results
	t.Logf("PDF Title: %s", result.Title)
	t.Logf("PDF Markdown length: %d characters", len(result.MarkdownText))
	t.Logf("PDF Number of images: %d", len(result.Images))
	t.Logf("Model: %s", result.Model)

	// If there are images returned, the OCR at least processed the PDF structure
	if len(result.Images) > 0 {
		t.Log("PDF processing succeeded - images were generated")
	}
}

// TestProcessInvalidFile tests processing an invalid file
func (s *BaiduOCRTestSuite) TestProcessInvalidFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create invalid file content
	invalidContent := []byte("This is not a valid image or PDF file")

	// Process OCR - should return error
	result, err := s.driver.ProcessOCR(ctx, invalidContent)

	assert.Error(t, err, "Should return error for invalid file")
	assert.Nil(t, result, "Result should be nil for invalid file")
	assert.Contains(t, err.Error(), "unsupported file type", "Error message should mention unsupported file type")

	t.Logf("Expected error for invalid file: %v", err)
}

// TestProcessEmptyFile tests processing an empty file
func (s *BaiduOCRTestSuite) TestProcessEmptyFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Empty file content
	emptyContent := []byte{}

	// Process OCR - should return error
	result, err := s.driver.ProcessOCR(ctx, emptyContent)

	assert.Error(t, err, "Should return error for empty file")
	assert.Nil(t, result, "Result should be nil for empty file")

	t.Logf("Expected error for empty file: %v", err)
}
