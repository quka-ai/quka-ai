package editorjs

import (
	"encoding/json"
	"testing"

	"github.com/davidscottmills/goeditorjs"
)

func TestConvertMarkdownToEditorJSBlocks(t *testing.T) {
	input := "# Title\n\n" +
		"Paragraph line 1\n" +
		"paragraph line 2\n\n" +
		"- item 1\n" +
		"- item 2\n\n" +
		"1. first\n" +
		"2. second\n\n" +
		"- [x] checked\n" +
		"- [ ] pending\n\n" +
		"> quote line 1\n" +
		"> quote line 2\n\n" +
		"![cover](https://example.com/image.png)\n\n" +
		"---\n\n" +
		"```go\n" +
		"fmt.Println(\"hello\")\n" +
		"```\n"

	blocks, err := ConvertMarkdownToEditorJSBlocks(input)
	if err != nil {
		t.Fatalf("ConvertMarkdownToEditorJSBlocks() error = %v", err)
	}

	if got := len(blocks.Blocks); got != 9 {
		t.Fatalf("expected 9 blocks, got %d", got)
	}

	assertBlockType(t, blocks.Blocks[0], "header")
	assertBlockType(t, blocks.Blocks[1], "paragraph")
	assertBlockType(t, blocks.Blocks[2], "listv2")
	assertBlockType(t, blocks.Blocks[3], "listv2")
	assertBlockType(t, blocks.Blocks[4], "listv2")
	assertBlockType(t, blocks.Blocks[5], "quote")
	assertBlockType(t, blocks.Blocks[6], "image")
	assertBlockType(t, blocks.Blocks[7], "delimiter")
	assertBlockType(t, blocks.Blocks[8], "codeBox")
	for index, block := range blocks.Blocks {
		if block.ID == "" {
			t.Fatalf("expected block %d to have id", index)
		}
	}
}

func TestConvertMarkdownToEditorJSRaw(t *testing.T) {
	input := "## Subtitle\n\nSimple paragraph"

	raw, err := ConvertMarkdownToEditorJSRaw(input)
	if err != nil {
		t.Fatalf("ConvertMarkdownToEditorJSRaw() error = %v", err)
	}

	parsed, err := ParseRawToBlocks(raw)
	if err != nil {
		t.Fatalf("ParseRawToBlocks() error = %v", err)
	}

	if got := len(parsed.Blocks); got != 2 {
		t.Fatalf("expected 2 blocks, got %d", got)
	}

	assertBlockType(t, parsed.Blocks[0], "header")
	assertBlockType(t, parsed.Blocks[1], "paragraph")

	var payload struct {
		Blocks []struct {
			ID string `json:"id"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal(raw) error = %v", err)
	}
	if len(payload.Blocks) != 2 {
		t.Fatalf("expected 2 raw blocks, got %d", len(payload.Blocks))
	}
	for index, block := range payload.Blocks {
		if block.ID == "" {
			t.Fatalf("expected block %d to have id", index)
		}
	}
}

func TestConvertMarkdownToEditorJSBlocksNestedChecklist(t *testing.T) {
	input := "- [ ] parent\n  - [x] child"

	blocks, err := ConvertMarkdownToEditorJSBlocks(input)
	if err != nil {
		t.Fatalf("ConvertMarkdownToEditorJSBlocks() error = %v", err)
	}

	if got := len(blocks.Blocks); got != 1 {
		t.Fatalf("expected 1 block, got %d", got)
	}

	var list listv2
	if err := json.Unmarshal(blocks.Blocks[0].Data, &list); err != nil {
		t.Fatalf("unmarshal list data error = %v", err)
	}

	if list.Style != "checklist" {
		t.Fatalf("expected checklist style, got %s", list.Style)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 top-level item, got %d", len(list.Items))
	}
	if len(list.Items[0].Items) == 0 {
		t.Fatal("expected nested child items")
	}

	var nested []listv2Item
	if err := json.Unmarshal(list.Items[0].Items, &nested); err != nil {
		t.Fatalf("unmarshal nested items error = %v", err)
	}
	if len(nested) != 1 {
		t.Fatalf("expected 1 nested item, got %d", len(nested))
	}
	if string(nested[0].Items) != "[]" {
		t.Fatalf("expected nested leaf items to be [], got %s", string(nested[0].Items))
	}
}

func TestConvertMarkdownToEditorJSBlocksChecklistLeafItemsEmptyArray(t *testing.T) {
	input := "- [x] checked\n- [ ] pending"

	blocks, err := ConvertMarkdownToEditorJSBlocks(input)
	if err != nil {
		t.Fatalf("ConvertMarkdownToEditorJSBlocks() error = %v", err)
	}

	if got := len(blocks.Blocks); got != 1 {
		t.Fatalf("expected 1 block, got %d", got)
	}

	var list listv2
	if err := json.Unmarshal(blocks.Blocks[0].Data, &list); err != nil {
		t.Fatalf("unmarshal list data error = %v", err)
	}

	if len(list.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list.Items))
	}
	for index, item := range list.Items {
		if string(item.Items) != "[]" {
			t.Fatalf("expected leaf item %d items to be [], got %s", index, string(item.Items))
		}
	}
}

func assertBlockType(t *testing.T, block goeditorjs.EditorJSBlock, expected string) {
	t.Helper()
	if block.Type != expected {
		t.Fatalf("expected block type %s, got %s", expected, block.Type)
	}
}
