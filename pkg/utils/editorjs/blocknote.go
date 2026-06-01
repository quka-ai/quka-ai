package editorjs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"

	"github.com/quka-ai/quka-ai/pkg/types"
)

type BlockNoteBlock struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Props    BlockNoteProps   `json:"props"`
	Content  json.RawMessage  `json:"content,omitempty"`
	Children []BlockNoteBlock `json:"children,omitempty"`
}

type BlockNoteProps map[string]any

type blockNoteInline struct {
	Type    string            `json:"type"`
	Text    string            `json:"text,omitempty"`
	Href    string            `json:"href,omitempty"`
	Styles  map[string]any    `json:"styles,omitempty"`
	Content []blockNoteInline `json:"content,omitempty"`
}

type blockNoteTableContent struct {
	Type         string              `json:"type"`
	ColumnWidths []json.RawMessage   `json:"columnWidths,omitempty"`
	HeaderRows   int                 `json:"headerRows,omitempty"`
	HeaderCols   int                 `json:"headerCols,omitempty"`
	Rows         []blockNoteTableRow `json:"rows"`
}

type blockNoteTableRow struct {
	Cells []json.RawMessage `json:"cells"`
}

type blockNoteTableCell struct {
	Type    string          `json:"type"`
	Props   BlockNoteProps  `json:"props,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
}

// ConvertKnowledgeRawToMarkdown 根据 content_type 将知识内容转换为 Markdown。
func ConvertKnowledgeRawToMarkdown(contentType types.KnowledgeContentType, content types.KnowledgeContent) (string, error) {
	switch contentType {
	case types.KNOWLEDGE_CONTENT_TYPE_BLOCKS:
		return ConvertEditorJSRawToMarkdown(content)
	case types.KNOWLEDGE_CONTENT_TYPE_BLOCKS_V2:
		return ConvertBlockNoteRawToMarkdown(content)
	default:
		return content.String(), nil
	}
}

// ConvertRawToMarkdownAuto 根据原始 JSON 顶层结构自动选择 EditorJS 或 BlockNote 转换。
func ConvertRawToMarkdownAuto(content types.KnowledgeContent) (string, error) {
	raw := bytes.TrimSpace(content)
	if len(raw) == 0 {
		return "", nil
	}

	switch raw[0] {
	case '{':
		return ConvertEditorJSRawToMarkdown(content)
	case '[':
		return ConvertBlockNoteRawToMarkdown(content)
	default:
		return content.String(), nil
	}
}

// ConvertBlockNoteRawToMarkdown 将 BlockNote 原生 blocks JSON 转换为 Markdown。
func ConvertBlockNoteRawToMarkdown(blockString types.KnowledgeContent) (string, error) {
	var blocks []BlockNoteBlock
	if err := json.Unmarshal(blockString, &blocks); err != nil {
		return "", err
	}

	return ConvertBlockNoteBlocksToMarkdown(blocks), nil
}

// ConvertBlockNoteBlocksToMarkdown 将 BlockNote blocks 转换为 Markdown。
func ConvertBlockNoteBlocksToMarkdown(blocks []BlockNoteBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		markdown := renderBlockNoteBlock(block, 0)
		if markdown != "" {
			parts = append(parts, markdown)
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

// RemoveBlockNoteFileBlockHost 移除 BlockNote 文件块中的主机名。
func RemoveBlockNoteFileBlockHost(blocks []BlockNoteBlock, bucketName string) []BlockNoteBlock {
	for i := range blocks {
		switch blocks[i].Type {
		case "image", "video", "audio", "file":
			rawURL := stringProp(blocks[i].Props, "url")
			if rawURL != "" {
				blocks[i].Props["url"] = removeAttachURLHost(rawURL, bucketName)
			}
		}

		if len(blocks[i].Children) > 0 {
			blocks[i].Children = RemoveBlockNoteFileBlockHost(blocks[i].Children, bucketName)
		}
	}

	return blocks
}

// ExtractBlockNoteFileURLs 提取 BlockNote 文件/媒体块中的 URL。
func ExtractBlockNoteFileURLs(blocks []BlockNoteBlock) []string {
	files := make([]string, 0)
	for _, block := range blocks {
		switch block.Type {
		case "image", "video", "audio", "file":
			if rawURL := stringProp(block.Props, "url"); rawURL != "" {
				files = append(files, rawURL)
			}
		}

		files = append(files, ExtractBlockNoteFileURLs(block.Children)...)
	}

	return files
}

func renderBlockNoteBlock(block BlockNoteBlock, depth int) string {
	var markdown string

	switch block.Type {
	case "paragraph":
		markdown = renderBlockNoteInlineContent(block.Content)
	case "heading":
		level := intProp(block.Props, "level", 1)
		if level < 1 || level > 6 {
			level = 1
		}
		text := renderBlockNoteInlineContent(block.Content)
		if text != "" {
			markdown = fmt.Sprintf("%s %s", strings.Repeat("#", level), text)
		}
	case "bulletListItem":
		markdown = renderBlockNoteListItem("-", block, depth)
	case "numberedListItem":
		start := intProp(block.Props, "start", 1)
		if start <= 0 {
			start = 1
		}
		markdown = renderBlockNoteListItem(fmt.Sprintf("%d.", start), block, depth)
	case "checkListItem":
		marker := "- [ ]"
		if boolProp(block.Props, "checked") {
			marker = "- [x]"
		}
		markdown = renderBlockNoteListItem(marker, block, depth)
	case "toggleListItem":
		markdown = renderBlockNoteListItem("-", block, depth)
	case "quote":
		markdown = renderBlockNoteQuote(renderBlockNoteInlineContent(block.Content))
	case "codeBlock":
		language := stringProp(block.Props, "language")
		code := renderBlockNotePlainText(block.Content)
		markdown = fmt.Sprintf("```%s\n%s\n```", language, code)
	case "divider":
		markdown = "---"
	case "image":
		markdown = renderBlockNoteImage(block.Props)
	case "video":
		markdown = renderBlockNoteVideo(block.Props)
	case "audio":
		markdown = renderBlockNoteAudio(block.Props)
	case "file":
		markdown = renderBlockNoteFile(block.Props)
	case "table":
		markdown = renderBlockNoteTable(block.Content)
	default:
		markdown = renderBlockNoteInlineContent(block.Content)
	}

	children := renderBlockNoteChildren(block.Children, depth)
	if markdown == "" {
		return children
	}
	if children == "" {
		if isBlockNoteListType(block.Type) {
			return strings.TrimRight(markdown, "\n")
		}
		return strings.TrimSpace(markdown)
	}

	if isBlockNoteListType(block.Type) {
		return strings.TrimRight(markdown, "\n") + "\n" + children
	}

	return strings.TrimSpace(markdown) + "\n\n" + children
}

func renderBlockNoteChildren(children []BlockNoteBlock, depth int) string {
	if len(children) == 0 {
		return ""
	}

	parts := make([]string, 0, len(children))
	for _, child := range children {
		childMarkdown := renderBlockNoteBlock(child, depth+1)
		if childMarkdown != "" {
			parts = append(parts, childMarkdown)
		}
	}

	return strings.Join(parts, "\n")
}

func renderBlockNoteListItem(marker string, block BlockNoteBlock, depth int) string {
	content := renderBlockNoteInlineContent(block.Content)
	prefix := strings.Repeat("  ", depth) + marker
	if content == "" {
		return prefix
	}

	return prefix + " " + content
}

func renderBlockNoteQuote(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = "> " + line
	}

	return strings.Join(lines, "\n")
}

func renderBlockNoteImage(props BlockNoteProps) string {
	rawURL := stringProp(props, "url")
	if rawURL == "" {
		return ""
	}

	caption := stringProp(props, "caption")
	if caption == "" {
		caption = stringProp(props, "name")
	}

	return fmt.Sprintf("![%s](%s)", escapeMarkdownAlt(caption), rawURL)
}

func renderBlockNoteVideo(props BlockNoteProps) string {
	rawURL := stringProp(props, "url")
	if rawURL == "" {
		return ""
	}

	caption := stringProp(props, "caption")
	html := fmt.Sprintf("<video controls preload=\"metadata\"><source src=\"%s\"></video>", html.EscapeString(rawURL))
	if caption != "" {
		return html + "\n" + caption
	}

	return html
}

func renderBlockNoteAudio(props BlockNoteProps) string {
	rawURL := stringProp(props, "url")
	if rawURL == "" {
		return ""
	}

	caption := stringProp(props, "caption")
	html := fmt.Sprintf("<audio controls preload=\"metadata\"><source src=\"%s\"></audio>", html.EscapeString(rawURL))
	if caption != "" {
		return html + "\n" + caption
	}

	return html
}

func renderBlockNoteFile(props BlockNoteProps) string {
	rawURL := stringProp(props, "url")
	if rawURL == "" {
		return ""
	}

	name := stringProp(props, "name")
	if name == "" {
		name = fileNameFromURL(rawURL)
	}
	if name == "" {
		name = rawURL
	}

	caption := stringProp(props, "caption")
	markdown := fmt.Sprintf("[%s](%s)", escapeMarkdownLinkText(name), rawURL)
	if caption != "" {
		return markdown + "\n" + caption
	}

	return markdown
}

func renderBlockNoteTable(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var table blockNoteTableContent
	if err := json.Unmarshal(raw, &table); err != nil {
		return ""
	}
	if len(table.Rows) == 0 {
		return ""
	}

	rows := make([][]string, 0, len(table.Rows))
	maxCols := 0
	for _, row := range table.Rows {
		cells := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			cells = append(cells, renderBlockNoteTableCell(cell))
		}
		if len(cells) > maxCols {
			maxCols = len(cells)
		}
		rows = append(rows, cells)
	}
	if maxCols == 0 {
		return ""
	}

	for i := range rows {
		for len(rows[i]) < maxCols {
			rows[i] = append(rows[i], "")
		}
	}

	var builder strings.Builder
	builder.WriteString(renderMarkdownTableRow(rows[0]))
	builder.WriteString("\n")
	builder.WriteString(renderMarkdownTableSeparator(maxCols))
	for _, row := range rows[1:] {
		builder.WriteString("\n")
		builder.WriteString(renderMarkdownTableRow(row))
	}

	return builder.String()
}

func renderBlockNoteTableCell(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var inline []blockNoteInline
	if err := json.Unmarshal(raw, &inline); err == nil {
		return sanitizeMarkdownTableCell(renderBlockNoteInlines(inline))
	}

	var cell blockNoteTableCell
	if err := json.Unmarshal(raw, &cell); err != nil {
		return ""
	}

	return sanitizeMarkdownTableCell(renderBlockNoteInlineContent(cell.Content))
}

func renderMarkdownTableRow(cells []string) string {
	return "| " + strings.Join(cells, " | ") + " |"
}

func renderMarkdownTableSeparator(cols int) string {
	parts := make([]string, cols)
	for i := range parts {
		parts[i] = "---"
	}

	return renderMarkdownTableRow(parts)
}

func sanitizeMarkdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "\n", "<br>")
	value = strings.ReplaceAll(value, "|", "\\|")

	return value
}

func renderBlockNoteInlineContent(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var inlines []blockNoteInline
	if err := json.Unmarshal(raw, &inlines); err == nil {
		return renderBlockNoteInlines(inlines)
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	return ""
}

func renderBlockNotePlainText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var inlines []blockNoteInline
	if err := json.Unmarshal(raw, &inlines); err == nil {
		var builder strings.Builder
		for _, inline := range inlines {
			builder.WriteString(inlinePlainText(inline))
		}
		return builder.String()
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	return ""
}

func renderBlockNoteInlines(inlines []blockNoteInline) string {
	var builder strings.Builder
	for _, inline := range inlines {
		builder.WriteString(renderBlockNoteInline(inline))
	}

	return builder.String()
}

func renderBlockNoteInline(inline blockNoteInline) string {
	switch inline.Type {
	case "text":
		return applyBlockNoteStyles(inline.Text, inline.Styles)
	case "link":
		text := renderBlockNoteInlines(inline.Content)
		if text == "" {
			text = inline.Href
		}
		return fmt.Sprintf("[%s](%s)", escapeMarkdownLinkText(text), inline.Href)
	default:
		if len(inline.Content) > 0 {
			return renderBlockNoteInlines(inline.Content)
		}
		return inline.Text
	}
}

func inlinePlainText(inline blockNoteInline) string {
	if inline.Type == "link" {
		return renderBlockNoteInlines(inline.Content)
	}
	if len(inline.Content) > 0 {
		return renderBlockNoteInlines(inline.Content)
	}
	return inline.Text
}

func applyBlockNoteStyles(text string, styles map[string]any) string {
	if text == "" || len(styles) == 0 {
		return text
	}

	result := text
	if boolStyle(styles, "code") {
		result = "`" + strings.ReplaceAll(result, "`", "\\`") + "`"
	}
	if boolStyle(styles, "bold") {
		result = "**" + result + "**"
	}
	if boolStyle(styles, "italic") {
		result = "*" + result + "*"
	}
	if boolStyle(styles, "strike") {
		result = "~~" + result + "~~"
	}
	if boolStyle(styles, "underline") {
		result = "<u>" + result + "</u>"
	}

	return result
}

func boolStyle(styles map[string]any, key string) bool {
	value, ok := styles[key]
	if !ok {
		return false
	}

	return value == true
}

func stringProp(props BlockNoteProps, key string) string {
	value, ok := props[key]
	if !ok || value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func intProp(props BlockNoteProps, key string, fallback int) int {
	value, ok := props[key]
	if !ok || value == nil {
		return fallback
	}

	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		result, err := v.Int64()
		if err != nil {
			return fallback
		}
		return int(result)
	case string:
		result, err := strconv.Atoi(v)
		if err != nil {
			return fallback
		}
		return result
	default:
		return fallback
	}
}

func boolProp(props BlockNoteProps, key string) bool {
	value, ok := props[key]
	if !ok || value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	default:
		return false
	}
}

func isBlockNoteListType(blockType string) bool {
	switch blockType {
	case "bulletListItem", "numberedListItem", "checkListItem", "toggleListItem":
		return true
	default:
		return false
	}
}

func fileNameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}

	return parts[len(parts)-1]
}

func removeAttachURLHost(rawURL, bucketName string) string {
	res, err := url.Parse(rawURL)
	if err != nil || !res.IsAbs() {
		return rawURL
	}

	if bucketName != "" && strings.HasPrefix(res.Path, "/"+bucketName+"/") {
		res.Path = strings.TrimPrefix(res.Path, "/"+bucketName)
	}

	res.Scheme = ""
	res.Host = ""

	return res.RequestURI()
}

func escapeMarkdownAlt(value string) string {
	value = strings.ReplaceAll(value, "]", "\\]")
	value = strings.ReplaceAll(value, "\n", " ")

	return value
}

func escapeMarkdownLinkText(value string) string {
	value = strings.ReplaceAll(value, "[", "\\[")
	value = strings.ReplaceAll(value, "]", "\\]")
	value = strings.ReplaceAll(value, "\n", " ")

	return value
}
