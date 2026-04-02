package handler

import (
	"testing"

	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
)

func TestKnowledgeToKnowledgeResponseLiteReplacesHiddenContent(t *testing.T) {
	item := &types.Knowledge{
		ID:          "knowledge-1",
		SpaceID:     "space-1",
		ContentType: types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN,
		Content:     types.KnowledgeContent("before $hidden[top-secret] after"),
		UpdatedAt:   1,
		CreatedAt:   1,
	}

	result := KnowledgeToKnowledgeResponseLite(item)
	if result.Content != "before [Secret] after" {
		t.Fatalf("expected hidden content to be replaced, got %q", result.Content)
	}
}

func TestKnowledgeToKnowledgeResponseLiteReplacesHiddenContentInBlocks(t *testing.T) {
	editorjs.SetupGlobalEditorJS("")

	item := &types.Knowledge{
		ID:          "knowledge-2",
		SpaceID:     "space-1",
		ContentType: types.KNOWLEDGE_CONTENT_TYPE_BLOCKS,
		Content: types.KnowledgeContent(`{
			"time": 1710000000000,
			"blocks": [
				{
					"id": "paragraph-1",
					"type": "paragraph",
					"data": {
						"text": "value: $hidden[internal-token]"
					}
				}
			],
			"version": "2.29.1"
		}`),
		UpdatedAt: 1,
		CreatedAt: 1,
	}

	result := KnowledgeToKnowledgeResponseLite(item)
	if result.Content != "value: [Secret]" {
		t.Fatalf("expected hidden content in blocks to be replaced, got %q", result.Content)
	}
	if result.ContentType != types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN {
		t.Fatalf("expected content type to be markdown, got %q", result.ContentType)
	}
}
