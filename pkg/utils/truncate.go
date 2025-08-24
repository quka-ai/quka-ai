package utils

import (
	"regexp"
	"strings"
)

// SmartTruncateContent 智能截断内容，保留媒体资源
func SmartTruncateContent(content string, maxLength int) string {
	runes := []rune(content)
	if len(runes) <= maxLength {
		return content
	}

	// 先进行基本截断
	truncated := string(runes[:maxLength])

	// 检查截断后的内容是否有不完整的媒体资源
	if hasIncompleteMedia(truncated) {
		// 找到最后一个完整的媒体资源位置
		truncated = adjustToCompleteMedia(truncated, content)
	}

	// 在句子边界进行最终调整
	truncated = adjustToSentenceBoundary(truncated)

	return strings.TrimSpace(truncated) + "..."
}

// hasIncompleteMedia 检查内容是否包含不完整的媒体资源
func hasIncompleteMedia(content string) bool {
	// 检查末尾是否有不完整的标签开始
	if hasIncompleteTagAtEnd(content) {
		return true
	}

	// 检查是否有不完整的图片语法 ![...](...)
	if strings.Contains(content, "![") {
		lastImageStart := strings.LastIndex(content, "![")
		if lastImageStart != -1 {
			// 检查这个图片语法是否完整
			remaining := content[lastImageStart:]
			if !strings.Contains(remaining, "](") || !strings.Contains(remaining, ")") {
				return true
			}
		}
	}

	// 检查是否有不完整的视频标签 <video>...</video>
	if strings.Contains(content, "<video") {
		lastVideoStart := strings.LastIndex(content, "<video")
		if lastVideoStart != -1 {
			// 检查这个视频标签是否完整
			remaining := content[lastVideoStart:]
			if !strings.Contains(remaining, "</video>") {
				return true
			}
		}
	}

	// 检查是否有不完整的HTML图片标签 <img ... />
	if strings.Contains(content, "<img") {
		lastImgStart := strings.LastIndex(content, "<img")
		if lastImgStart != -1 {
			// 检查这个图片标签是否完整
			remaining := content[lastImgStart:]
			if !strings.Contains(remaining, "/>") && !strings.Contains(remaining, ">") {
				return true
			}
		}
	}

	return false
}

// hasIncompleteTagAtEnd 检查末尾是否有不完整的标签开始
func hasIncompleteTagAtEnd(content string) bool {
	// 检查可能的不完整HTML标签前缀（按长度从长到短排序）
	htmlPatterns := []string{
		"<video", "<vide", "<vid", "<vi", "<v",
		"<img", "<im", "<i",
		"<",
	}

	for _, pattern := range htmlPatterns {
		if strings.HasSuffix(content, pattern) {
			return true
		}
	}

	// 检查不完整的图片语法 ![
	if strings.HasSuffix(content, "![") {
		return true
	}

	// 检查末尾是否以 < 结尾且前面有空格或其他字符（可能是标签开始）
	if strings.HasSuffix(content, "<") && len(content) > 1 {
		// 检查 < 前面的字符，如果是空格或换行，很可能是标签开始
		prevChar := content[len(content)-2]
		if prevChar == ' ' || prevChar == '\n' || prevChar == '\t' {
			return true
		}
	}

	// 检查末尾是否以 ! 结尾且前面有空格（可能是图片语法开始）
	if strings.HasSuffix(content, "!") && len(content) > 1 {
		// 检查 ! 前面的字符，如果是空格或换行，很可能是图片语法开始
		prevChar := content[len(content)-2]
		if prevChar == ' ' || prevChar == '\n' || prevChar == '\t' {
			return true
		}
	}

	return false
}

// adjustToCompleteMedia 调整截断位置到完整的媒体资源边界
func adjustToCompleteMedia(truncated, fullContent string) string {
	// 使用正则表达式找到所有完整的媒体资源
	patterns := []string{
		`!\[([^\]]*)\]\(([^)]+)\)`, // 图片语法
		`<video[^>]*>.*?</video>`,  // 视频标签
		`<img[^>]*/>`,              // 自闭合图片标签
		`<img[^>]*>[^<]*</img>`,    // 完整图片标签
	}

	var lastCompletePos int
	for _, pattern := range patterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindAllStringIndex(fullContent, -1)

		for _, match := range matches {
			// 如果整个匹配都在截断范围内，更新最后完整位置
			if match[1] <= len(truncated) {
				if match[1] > lastCompletePos {
					lastCompletePos = match[1]
				}
			} else {
				// 如果匹配跨越了截断位置，停止搜索
				break
			}
		}
	}

	// 如果找到了完整的媒体资源位置，调整截断位置
	if lastCompletePos > 0 {
		return fullContent[:lastCompletePos]
	}

	// 如果没有找到完整的媒体资源，找到最后一个不完整资源的开始位置
	mediaStarts := []int{
		strings.LastIndex(truncated, "!["),
		strings.LastIndex(truncated, "<video"),
		strings.LastIndex(truncated, "<img"),
	}

	var lastMediaStart int
	for _, start := range mediaStarts {
		if start > lastMediaStart {
			lastMediaStart = start
		}
	}

	if lastMediaStart > 0 {
		return truncated[:lastMediaStart]
	}

	return truncated
}

// adjustToSentenceBoundary 在句子边界调整截断位置
func adjustToSentenceBoundary(content string) string {
	// 寻找最后一个句号、换行符或空格，作为更好的截断点
	length := len(content)
	minPos := length / 2 // 不要截断太多

	if lastPeriod := strings.LastIndex(content, "。"); lastPeriod > minPos {
		return content[:lastPeriod+1]
	}
	if lastNewline := strings.LastIndex(content, "\n"); lastNewline > minPos {
		return content[:lastNewline]
	}
	if lastSpace := strings.LastIndex(content, " "); lastSpace > minPos {
		return content[:lastSpace]
	}

	return content
}
