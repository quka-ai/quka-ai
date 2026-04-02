package editorjs

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/davidscottmills/goeditorjs"
)

var (
	markdownHeaderPattern    = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	markdownImagePattern     = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)]+)\)\s*$`)
	markdownChecklistPattern = regexp.MustCompile(`^(\s*)[-*+]\s+\[([ xX])\]\s+(.*)$`)
	markdownOrderedPattern   = regexp.MustCompile(`^(\s*)(\d+)\.\s+(.*)$`)
	markdownUnorderedPattern = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
)

type markdownListLine struct {
	indent  int
	content string
	checked bool
	style   string
}

type editorHeader struct {
	Text  string `json:"text"`
	Level int    `json:"level"`
}

type editorCodeBox struct {
	Code     string `json:"code"`
	Language string `json:"language"`
}

// ConvertMarkdownToEditorJSBlocks 将 markdown 转为 EditorJS block 内容
func ConvertMarkdownToEditorJSBlocks(markdown string) (*BlockContent, error) {
	lines := splitMarkdownLines(markdown)
	blocks := make([]goeditorjs.EditorJSBlock, 0, len(lines))

	for index := 0; index < len(lines); {
		currentLine := lines[index]
		trimmed := strings.TrimSpace(currentLine)
		if trimmed == "" {
			index++
			continue
		}

		if isFenceStart(trimmed) {
			block, next, err := parseCodeBlock(lines, index)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
			index = next
			continue
		}

		if isDelimiter(trimmed) {
			block, err := marshalBlock("delimiter", line{
				Style:         "solid",
				LineThickness: 1,
				LineWidth:     100,
			})
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
			index++
			continue
		}

		if match := markdownHeaderPattern.FindStringSubmatch(trimmed); match != nil {
			block, err := marshalBlock("header", editorHeader{
				Text:  strings.TrimSpace(match[2]),
				Level: len(match[1]),
			})
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
			index++
			continue
		}

		if match := markdownImagePattern.FindStringSubmatch(trimmed); match != nil {
			block, err := marshalBlock("image", EditorImage{
				File:    EditorImageFile{URL: strings.TrimSpace(match[2])},
				Caption: strings.TrimSpace(match[1]),
			})
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
			index++
			continue
		}

		if strings.HasPrefix(trimmed, ">") {
			block, next, err := parseQuoteBlock(lines, index)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
			index = next
			continue
		}

		if parsed, ok := parseMarkdownListLine(currentLine); ok {
			items, next, err := parseListItems(lines, index, parsed.indent, parsed.style)
			if err != nil {
				return nil, err
			}
			block, err := marshalBlock("listv2", listv2{
				Style: parsed.style,
				Items: items,
			})
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
			index = next
			continue
		}

		block, next, err := parseParagraphBlock(lines, index)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
		index = next
	}

	return &BlockContent{
		Blocks:  blocks,
		Time:    time.Now().UnixMilli(),
		Version: "2.0.0",
	}, nil
}

// ConvertMarkdownToEditorJSRaw 将 markdown 转为 EditorJS 原始 JSON
func ConvertMarkdownToEditorJSRaw(markdown string) (json.RawMessage, error) {
	blocks, err := ConvertMarkdownToEditorJSBlocks(markdown)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(blocks)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func splitMarkdownLines(markdown string) []string {
	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.Split(normalized, "\n")
}

func marshalBlock(blockType string, data any) (goeditorjs.EditorJSBlock, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return goeditorjs.EditorJSBlock{}, err
	}
	id, err := generateEditorJSBlockID()
	if err != nil {
		return goeditorjs.EditorJSBlock{}, err
	}
	return goeditorjs.EditorJSBlock{
		ID:   id,
		Type: blockType,
		Data: raw,
	}, nil
}

func parseCodeBlock(lines []string, start int) (goeditorjs.EditorJSBlock, int, error) {
	first := strings.TrimSpace(lines[start])
	fence := first[:3]
	language := strings.TrimSpace(first[3:])
	collected := make([]string, 0, 8)
	index := start + 1
	for ; index < len(lines); index++ {
		if strings.HasPrefix(strings.TrimSpace(lines[index]), fence) {
			index++
			break
		}
		collected = append(collected, lines[index])
	}

	block, err := marshalBlock("codeBox", editorCodeBox{
		Code:     strings.Join(collected, "\n"),
		Language: language,
	})
	if err != nil {
		return goeditorjs.EditorJSBlock{}, start, err
	}
	return block, index, nil
}

func parseQuoteBlock(lines []string, start int) (goeditorjs.EditorJSBlock, int, error) {
	collected := []string{}
	index := start
	for ; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" {
			break
		}
		if !strings.HasPrefix(trimmed, ">") {
			break
		}
		text := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
		collected = append(collected, text)
	}

	block, err := marshalBlock("quote", quote{
		Text:      strings.Join(collected, "\n"),
		Caption:   "",
		Alignment: "left",
	})
	if err != nil {
		return goeditorjs.EditorJSBlock{}, start, err
	}
	return block, index, nil
}

func parseParagraphBlock(lines []string, start int) (goeditorjs.EditorJSBlock, int, error) {
	collected := []string{}
	index := start
	for ; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" {
			break
		}
		if isFenceStart(trimmed) || isDelimiter(trimmed) || strings.HasPrefix(trimmed, ">") {
			break
		}
		if markdownHeaderPattern.MatchString(trimmed) || markdownImagePattern.MatchString(trimmed) {
			break
		}
		if _, ok := parseMarkdownListLine(lines[index]); ok {
			break
		}
		collected = append(collected, trimmed)
	}

	block, err := marshalBlock("paragraph", EditorParagraph{
		Text:      strings.Join(collected, " "),
		Alignment: "left",
	})
	if err != nil {
		return goeditorjs.EditorJSBlock{}, start, err
	}
	return block, index, nil
}

func parseListItems(lines []string, start, baseIndent int, style string) ([]listv2Item, int, error) {
	items := []listv2Item{}
	index := start
	for index < len(lines) {
		if strings.TrimSpace(lines[index]) == "" {
			break
		}

		current, ok := parseMarkdownListLine(lines[index])
		if !ok {
			break
		}
		if current.indent < baseIndent {
			break
		}
		if current.indent > baseIndent {
			if len(items) == 0 {
				break
			}
			children, next, err := parseListItems(lines, index, current.indent, style)
			if err != nil {
				return nil, index, err
			}
			raw, err := json.Marshal(children)
			if err != nil {
				return nil, index, err
			}
			if len(raw) != 0 {
				items[len(items)-1].Items = raw
			} else {
				items[len(items)-1].Items = []byte("[]")
			}

			index = next
			continue
		}
		if current.style != style {
			break
		}

		items = append(items, listv2Item{
			Content: current.content,
			Items:   json.RawMessage("[]"),
			Meta: listV2ItemMeta{
				Checked: current.checked,
			},
		})
		index++
	}
	return items, index, nil
}

func parseMarkdownListLine(line string) (markdownListLine, bool) {
	if match := markdownChecklistPattern.FindStringSubmatch(line); match != nil {
		return markdownListLine{
			indent:  leadingIndentWidth(match[1]),
			content: strings.TrimSpace(match[3]),
			checked: strings.EqualFold(match[2], "x"),
			style:   "checklist",
		}, true
	}
	if match := markdownOrderedPattern.FindStringSubmatch(line); match != nil {
		return markdownListLine{
			indent:  leadingIndentWidth(match[1]),
			content: strings.TrimSpace(match[3]),
			style:   "ordered",
		}, true
	}
	if match := markdownUnorderedPattern.FindStringSubmatch(line); match != nil {
		return markdownListLine{
			indent:  leadingIndentWidth(match[1]),
			content: strings.TrimSpace(match[2]),
			style:   "unordered",
		}, true
	}
	return markdownListLine{}, false
}

func leadingIndentWidth(prefix string) int {
	width := 0
	for _, ch := range prefix {
		if ch == '\t' {
			width += 4
			continue
		}
		width++
	}
	return width
}

func isFenceStart(line string) bool {
	return strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~")
}

func isDelimiter(line string) bool {
	if len(line) < 3 {
		return false
	}
	switch strings.Trim(line, "-") {
	case "":
		return true
	}
	switch strings.Trim(line, "*") {
	case "":
		return true
	}
	switch strings.Trim(line, "_") {
	case "":
		return true
	}
	return false
}
