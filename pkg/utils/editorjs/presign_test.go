package editorjs

import (
	"testing"
)

func TestExtractObjectPath(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "MinIO路径风格URL",
			input:    "/mybucket/uploads/2023/image.jpg",
			expected: "uploads/2023/image.jpg",
		},
		{
			name:     "MinIO路径风格URL - 深层路径",
			input:    "/testbucket/users/123/files/document.pdf",
			expected: "users/123/files/document.pdf",
		},
		{
			name:     "非bucket路径",
			input:    "/uploads/image.jpg",
			expected: "uploads/image.jpg",
		},
		{
			name:     "Image前缀路径",
			input:    "/image/uploads/test.jpg",
			expected: "uploads/test.jpg",
		},
		{
			name:     "File前缀路径",
			input:    "/file/uploads/test.pdf",
			expected: "uploads/test.pdf",
		},
		{
			name:     "Object前缀路径",
			input:    "/object/uploads/test.mp4",
			expected: "uploads/test.mp4",
		},
		{
			name:     "Public资源路径",
			input:    "/public/static/logo.png",
			expected: "",
		},
		{
			name:     "外部HTTP URL",
			input:    "http://example.com/image.jpg",
			expected: "",
		},
		{
			name:     "外部HTTPS URL",
			input:    "https://example.com/image.jpg",
			expected: "",
		},
		{
			name:     "带查询参数的URL",
			input:    "/mybucket/uploads/image.jpg?version=1",
			expected: "uploads/image.jpg",
		},
		{
			name:     "带fragment的URL",
			input:    "/mybucket/uploads/image.jpg#section1",
			expected: "uploads/image.jpg",
		},
		{
			name:     "单级路径",
			input:    "/image.jpg",
			expected: "image.jpg",
		},
		{
			name:     "空路径",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractObjectPath(tc.input)
			if result != tc.expected {
				t.Errorf("ExtractObjectPath(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsLikelyBucketName(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "有效的bucket名称",
			input:    "mybucket",
			expected: true,
		},
		{
			name:     "带连字符的bucket名称",
			input:    "my-bucket",
			expected: true,
		},
		{
			name:     "带数字的bucket名称",
			input:    "bucket123",
			expected: true,
		},
		{
			name:     "包含大写字母",
			input:    "MyBucket",
			expected: false,
		},
		{
			name:     "包含下划线",
			input:    "my_bucket",
			expected: false,
		},
		{
			name:     "以连字符开头",
			input:    "-mybucket",
			expected: false,
		},
		{
			name:     "以连字符结尾",
			input:    "mybucket-",
			expected: false,
		},
		{
			name:     "太短",
			input:    "ab",
			expected: false,
		},
		{
			name:     "太长",
			input:    "this-is-a-very-long-bucket-name-that-exceeds-sixty-three-characters",
			expected: false,
		},
		{
			name:     "普通目录名",
			input:    "uploads",
			expected: true,
		},
		{
			name:     "包含特殊字符",
			input:    "bucket@name",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isLikelyBucketName(tc.input)
			if result != tc.expected {
				t.Errorf("isLikelyBucketName(%q) = %t, expected %t", tc.input, result, tc.expected)
			}
		})
	}
}