package editorjs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/davidscottmills/goeditorjs"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func TestConvertBlockNoteRawToMarkdown(t *testing.T) {
	blocks := []BlockNoteBlock{
		{
			ID:   "heading-id",
			Type: "heading",
			Props: BlockNoteProps{
				"level": 2,
			},
			Content: mustMarshalBlockNoteJSON(t, []blockNoteInline{
				{Type: "text", Text: "标题", Styles: map[string]any{}},
			}),
		},
		{
			ID:   "paragraph-id",
			Type: "paragraph",
			Props: BlockNoteProps{
				"textColor":       "default",
				"backgroundColor": "default",
				"textAlignment":   "left",
			},
			Content: mustMarshalBlockNoteJSON(t, []blockNoteInline{
				{Type: "text", Text: "这是一段", Styles: map[string]any{"bold": true}},
				{Type: "text", Text: "正文", Styles: map[string]any{}},
				{Type: "link", Href: "https://example.com", Content: []blockNoteInline{
					{Type: "text", Text: "链接", Styles: map[string]any{}},
				}},
			}),
		},
		{
			ID:   "check-id",
			Type: "checkListItem",
			Props: BlockNoteProps{
				"checked": true,
			},
			Content: mustMarshalBlockNoteJSON(t, []blockNoteInline{
				{Type: "text", Text: "完成任务", Styles: map[string]any{}},
			}),
			Children: []BlockNoteBlock{
				{
					ID:   "child-id",
					Type: "bulletListItem",
					Content: mustMarshalBlockNoteJSON(t, []blockNoteInline{
						{Type: "text", Text: "子任务", Styles: map[string]any{}},
					}),
				},
			},
		},
		{
			ID:   "image-id",
			Type: "image",
			Props: BlockNoteProps{
				"url":     "/object/demo.png",
				"caption": "图片说明",
			},
		},
		{
			ID:   "table-id",
			Type: "table",
			Content: mustMarshalBlockNoteJSON(t, blockNoteTableContent{
				Type: "tableContent",
				Rows: []blockNoteTableRow{
					{Cells: []json.RawMessage{
						mustMarshalBlockNoteJSON(t, []blockNoteInline{{Type: "text", Text: "列 A", Styles: map[string]any{}}}),
						mustMarshalBlockNoteJSON(t, []blockNoteInline{{Type: "text", Text: "列 B", Styles: map[string]any{}}}),
					}},
					{Cells: []json.RawMessage{
						mustMarshalBlockNoteJSON(t, []blockNoteInline{{Type: "text", Text: "值 1", Styles: map[string]any{}}}),
						mustMarshalBlockNoteJSON(t, []blockNoteInline{{Type: "text", Text: "值 | 2", Styles: map[string]any{}}}),
					}},
				},
			}),
		},
	}

	raw := mustMarshalBlockNoteJSON(t, blocks)
	markdown, err := ConvertBlockNoteRawToMarkdown(types.KnowledgeContent(raw))
	if err != nil {
		t.Fatalf("ConvertBlockNoteRawToMarkdown() error = %v", err)
	}

	expectedParts := []string{
		"## 标题",
		"**这是一段**正文[链接](https://example.com)",
		"- [x] 完成任务\n  - 子任务",
		"![图片说明](/object/demo.png)",
		"| 列 A | 列 B |\n| --- | --- |\n| 值 1 | 值 \\| 2 |",
	}
	for _, expected := range expectedParts {
		if !strings.Contains(string(markdown), expected) {
			t.Fatalf("expected markdown to contain %q, got:\n%s", expected, markdown)
		}
	}
}

func TestConvertRawToMarkdownAuto_BlockNote(t *testing.T) {
	raw := mustMarshalBlockNoteJSON(t, []BlockNoteBlock{
		{
			ID:   "paragraph-id",
			Type: "paragraph",
			Props: BlockNoteProps{
				"textColor":       "default",
				"backgroundColor": "default",
				"textAlignment":   "left",
			},
			Content: mustMarshalBlockNoteJSON(t, []blockNoteInline{
				{Type: "text", Text: "BlockNote 日记", Styles: map[string]any{}},
			}),
		},
	})

	markdown, err := ConvertRawToMarkdownAuto(types.KnowledgeContent(raw))
	if err != nil {
		t.Fatalf("ConvertRawToMarkdownAuto() error = %v", err)
	}
	if markdown != "BlockNote 日记" {
		t.Fatalf("ConvertRawToMarkdownAuto() = %q, expected %q", markdown, "BlockNote 日记")
	}
}

func TestConvertRawToMarkdownAuto_EditorJS(t *testing.T) {
	SetupGlobalEditorJS("")

	raw := mustMarshalBlockNoteJSON(t, BlockContent{
		Blocks: []goeditorjs.EditorJSBlock{
			{
				Type: "paragraph",
				Data: mustMarshalBlockNoteJSON(t, EditorParagraph{
					Text: "EditorJS 日记",
				}),
			},
		},
	})

	markdown, err := ConvertRawToMarkdownAuto(types.KnowledgeContent(raw))
	if err != nil {
		t.Fatalf("ConvertRawToMarkdownAuto() error = %v", err)
	}
	if !strings.Contains(markdown, "EditorJS 日记") {
		t.Fatalf("expected markdown to contain EditorJS content, got %q", markdown)
	}
}

func TestConvertRawToMarkdownAuto_PlainText(t *testing.T) {
	markdown, err := ConvertRawToMarkdownAuto(types.KnowledgeContent("普通日记"))
	if err != nil {
		t.Fatalf("ConvertRawToMarkdownAuto() error = %v", err)
	}
	if markdown != "普通日记" {
		t.Fatalf("ConvertRawToMarkdownAuto() = %q, expected %q", markdown, "普通日记")
	}
}

func mustMarshalBlockNoteJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	return data
}
