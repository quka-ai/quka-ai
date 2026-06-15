package types

import (
	"reflect"
	"strings"
	"testing"

	sq "github.com/Masterminds/squirrel"
)

func TestGetMemoryOptionsApplyAccessibleLayers(t *testing.T) {
	query := sq.Select("id").From(TABLE_MEMORY.Name())
	GetMemoryOptions{
		AccessibleUserID:         "user_a",
		AccessibleSpaceID:        "space_1",
		IncludeGlobalUserMemory:  true,
		IncludeSpaceUserMemory:   true,
		IncludeSpaceSharedMemory: true,
	}.Apply(&query)

	sql, args, err := query.ToSql()
	if err != nil {
		t.Fatalf("ToSql() error = %v", err)
	}

	for _, want := range []string{
		"space_id = ? AND scope = ? AND user_id = ?",
		"space_id = ? AND scope = ?",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("sql %q does not contain %q", sql, want)
		}
	}

	wantArgs := []any{
		GLOBAL_MEMORY_SPACE_ID, MEMORY_SCOPE_USER, "user_a",
		"space_1", MEMORY_SCOPE_USER, "user_a",
		"space_1", MEMORY_SCOPE_SPACE,
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestGetMemoryOptionsApplyAccessibleLayerFlags(t *testing.T) {
	tests := []struct {
		name     string
		opts     GetMemoryOptions
		wantArgs []any
		notArgs  []any
	}{
		{
			name: "space shared only",
			opts: GetMemoryOptions{
				AccessibleUserID:         "user_a",
				AccessibleSpaceID:        "space_1",
				IncludeSpaceSharedMemory: true,
			},
			wantArgs: []any{"space_1", MEMORY_SCOPE_SPACE},
			notArgs:  []any{GLOBAL_MEMORY_SPACE_ID, "user_a"},
		},
		{
			name: "private user layers only",
			opts: GetMemoryOptions{
				AccessibleUserID:        "user_a",
				AccessibleSpaceID:       "space_1",
				IncludeGlobalUserMemory: true,
				IncludeSpaceUserMemory:  true,
			},
			wantArgs: []any{
				GLOBAL_MEMORY_SPACE_ID, MEMORY_SCOPE_USER, "user_a",
				"space_1", MEMORY_SCOPE_USER, "user_a",
			},
			notArgs: []any{MEMORY_SCOPE_SPACE},
		},
		{
			name: "global user only",
			opts: GetMemoryOptions{
				AccessibleUserID:        "user_a",
				AccessibleSpaceID:       "space_1",
				IncludeGlobalUserMemory: true,
			},
			wantArgs: []any{GLOBAL_MEMORY_SPACE_ID, MEMORY_SCOPE_USER, "user_a"},
			notArgs:  []any{"space_1", MEMORY_SCOPE_SPACE},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := sq.Select("id").From(TABLE_MEMORY.Name())
			tt.opts.Apply(&query)

			_, args, err := query.ToSql()
			if err != nil {
				t.Fatalf("ToSql() error = %v", err)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Fatalf("args = %#v, want %#v", args, tt.wantArgs)
			}

			for _, forbidden := range tt.notArgs {
				for _, arg := range args {
					if arg == forbidden {
						t.Fatalf("args = %#v unexpectedly contains %#v", args, forbidden)
					}
				}
			}
		})
	}
}

func TestGetMemoryBindingOptionsApplyUserID(t *testing.T) {
	query := sq.Select("id").From(TABLE_MEMORY_BINDING.Name())
	GetMemoryBindingOptions{
		SpaceID:     "space_1",
		UserID:      "user_a",
		ContextType: MEMORY_CONTEXT_AGENT_RUN,
		ContextID:   "run_1",
	}.Apply(&query)

	sql, args, err := query.ToSql()
	if err != nil {
		t.Fatalf("ToSql() error = %v", err)
	}

	if !strings.Contains(sql, "user_id = ?") {
		t.Fatalf("sql %q does not contain user_id predicate", sql)
	}

	wantArgs := []any{"space_1", "user_a", MEMORY_CONTEXT_AGENT_RUN, "run_1"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestMemoryHasInlineContent(t *testing.T) {
	if !(Memory{Content: KnowledgeContent("short-lived working context")}).HasInlineContent() {
		t.Fatal("memory with inline content should report HasInlineContent")
	}
	if (Memory{Content: KnowledgeContent("   ")}).HasInlineContent() {
		t.Fatal("blank inline content should not report HasInlineContent")
	}
}

func TestRuntimeContextExtractionReflectionContentOmitsReasoning(t *testing.T) {
	content := (&RuntimeContextExtraction{
		Messages: []RuntimeContextMessage{
			{
				Role:      "assistant",
				Content:   "User prefers concise Chinese answers.",
				Reasoning: "hidden chain of thought that must not be persisted",
			},
			{
				Role:      "tool",
				Reasoning: "tool reasoning only",
			},
		},
	}).ReflectionContent()

	if strings.Contains(content, "hidden chain of thought") || strings.Contains(content, "tool reasoning") {
		t.Fatalf("ReflectionContent() persisted reasoning: %q", content)
	}
	if !strings.Contains(content, "assistant: User prefers concise Chinese answers.") {
		t.Fatalf("ReflectionContent() = %q, want assistant content", content)
	}
}
