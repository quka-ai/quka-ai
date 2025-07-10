package utils

import (
	"errors"
	"strings"
	"testing"
)

// MockFileStorage 模拟文件存储接口用于测试
type MockFileStorage struct {
	shouldFail     bool
	failObjectPath string
}

func (m *MockFileStorage) GenGetObjectPreSignURL(objectPath string) (string, error) {
	if m.shouldFail && (m.failObjectPath == "" || m.failObjectPath == objectPath) {
		return "", errors.New("mock presign error")
	}
	return "https://s3.example.com/presigned/" + objectPath + "?signature=abc123", nil
}

func TestShouldPresignURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "Internal image URL should be presigned",
			url:      "/image/space123/photo.jpg",
			expected: true,
		},
		{
			name:     "Internal object URL should be presigned",
			url:      "/object/space123/document.pdf",
			expected: true,
		},
		{
			name:     "Internal file URL should be presigned",
			url:      "/file/space123/video.mp4",
			expected: true,
		},
		{
			name:     "Public URL should not be presigned",
			url:      "/public/logo.png",
			expected: false,
		},
		{
			name:     "External HTTP URL should not be presigned",
			url:      "http://example.com/image.jpg",
			expected: false,
		},
		{
			name:     "External HTTPS URL should not be presigned",
			url:      "https://example.com/image.jpg",
			expected: false,
		},
		{
			name:     "Base64 data URL should not be presigned",
			url:      "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==",
			expected: false,
		},
		{
			name:     "Already presigned URL with X-Amz-Algorithm should not be presigned",
			url:      "/image/space123/photo.jpg?X-Amz-Algorithm=AWS4-HMAC-SHA256",
			expected: false,
		},
		{
			name:     "Already presigned URL with Signature should not be presigned",
			url:      "/image/space123/photo.jpg?Signature=abc123",
			expected: false,
		},
		{
			name:     "Custom internal path should be presigned",
			url:      "/custom/path/file.jpg",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldPresignURL(tt.url)
			if result != tt.expected {
				t.Errorf("shouldPresignURL(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestExtractObjectPath(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Extract from image URL",
			url:      "/image/space123/photo.jpg",
			expected: "space123/photo.jpg",
		},
		{
			name:     "Extract from object URL",
			url:      "/object/space123/document.pdf",
			expected: "space123/document.pdf",
		},
		{
			name:     "Extract from file URL",
			url:      "/file/space123/video.mp4",
			expected: "space123/video.mp4",
		},
		{
			name:     "Extract with query parameters",
			url:      "/image/space123/photo.jpg?width=300&height=200",
			expected: "space123/photo.jpg",
		},
		{
			name:     "Extract with fragment",
			url:      "/image/space123/photo.jpg#section1",
			expected: "space123/photo.jpg",
		},
		{
			name:     "Extract with both query and fragment",
			url:      "/image/space123/photo.jpg?width=300#section1",
			expected: "space123/photo.jpg",
		},
		{
			name:     "Non-matching URL should return empty",
			url:      "/public/logo.png",
			expected: "",
		},
		{
			name:     "External URL should return empty",
			url:      "https://example.com/image.jpg",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractObjectPath(tt.url)
			if result != tt.expected {
				t.Errorf("extractObjectPath(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestReplaceMarkdownStaticResourcesWithPresignedURL(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		shouldFail     bool
		failObjectPath string
		expected       string
		contains       []string
	}{
		{
			name:     "Replace markdown image successfully",
			content:  "This is a test ![Test Image](/image/space123/photo.jpg) with image.",
			expected: "This is a test ![Test Image](https://s3.example.com/presigned/space123/photo.jpg?signature=abc123) with image.",
		},
		{
			name:     "Replace multiple markdown images",
			content:  "Image 1: ![First](/image/space123/first.jpg) and Image 2: ![Second](/image/space123/second.png)",
			expected: "Image 1: ![First](https://s3.example.com/presigned/space123/first.jpg?signature=abc123) and Image 2: ![Second](https://s3.example.com/presigned/space123/second.png?signature=abc123)",
		},
		{
			name:     "Replace HTML img tag successfully",
			content:  `<img src="/image/space123/photo.jpg" alt="Test Image" class="responsive">`,
			expected: `<img src="https://s3.example.com/presigned/space123/photo.jpg?signature=abc123" alt="Test Image" class="responsive">`,
		},
		{
			name:     "Skip public resources",
			content:  "![Public Image](/public/logo.png)",
			expected: "![Public Image](/public/logo.png)",
		},
		{
			name:     "Skip external URLs",
			content:  "![External Image](https://example.com/image.jpg)",
			expected: "![External Image](https://example.com/image.jpg)",
		},
		{
			name:     "Skip base64 data URLs",
			content:  "![Data Image](" + PRESIGN_FAILURE_PLACEHOLDER_IMAGE + ")",
			expected: "![Data Image](" + PRESIGN_FAILURE_PLACEHOLDER_IMAGE + ")",
		},
		{
			name:       "Handle presign failure for markdown image",
			content:    "This is a test ![Test Image](/image/space123/photo.jpg) with image.",
			shouldFail: true,
			contains:   []string{"Resource temporarily unavailable", "mock presign error"},
		},
		{
			name:       "Handle presign failure for HTML img tag",
			content:    `<img src="/image/space123/photo.jpg" alt="Test Image">`,
			shouldFail: true,
			contains:   []string{PRESIGN_FAILURE_PLACEHOLDER_IMAGE, "Resource unavailable"},
		},
		{
			name:     "Mixed content with markdown and HTML",
			content:  "Markdown: ![Test](/image/space123/md.jpg) and HTML: <img src=\"/image/space123/html.jpg\" alt=\"HTML\">",
			expected: "Markdown: ![Test](https://s3.example.com/presigned/space123/md.jpg?signature=abc123) and HTML: <img src=\"https://s3.example.com/presigned/space123/html.jpg?signature=abc123\" alt=\"HTML\">",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockFileStorage{
				shouldFail:     tt.shouldFail,
				failObjectPath: tt.failObjectPath,
			}

			result := ReplaceMarkdownStaticResourcesWithPresignedURL(tt.content, mockStorage)

			if tt.expected != "" {
				if result != tt.expected {
					t.Errorf("ReplaceMarkdownStaticResourcesWithPresignedURL() = %q, want %q", result, tt.expected)
				}
			}

			for _, containsStr := range tt.contains {
				if !strings.Contains(result, containsStr) {
					t.Errorf("Result should contain %q, but got %q", containsStr, result)
				}
			}
		})
	}
}

func TestReplaceMarkdownStaticResourcesWithPresignedURL_EdgeCases(t *testing.T) {
	mockStorage := &MockFileStorage{}

	// Test with nil fileStorage
	result := ReplaceMarkdownStaticResourcesWithPresignedURL("![Test](/image/test.jpg)", nil)
	expected := "![Test](/image/test.jpg)"
	if result != expected {
		t.Errorf("With nil fileStorage, expected %q, got %q", expected, result)
	}

	// Test with empty content
	result = ReplaceMarkdownStaticResourcesWithPresignedURL("", mockStorage)
	expected = ""
	if result != expected {
		t.Errorf("With empty content, expected %q, got %q", expected, result)
	}
}

func TestReplaceEditorJSBlocksStaticResourcesWithPresignedURL(t *testing.T) {
	tests := []struct {
		name       string
		blocksJSON string
		shouldFail bool
		expected   string
		contains   []string
	}{
		{
			name: "Replace image block URL successfully",
			blocksJSON: `{
				"blocks": [
					{
						"type": "image",
						"data": {
							"file": {
								"url": "/image/space123/photo.jpg"
							},
							"caption": "Test Image"
						}
					}
				]
			}`,
			expected: `{"blocks":[{"type":"image","data":{"file":{"url":"https://s3.example.com/presigned/space123/photo.jpg?signature=abc123"},"caption":"Test Image","withBorder":false,"withBackground":false,"stretched":false}}]}`,
		},
		{
			name: "Replace video block URL successfully",
			blocksJSON: `{
				"blocks": [
					{
						"type": "video",
						"data": {
							"file": {
								"url": "/file/space123/video.mp4"
							},
							"caption": "Test Video"
						}
					}
				]
			}`,
			expected: `{"blocks":[{"type":"video","data":{"file":{"type":"","url":"https://s3.example.com/presigned/space123/video.mp4?signature=abc123"},"caption":"Test Video","withBorder":false,"withBackground":false,"stretched":false}}]}`,
		},
		{
			name: "Replace attaches block URL successfully",
			blocksJSON: `{
				"blocks": [
					{
						"type": "attaches",
						"data": {
							"file": {
								"url": "/file/space123/document.pdf",
								"name": "document.pdf"
							}
						}
					}
				]
			}`,
			expected: `{"blocks":[{"type":"attaches","data":{"file":{"url":"https://s3.example.com/presigned/space123/document.pdf?signature=abc123","name":"document.pdf"}}}]}`,
		},
		{
			name: "Skip public resources in blocks",
			blocksJSON: `{
				"blocks": [
					{
						"type": "image",
						"data": {
							"file": {
								"url": "/public/logo.png"
							}
						}
					}
				]
			}`,
			expected: `{"blocks":[{"type":"image","data":{"file":{"url":"/public/logo.png"},"caption":"","withBorder":false,"withBackground":false,"stretched":false}}]}`,
		},
		{
			name: "Handle presign failure for image block",
			blocksJSON: `{
				"blocks": [
					{
						"type": "image",
						"data": {
							"file": {
								"url": "/image/space123/photo.jpg"
							}
						}
					}
				]
			}`,
			shouldFail: true,
			contains:   []string{PRESIGN_FAILURE_PLACEHOLDER_IMAGE},
		},
		{
			name: "Handle presign failure for video block",
			blocksJSON: `{
				"blocks": [
					{
						"type": "video",
						"data": {
							"file": {
								"url": "/file/space123/video.mp4"
							}
						}
					}
				]
			}`,
			shouldFail: true,
			contains:   []string{"/file/space123/video.mp4"},
		},
		{
			name: "Handle multiple block types",
			blocksJSON: `{
				"blocks": [
					{
						"type": "paragraph",
						"data": {
							"text": "This is a paragraph"
						}
					},
					{
						"type": "image",
						"data": {
							"file": {
								"url": "/image/space123/photo.jpg"
							}
						}
					},
					{
						"type": "video",
						"data": {
							"file": {
								"url": "/file/space123/video.mp4"
							}
						}
					}
				]
			}`,
			contains: []string{
				"This is a paragraph",
				"https://s3.example.com/presigned/space123/photo.jpg?signature=abc123",
				"https://s3.example.com/presigned/space123/video.mp4?signature=abc123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockFileStorage{shouldFail: tt.shouldFail}

			result := ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL(tt.blocksJSON, mockStorage)

			if tt.expected != "" {
				if result != tt.expected {
					t.Errorf("ReplaceEditorJSBlocksStaticResourcesWithPresignedURL() = %q, want %q", result, tt.expected)
				}
			}

			for _, containsStr := range tt.contains {
				if !strings.Contains(result, containsStr) {
					t.Errorf("Result should contain %q, but got %q", containsStr, result)
				}
			}
		})
	}
}

func TestReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL_EdgeCases(t *testing.T) {
	mockStorage := &MockFileStorage{}

	// Test with invalid JSON
	result := ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL(`{"invalid": json}`, mockStorage)
	expected := `{"invalid": json}`
	if result != expected {
		t.Errorf("With invalid JSON, expected %q, got %q", expected, result)
	}

	// Test with nil fileStorage
	result = ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL(`{"blocks":[]}`, nil)
	expected = `{"blocks":[]}`
	if result != expected {
		t.Errorf("With nil fileStorage, expected %q, got %q", expected, result)
	}

	// Test with empty string
	result = ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL("", mockStorage)
	expected = ""
	if result != expected {
		t.Errorf("With empty string, expected %q, got %q", expected, result)
	}

	// Test with JSON without blocks
	result = ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL(`{"version": "2.0"}`, mockStorage)
	expected = `{"blocks":null}`
	if result != expected {
		t.Errorf("With JSON without blocks, expected %q, got %q", expected, result)
	}

	// Test with blocks that have no file data
	result = ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL(`{
		"blocks": [
			{
				"type": "image",
				"data": {
					"caption": "No file data"
				}
			}
		]
	}`, mockStorage)
	// Should not crash and should return valid JSON
	if !strings.Contains(result, "No file data") {
		t.Errorf("Result should contain original caption, got %q", result)
	}
}

func BenchmarkReplaceMarkdownStaticResourcesWithPresignedURL(b *testing.B) {
	mockStorage := &MockFileStorage{}
	content := `
# Test Document

This document contains multiple images:

![Image 1](/image/space123/photo1.jpg)
![Image 2](/image/space123/photo2.png)
![Image 3](/image/space123/photo3.gif)

And some HTML images:

<img src="/image/space123/html1.jpg" alt="HTML Image 1">
<img src="/image/space123/html2.png" alt="HTML Image 2" class="responsive">

Mixed with public resources:

![Public Logo](/public/logo.png)
<img src="/public/favicon.ico" alt="Favicon">

And external images:

![External](https://example.com/external.jpg)
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ReplaceMarkdownStaticResourcesWithPresignedURL(content, mockStorage)
	}
}

func BenchmarkReplaceEditorJSBlocksStaticResourcesWithPresignedURL(b *testing.B) {
	mockStorage := &MockFileStorage{}
	blocksJSON := `{
		"blocks": [
			{
				"type": "paragraph",
				"data": {
					"text": "This is a test paragraph"
				}
			},
			{
				"type": "image",
				"data": {
					"file": {
						"url": "/image/space123/photo1.jpg"
					},
					"caption": "Test Image 1"
				}
			},
			{
				"type": "image",
				"data": {
					"file": {
						"url": "/image/space123/photo2.png"
					},
					"caption": "Test Image 2"
				}
			},
			{
				"type": "video",
				"data": {
					"file": {
						"url": "/file/space123/video.mp4"
					},
					"caption": "Test Video"
				}
			},
			{
				"type": "attaches",
				"data": {
					"file": {
						"url": "/file/space123/document.pdf",
						"name": "document.pdf"
					}
				}
			}
		]
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL(blocksJSON, mockStorage)
	}
}
