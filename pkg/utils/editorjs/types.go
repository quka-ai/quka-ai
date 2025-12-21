package editorjs

import (
	"encoding/json"

	"github.com/davidscottmills/goeditorjs"
)

// EditorAttaches 表示EditorJS的附件块数据结构
type EditorAttaches struct {
	File    EditorAttachesFile `json:"file"`
	Title   string             `json:"title,omitempty"`
	Caption string             `json:"caption,omitempty"`
}

// EditorAttachesFile 表示附件的文件信息
type EditorAttachesFile struct {
	URL   string `json:"url"`
	Name  string `json:"name,omitempty"`
	Size  int64  `json:"size,omitempty"`
	Error string `json:"error,omitempty"`
}

// EditorImageFileWithError 扩展的图片文件结构，支持错误信息
type EditorImageFileWithError struct {
	URL   string `json:"url"`
	Error string `json:"error,omitempty"`
}

// EditorImageWithError 扩展的图片结构，支持错误信息
type EditorImageWithError struct {
	File           EditorImageFileWithError `json:"file"`
	Caption        string                   `json:"caption,omitempty"`
	WithBorder     bool                     `json:"withBorder,omitempty"`
	WithBackground bool                     `json:"withBackground,omitempty"`
	Stretched      bool                     `json:"stretched,omitempty"`
}

// EditorVideoFileWithError 扩展的视频文件结构，支持错误信息
type EditorVideoFileWithError struct {
	Type  string `json:"type,omitempty"`
	URL   string `json:"url"`
	Error string `json:"error,omitempty"`
}

// EditorVideoWithError 扩展的视频结构，支持错误信息
type EditorVideoWithError struct {
	File           EditorVideoFileWithError `json:"file"`
	Caption        string                   `json:"caption,omitempty"`
	WithBorder     bool                     `json:"withBorder,omitempty"`
	WithBackground bool                     `json:"withBackground,omitempty"`
	Stretched      bool                     `json:"stretched,omitempty"`
}

// EditorVideo 表示EditorJS的视频块数据结构
type EditorVideo struct {
	File           EditorVideoFile `json:"file"`
	Caption        string          `json:"caption"`
	WithBorder     bool            `json:"withBorder"`
	WithBackground bool            `json:"withBackground"`
	Stretched      bool            `json:"stretched"`
}

type EditorVideoFile struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// EditorImage 表示EditorJS的图片块数据结构
type EditorImage struct {
	File           EditorImageFile `json:"file"`
	Caption        string          `json:"caption"`
	WithBorder     bool            `json:"withBorder"`
	WithBackground bool            `json:"withBackground"`
	Stretched      bool            `json:"stretched"`
	Width          float64         `json:"width"`
}

type EditorImageFile struct {
	URL string `json:"url"`
}

// EditorParagraph 表示EditorJS的段落块数据结构
type EditorParagraph struct {
	Text      string `json:"text"`
	Alignment string `json:"alignment"`
}

// listv2 represents list data from EditorJS
type listv2 struct {
	Style string       `json:"style"`
	Items []listv2Item `json:"items"`
}

type listv2Item struct {
	Content string          `json:"content"`
	Items   json.RawMessage `json:"items"`
	Meta    listV2ItemMeta  `json:"meta"`
}

type listV2ItemMeta struct {
	Checked bool `json:"checked,omitempty"`
}

// line represents delimiter line data from EditorJS
type line struct {
	Style         string `json:"style"`
	LineThickness int    `json:"lineThickness"`
	LineWidth     int    `json:"lineWidth"`
}

// quote represents quote data from EditorJS
type quote struct {
	Alignment string `json:"alignment"`
	Caption   string `json:"caption"`
	Text      string `json:"text"`
}

// ParseRawToBlocks 解析JSON原始数据为块内容
func ParseRawToBlocks(blockString json.RawMessage) (*BlockContent, error) {
	var blocks BlockContent
	if err := json.Unmarshal(blockString, &blocks); err != nil {
		return nil, err
	}
	return &blocks, nil
}

// BlockContent 表示EditorJS的内容块
type BlockContent struct {
	Blocks  []goeditorjs.EditorJSBlock `json:"blocks"`
	Time    int64                      `json:"time"` // javascript time
	Version string                     `json:"version"`
}
