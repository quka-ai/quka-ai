package utils

import (
	"strings"
	"testing"
)

func TestSmartTruncateContent(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		maxLength int
		wantCheck func(result string) bool
	}{
		{
			name:      "纯文本内容截断",
			content:   "这是一段很长的文本内容，需要进行截断处理，以确保不会超过指定的长度限制。",
			maxLength: 20,
			wantCheck: func(result string) bool {
				return len([]rune(result)) <= 23 && strings.HasSuffix(result, "...")
			},
		},
		{
			name:      "包含图片的内容截断",
			content:   "这是一段文本 ![图片描述](https://example.com/image.jpg) 后面还有更多文本内容，需要确保图片不会被截断。",
			maxLength: 30,
			wantCheck: func(result string) bool {
				// 如果包含图片，应该保持完整
				if strings.Contains(result, "![") {
					return strings.Contains(result, "](") && strings.Contains(result, ")")
				}
				return true
			},
		},
		{
			name:      "包含视频的内容截断",
			content:   "这是一段文本 <video controls><source src=\"video.mp4\"></video> 后面还有更多文本内容。",
			maxLength: 30,
			wantCheck: func(result string) bool {
				// 如果包含视频，应该保持完整
				if strings.Contains(result, "<video") {
					return strings.Contains(result, "</video>")
				}
				return true
			},
		},
		{
			name:      "包含HTML图片的内容截断",
			content:   "这是一段文本 <img src=\"image.jpg\" alt=\"描述\"/> 后面还有更多文本内容。",
			maxLength: 30,
			wantCheck: func(result string) bool {
				// 如果包含HTML图片，应该保持完整
				if strings.Contains(result, "<img") {
					return strings.Contains(result, "/>")
				}
				return true
			},
		},
		{
			name:      "媒体资源超出长度限制",
			content:   "简短文本 ![很长的图片描述内容](https://example.com/very-long-image-url.jpg) 后面的文本",
			maxLength: 10,
			wantCheck: func(result string) bool {
				// 如果媒体资源太长，应该在媒体资源前截断
				return !strings.Contains(result, "![") ||
					(strings.Contains(result, "![") && strings.Contains(result, "](") && strings.Contains(result, ")"))
			},
		},
		{
			name:      "多个媒体资源",
			content:   "文本1 ![图片1](url1.jpg) 文本2 ![图片2](url2.jpg) 文本3",
			maxLength: 50,
			wantCheck: func(result string) bool {
				// 检查所有包含的图片都是完整的
				imageStarts := 0
				imageEnds := 0
				for i := 0; i < len(result); i++ {
					if i < len(result)-1 && result[i:i+2] == "![" {
						imageStarts++
					}
					if result[i] == ')' && i > 0 && result[i-1] != '\\' {
						imageEnds++
					}
				}
				return imageStarts == imageEnds
			},
		},
		{
			name:      "内容长度小于限制",
			content:   "短文本 ![图片](url.jpg)",
			maxLength: 100,
			wantCheck: func(result string) bool {
				// 不应该被截断
				return !strings.HasSuffix(result, "...")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SmartTruncateContent(tt.content, tt.maxLength)

			// 基本长度检查
			if !strings.HasSuffix(result, "...") {
				// 如果没有省略号，说明内容没有被截断
				if len([]rune(result)) > tt.maxLength {
					t.Errorf("未截断的内容长度超过限制: got %d, want <= %d", len([]rune(result)), tt.maxLength)
				}
			} else {
				// 如果有省略号，检查截断后的长度是否合理
				contentWithoutDots := strings.TrimSuffix(result, "...")
				if len([]rune(contentWithoutDots)) > tt.maxLength {
					t.Errorf("截断后的内容长度超过限制: got %d, want <= %d", len([]rune(contentWithoutDots)), tt.maxLength)
				}
			}

			// 自定义检查
			if !tt.wantCheck(result) {
				t.Errorf("自定义检查失败")
			}

			t.Logf("输入: %s", tt.content)
			t.Logf("输出: %s", result)
			t.Logf("输出长度: %d 字符", len([]rune(result)))
		})
	}
}

// TestSpecificBoundaryCase 测试特定的边界情况
func TestSpecificBoundaryCase(t *testing.T) {
	// 直接创建包含不完整标签的测试场景
	testCases := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "截断到<",
			content:  "这是一段文本 <",
			expected: true,
		},
		{
			name:     "截断到<i",
			content:  "这是一段文本 <i",
			expected: true,
		},
		{
			name:     "截断到<im",
			content:  "这是一段文本 <im",
			expected: true,
		},
		{
			name:     "截断到<img",
			content:  "这是一段文本 <img",
			expected: true,
		},
		{
			name:     "截断到<v",
			content:  "这是一段文本 <v",
			expected: true,
		},
		{
			name:     "截断到<video",
			content:  "这是一段文本 <video",
			expected: true,
		},
		{
			name:     "截断到!",
			content:  "这是一段文本 !",
			expected: true,
		},
		{
			name:     "截断到![",
			content:  "这是一段文本 ![",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("测试内容: '%s'", tc.content)

			// 验证hasIncompleteMedia能正确检测到
			hasIncomplete := hasIncompleteMedia(tc.content)
			t.Logf("hasIncompleteMedia结果: %v", hasIncomplete)

			if hasIncomplete != tc.expected {
				t.Errorf("hasIncompleteMedia() = %v, want %v", hasIncomplete, tc.expected)
			}

			// 验证SmartTruncateContent能正确处理
			fullContent := tc.content + "g src=\"test.jpg\"/> 还有更多内容"
			result := SmartTruncateContent(fullContent, len([]rune(tc.content)))
			t.Logf("智能截断结果: %s", result)

			// 结果不应该包含不完整的标签
			cleanResult := strings.TrimSuffix(result, "...")
			if strings.HasSuffix(cleanResult, "<") ||
				strings.HasSuffix(cleanResult, "<i") ||
				strings.HasSuffix(cleanResult, "<im") ||
				strings.HasSuffix(cleanResult, "<img") ||
				strings.HasSuffix(cleanResult, "<v") ||
				strings.HasSuffix(cleanResult, "<video") ||
				strings.HasSuffix(cleanResult, "!") ||
				strings.HasSuffix(cleanResult, "![") {
				// 但是要排除正常的句子结尾
				if !strings.HasSuffix(cleanResult, "句子！") && !strings.HasSuffix(cleanResult, "内容！") {
					t.Errorf("结果包含不完整的标签: %s", result)
				}
			}
		})
	}
}

func TestHasIncompleteMedia(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "完整图片",
			content:  "文本 ![图片](url.jpg) 文本",
			expected: false,
		},
		{
			name:     "不完整图片-缺少右括号",
			content:  "文本 ![图片](url.jpg 文本",
			expected: true,
		},
		{
			name:     "不完整图片-缺少URL部分",
			content:  "文本 ![图片]( 文本",
			expected: true,
		},
		{
			name:     "完整视频",
			content:  "文本 <video controls></video> 文本",
			expected: false,
		},
		{
			name:     "不完整视频",
			content:  "文本 <video controls> 文本",
			expected: true,
		},
		{
			name:     "完整HTML图片",
			content:  "文本 <img src=\"test.jpg\"/> 文本",
			expected: false,
		},
		{
			name:     "不完整HTML图片",
			content:  "文本 <img src=\"test.jpg\" 文本",
			expected: true,
		},
		// 新增的边界情况测试
		{
			name:     "末尾不完整img标签-<im",
			content:  "文本内容 <im",
			expected: true,
		},
		{
			name:     "末尾不完整img标签-<i",
			content:  "文本内容 <i",
			expected: true,
		},
		{
			name:     "末尾不完整video标签-<v",
			content:  "文本内容 <v",
			expected: true,
		},
		{
			name:     "末尾不完整video标签-<vid",
			content:  "文本内容 <vid",
			expected: true,
		},
		{
			name:     "末尾不完整图片语法-!",
			content:  "文本内容 !",
			expected: true,
		},
		{
			name:     "末尾不完整标签-<",
			content:  "文本内容 <",
			expected: true,
		},
		{
			name:     "正常以感叹号结尾",
			content:  "这是一个正常的句子！",
			expected: false,
		},
		{
			name:     "正常文本包含小于号",
			content:  "1 < 2 是正确的",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasIncompleteMedia(tt.content)
			if result != tt.expected {
				t.Errorf("hasIncompleteMedia() = %v, want %v", result, tt.expected)
			}
		})
	}
}
