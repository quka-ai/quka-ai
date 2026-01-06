package baidu

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	config := Config{
		APIURL: os.Getenv("QUKA_TEST_BAIDU_OCR_ENDPOINT"),
		Token:  os.Getenv("QUKA_TEST_BAIDU_OCR_TOKEN"),
	}

	driver := New(config)

	assert.NotNil(t, driver)
	assert.Equal(t, config.APIURL, driver.apiURL)
	assert.Equal(t, config.Token, driver.token)
	assert.NotNil(t, driver.client)
}

func TestLang(t *testing.T) {
	driver := &Driver{}
	assert.Equal(t, "CN", driver.Lang())
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		results  []LayoutParsingResult
		expected string
	}{
		{
			name:     "empty results",
			results:  []LayoutParsingResult{},
			expected: "OCR Document",
		},
		{
			name: "short text",
			results: []LayoutParsingResult{
				{
					Markdown: MarkdownResult{
						Text: "Hello World",
					},
				},
			},
			expected: "Hello World",
		},
		{
			name: "long text",
			results: []LayoutParsingResult{
				{
					Markdown: MarkdownResult{
						Text: "This is a very long text that should be truncated because it exceeds fifty characters in length",
					},
				},
			},
			expected: "This is a very long text that should be truncated ...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTitle(tt.results)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractImageURLs(t *testing.T) {
	results := []LayoutParsingResult{
		{
			Markdown: MarkdownResult{
				Images: map[string]string{
					"img1": "https://example.com/img1.jpg",
					"img2": "https://example.com/img2.jpg",
				},
			},
			OutputImages: map[string]string{
				"output1": "https://example.com/output1.jpg",
			},
		},
		{
			Markdown: MarkdownResult{
				Images: map[string]string{
					"img3": "https://example.com/img3.jpg",
				},
			},
		},
	}

	imageURLs := extractImageURLs(results)

	assert.Len(t, imageURLs, 4)
	assert.Contains(t, imageURLs, "https://example.com/img1.jpg")
	assert.Contains(t, imageURLs, "https://example.com/img2.jpg")
	assert.Contains(t, imageURLs, "https://example.com/img3.jpg")
	assert.Contains(t, imageURLs, "https://example.com/output1.jpg")
}

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "PDF file",
			data:     []byte("%PDF-1.4\n"),
			expected: "pdf",
		},
		{
			name:     "PNG image",
			data:     []byte("\x89PNG\r\n\x1a\n"),
			expected: "image",
		},
		{
			name:     "JPEG image",
			data:     []byte{0xFF, 0xD8, 0xFF},
			expected: "image",
		},
		{
			name:     "GIF image",
			data:     []byte("GIF87a"),
			expected: "image",
		},
		{
			name:     "BMP image",
			data:     []byte{0x42, 0x4D},
			expected: "image",
		},
		{
			name:     "Unknown file",
			data:     []byte("unknown"),
			expected: "unknown",
		},
		{
			name:     "Empty data",
			data:     []byte{},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectFileType(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}
