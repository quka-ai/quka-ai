package editorjs

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/davidscottmills/goeditorjs"
	"github.com/quka-ai/quka-ai/pkg/types"
)

const (
	// PRESIGN_FAILURE_PLACEHOLDER_IMAGE 预签名失败时使用的占位图片
	// 这是一个base64编码的SVG错误图标，显示一个带有感叹号的警告图标
	// 颜色为橙色(#FF6B41)，尺寸为24x24像素
	// 当S3预签名URL生成失败时，使用此占位图片确保用户界面不会显示空白
	PRESIGN_FAILURE_PLACEHOLDER_IMAGE = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTEyIDlWMTNNMTIgMTcuMDEwOUwxMi4wMSAxN00yMSAxMkMxNyAxMiAxNyA4IDEyIDhDNyA4IDcgMTIgMyAxMiIgc3Ryb2tlPSIjRkY2QjQxIiBzdHJva2Utd2lkdGg9IjIiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIgc3Ryb2tlLWxpbmVqb2luPSJyb3VuZCIvPgo8L3N2Zz4K"
)

// FileStorageInterface 文件存储接口定义
type FileStorageInterface interface {
	GenGetObjectPreSignURL(objectPath string) (string, error)
	GetStaticDomain() string
}

// ReplaceMarkdownStaticResourcesWithPresignedURL 替换markdown中的静态资源URL为预签名URL
func ReplaceMarkdownStaticResourcesWithPresignedURL(content string, fileStorage FileStorageInterface) string {
	if len(content) == 0 || fileStorage == nil {
		return content
	}

	// 匹配markdown中的图片语法: ![alt](url)
	imageRegex := regexp.MustCompile(`!\[(.*?)\]\(([^)]+)\)`)

	// 替换图片URL
	strContent := string(content)
	convertedContent := imageRegex.ReplaceAllStringFunc(strContent, func(match string) string {
		// 提取URL部分
		submatches := imageRegex.FindStringSubmatch(match)
		if len(submatches) != 3 {
			return match
		}

		altText := submatches[1]
		originalURL := submatches[2]

		// 检查是否是需要预签名的内部资源
		if ShouldPresignURL(originalURL) {
			// 提取object path
			objectPath := ExtractObjectPath(originalURL)
			if objectPath != "" {
				// 生成预签名URL
				if presignedURL, err := fileStorage.GenGetObjectPreSignURL(objectPath); err == nil {
					return fmt.Sprintf("![%s](%s)", altText, presignedURL)
				} else {
					// 记录预签名失败的日志
					slog.Warn("Failed to generate presigned URL for markdown image",
						slog.String("object_path", objectPath),
						slog.String("error", err.Error()))
					// 预签名失败，返回一个降级的占位符或错误提示
					return fmt.Sprintf("![%s](# \"Resource temporarily unavailable: %s\")", altText, err.Error())
				}
			}
		}
		return match
	})

	// 匹配HTML中的img标签: <img src="url" ... />
	htmlImgRegex := regexp.MustCompile(`<img[^>]+src\s*=\s*["']([^"']+)["'][^>]*>`)

	// 替换HTML img标签中的src
	convertedContent = htmlImgRegex.ReplaceAllStringFunc(convertedContent, func(match string) string {
		// 提取src属性值
		srcRegex := regexp.MustCompile(`src\s*=\s*["']([^"']+)["']`)
		srcMatches := srcRegex.FindStringSubmatch(match)
		if len(srcMatches) != 2 {
			return match
		}

		originalURL := srcMatches[1]

		// 检查是否需要预签名
		if ShouldPresignURL(originalURL) {
			// 提取object path
			objectPath := ExtractObjectPath(originalURL)
			if objectPath != "" {
				// 生成预签名URL
				if presignedURL, err := fileStorage.GenGetObjectPreSignURL(objectPath); err == nil {
					return srcRegex.ReplaceAllString(match, fmt.Sprintf(`src="%s"`, presignedURL))
				} else {
					// 记录预签名失败的日志
					slog.Warn("Failed to generate presigned URL for HTML image",
						slog.String("object_path", objectPath),
						slog.String("error", err.Error()))
					// 预签名失败，返回一个错误占位图片
					return srcRegex.ReplaceAllString(match, fmt.Sprintf(`src="%s" alt="Resource unavailable"`, PRESIGN_FAILURE_PLACEHOLDER_IMAGE))
				}
			}
		}

		return match
	})

	return convertedContent
}

// ReplaceEditorJSBlocksStaticResourcesWithPresignedURL 替换EditorJS blocks中的静态资源URL为预签名URL
func ReplaceEditorJSBlocksStaticResourcesWithPresignedURL(blocks []goeditorjs.EditorJSBlock, fileStorage FileStorageInterface) []goeditorjs.EditorJSBlock {
	// 处理每个block
	for i, block := range blocks {
		switch block.Type {
		case "image":
			blocks[i] = processImageBlockWithStruct(block, fileStorage)
		case "video":
			blocks[i] = processVideoBlockWithStruct(block, fileStorage)
		case "attaches":
			blocks[i] = processAttachesBlockWithStruct(block, fileStorage)
		}
	}

	return blocks
}

// ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL 替换EditorJS blocks中的静态资源URL为预签名URL
func ReplaceEditorJSBlocksJsonStaticResourcesWithPresignedURL(blocksJSON types.KnowledgeContent, fileStorage FileStorageInterface) types.KnowledgeContent {
	if len(blocksJSON) == 0 || fileStorage == nil {
		return blocksJSON
	}

	// 解析为map以保持原有结构
	var data BlockContent
	if err := json.Unmarshal([]byte(blocksJSON), &data); err != nil {
		return blocksJSON
	}

	// 处理blocks
	data.Blocks = ReplaceEditorJSBlocksStaticResourcesWithPresignedURL(data.Blocks, fileStorage)

	// 重新序列化JSON
	if newJSON, err := json.Marshal(data); err == nil {
		return newJSON
	}

	return blocksJSON
}

// ReplaceBlockNoteBlocksStaticResourcesWithPresignedURL 替换 BlockNote blocks 中的静态资源 URL 为预签名 URL。
func ReplaceBlockNoteBlocksStaticResourcesWithPresignedURL(blocks []BlockNoteBlock, fileStorage FileStorageInterface) []BlockNoteBlock {
	for i := range blocks {
		switch blocks[i].Type {
		case "image", "video", "audio", "file":
			blocks[i] = processBlockNoteFileBlock(blocks[i], fileStorage)
		}

		if len(blocks[i].Children) > 0 {
			blocks[i].Children = ReplaceBlockNoteBlocksStaticResourcesWithPresignedURL(blocks[i].Children, fileStorage)
		}
	}

	return blocks
}

// ReplaceBlockNoteBlocksJsonStaticResourcesWithPresignedURL 替换 BlockNote blocks JSON 中的静态资源 URL 为预签名 URL。
func ReplaceBlockNoteBlocksJsonStaticResourcesWithPresignedURL(blocksJSON types.KnowledgeContent, fileStorage FileStorageInterface) types.KnowledgeContent {
	if len(blocksJSON) == 0 || fileStorage == nil {
		return blocksJSON
	}

	var blocks []BlockNoteBlock
	if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
		return blocksJSON
	}

	blocks = ReplaceBlockNoteBlocksStaticResourcesWithPresignedURL(blocks, fileStorage)
	if newJSON, err := json.Marshal(blocks); err == nil {
		return newJSON
	}

	return blocksJSON
}

func processBlockNoteFileBlock(block BlockNoteBlock, fileStorage FileStorageInterface) BlockNoteBlock {
	originalURL := stringProp(block.Props, "url")
	if fileStorage != nil {
		originalURL = strings.Replace(originalURL, fileStorage.GetStaticDomain(), "", 1)
	}

	if ShouldPresignURL(originalURL) {
		objectPath := ExtractObjectPath(originalURL)
		if objectPath != "" {
			if presignedURL, err := fileStorage.GenGetObjectPreSignURL(objectPath); err == nil {
				block.Props["url"] = presignedURL
			} else {
				slog.Warn("Failed to generate presigned URL for BlockNote file block",
					slog.String("object_path", objectPath),
					slog.String("error", err.Error()))
				if block.Type == "image" {
					block.Props["url"] = PRESIGN_FAILURE_PLACEHOLDER_IMAGE
				}
			}
		}
	}

	return block
}

// processImageBlockWithStruct 使用结构体处理图片块中的URL
func processImageBlockWithStruct(block goeditorjs.EditorJSBlock, fileStorage FileStorageInterface) goeditorjs.EditorJSBlock {
	image := &EditorImage{}
	if err := json.Unmarshal(block.Data, image); err != nil {
		return block
	}

	originalURL := image.File.URL
	if ShouldPresignURL(originalURL) {
		objectPath := ExtractObjectPath(originalURL)
		if objectPath != "" {
			if presignedURL, err := fileStorage.GenGetObjectPreSignURL(objectPath); err == nil {
				image.File.URL = presignedURL
			} else {
				// 记录预签名失败的日志
				slog.Warn("Failed to generate presigned URL for EditorJS image block",
					slog.String("object_path", objectPath),
					slog.String("error", err.Error()))
				// 预签名失败，设置错误占位符
				image.File.URL = PRESIGN_FAILURE_PLACEHOLDER_IMAGE
			}
		}
	}

	// 重新序列化block数据
	if newData, err := json.Marshal(image); err == nil {
		block.Data = newData
	}

	return block
}

// processVideoBlockWithStruct 使用结构体处理视频块中的URL
func processVideoBlockWithStruct(block goeditorjs.EditorJSBlock, fileStorage FileStorageInterface) goeditorjs.EditorJSBlock {
	video := &EditorVideo{}
	if err := json.Unmarshal(block.Data, video); err != nil {
		slog.Error("Failed to process video block", slog.String("error", err.Error()))
		return block
	}

	originalURL := strings.Replace(video.File.URL, fileStorage.GetStaticDomain(), "", 1)

	if ShouldPresignURL(originalURL) {
		objectPath := ExtractObjectPath(originalURL)
		if objectPath != "" {
			if presignedURL, err := fileStorage.GenGetObjectPreSignURL(objectPath); err == nil {
				video.File.URL = presignedURL
			} else {
				// 记录预签名失败的日志
				slog.Warn("Failed to generate presigned URL for EditorJS video block",
					slog.String("object_path", objectPath),
					slog.String("error", err.Error()))
				// 预签名失败，保持原URL（视频可能有其他处理方式）
			}
		}
	}

	// 重新序列化block数据
	if newData, err := json.Marshal(video); err == nil {
		block.Data = newData
	}

	return block
}

// processAttachesBlockWithStruct 使用结构体处理附件块中的URL
func processAttachesBlockWithStruct(block goeditorjs.EditorJSBlock, fileStorage FileStorageInterface) goeditorjs.EditorJSBlock {
	attaches := &EditorAttaches{}
	if err := json.Unmarshal(block.Data, attaches); err != nil {
		return block
	}

	originalURL := attaches.File.URL
	if ShouldPresignURL(originalURL) {
		objectPath := ExtractObjectPath(originalURL)
		if objectPath != "" {
			if presignedURL, err := fileStorage.GenGetObjectPreSignURL(objectPath); err == nil {
				attaches.File.URL = presignedURL
			} else {
				// 记录预签名失败的日志
				slog.Warn("Failed to generate presigned URL for EditorJS attaches block",
					slog.String("object_path", objectPath),
					slog.String("error", err.Error()))
				// 预签名失败，保持原URL
			}
		}
	}

	// 重新序列化block数据
	if newData, err := json.Marshal(attaches); err == nil {
		block.Data = newData
	}

	return block
}

// ShouldPresignURL 判断URL是否需要预签名处理
func ShouldPresignURL(url string) bool {
	// 跳过已经是预签名的URL
	if (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) && (strings.Contains(url, "X-Amz-Algorithm") || strings.Contains(url, "Signature")) {
		return false
	}

	// 跳过外部URL
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return false
	}

	// 跳过base64数据
	if strings.HasPrefix(url, "data:") {
		return false
	}

	// 跳过public资源
	if strings.HasPrefix(url, "/public/") {
		return false
	}

	// 其他所有内部资源都需要预签名
	return true
}

// ExtractObjectPath 从URL中提取object path
func ExtractObjectPath(url string) string {
	// 移除查询参数
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}

	// 移除fragment
	if idx := strings.Index(url, "#"); idx != -1 {
		url = url[:idx]
	}

	// 移除路径前缀（如 /image/, /file/, /object/ 等）
	if strings.HasPrefix(url, "/image/") {
		return url[7:] // 移除 "/image/"
	}
	if strings.HasPrefix(url, "/file/") {
		return url[6:] // 移除 "/file/"
	}
	if strings.HasPrefix(url, "/object/") {
		return url[8:] // 移除 "/object/"
	}

	// 对于public资源和外部URL，返回空字符串
	if strings.HasPrefix(url, "/public/") || strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return ""
	}

	return url
}

// isLikelyBucketName 检查给定的字符串是否可能是bucket名称
func isLikelyBucketName(name string) bool {
	// Bucket名称通常是小写字母、数字和连字符的组合
	// 长度通常在3-63个字符之间
	if len(name) < 3 || len(name) > 63 {
		return false
	}

	// 检查是否只包含小写字母、数字和连字符
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
			return false
		}
	}

	// 不能以连字符开头或结尾
	if name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}

	return true
}
