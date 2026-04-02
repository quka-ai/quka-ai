package editorjs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/davidscottmills/goeditorjs"
)

func TestConvertEditorJSRawToMarkdown_VideoPreservesSourceURL(t *testing.T) {
	SetupGlobalEditorJS("static.example.com")

	content := BlockContent{
		Blocks: []goeditorjs.EditorJSBlock{
			{
				Type: "video",
				Data: mustMarshalEditorJSData(t, EditorVideo{
					File: EditorVideoFile{
						Type: "video/mp4",
						URL:  "/object/uploads/demo.mp4?a=123123",
					},
					Caption: "demo",
				}),
			},
		},
	}

	raw, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	markdown, err := ConvertEditorJSRawToMarkdown(raw)
	if err != nil {
		t.Fatalf("ConvertEditorJSRawToMarkdown() error = %v", err)
	}

	if !strings.Contains(markdown, `<source src="https://static.example.com/object/uploads/demo.mp4?a=123123">`) {
		t.Fatalf("expected video source URL in markdown, got %q", markdown)
	}

	t.Log(markdown)
}

func TestBuildStaticResourceURL(t *testing.T) {
	tests := []struct {
		name         string
		rawURL       string
		staticDomain string
		expected     string
	}{
		{
			name:         "relative path with static domain",
			rawURL:       "/object/uploads/demo.mp4",
			staticDomain: "static.example.com",
			expected:     "https://static.example.com/object/uploads/demo.mp4",
		},
		{
			name:         "absolute url preserved",
			rawURL:       "https://cdn.example.com/object/uploads/demo.mp4?token=1",
			staticDomain: "static.example.com",
			expected:     "https://cdn.example.com/object/uploads/demo.mp4?token=1",
		},
		{
			name:         "relative path without static domain",
			rawURL:       "/object/uploads/demo.mp4?token=1",
			staticDomain: "",
			expected:     "/object/uploads/demo.mp4?token=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildStaticResourceURL(tt.rawURL, tt.staticDomain); got != tt.expected {
				t.Fatalf("buildStaticResourceURL() = %q, expected %q", got, tt.expected)
			}
		})
	}
}

func mustMarshalEditorJSData(t *testing.T, v any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	return data
}
