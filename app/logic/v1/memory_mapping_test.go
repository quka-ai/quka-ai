package v1

import (
	"reflect"
	"strings"
	"testing"

	"github.com/quka-ai/quka-ai/pkg/types"
)

func TestResolveMemoryLayer(t *testing.T) {
	tests := []struct {
		name      string
		layer     types.MemoryLayer
		fallback  types.MemoryScope
		wantSpace string
		wantScope types.MemoryScope
	}{
		{
			name:      "user global",
			layer:     types.MEMORY_LAYER_USER_GLOBAL,
			wantSpace: types.GLOBAL_MEMORY_SPACE_ID,
			wantScope: types.MEMORY_SCOPE_USER,
		},
		{
			name:      "user space",
			layer:     types.MEMORY_LAYER_USER_SPACE,
			wantSpace: "space_1",
			wantScope: types.MEMORY_SCOPE_USER,
		},
		{
			name:      "space shared",
			layer:     types.MEMORY_LAYER_SPACE_SHARED,
			wantSpace: "space_1",
			wantScope: types.MEMORY_SCOPE_SPACE,
		},
		{
			name:      "fallback scope",
			fallback:  types.MEMORY_SCOPE_SPACE,
			wantSpace: "space_1",
			wantScope: types.MEMORY_SCOPE_SPACE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpace, gotScope := ResolveMemoryLayer("space_1", tt.layer, tt.fallback)
			if gotSpace != tt.wantSpace || gotScope != tt.wantScope {
				t.Fatalf("ResolveMemoryLayer() = (%q, %q), want (%q, %q)", gotSpace, gotScope, tt.wantSpace, tt.wantScope)
			}
		})
	}
}

func TestMemoryLayerOf(t *testing.T) {
	tests := []struct {
		name   string
		memory *types.Memory
		want   types.MemoryLayer
	}{
		{
			name:   "nil defaults to user space",
			memory: nil,
			want:   types.MEMORY_LAYER_USER_SPACE,
		},
		{
			name: "user global",
			memory: &types.Memory{
				SpaceID: types.GLOBAL_MEMORY_SPACE_ID,
				Scope:   types.MEMORY_SCOPE_USER,
			},
			want: types.MEMORY_LAYER_USER_GLOBAL,
		},
		{
			name: "space shared",
			memory: &types.Memory{
				SpaceID: "space_1",
				Scope:   types.MEMORY_SCOPE_SPACE,
			},
			want: types.MEMORY_LAYER_SPACE_SHARED,
		},
		{
			name: "user space",
			memory: &types.Memory{
				SpaceID: "space_1",
				Scope:   types.MEMORY_SCOPE_USER,
			},
			want: types.MEMORY_LAYER_USER_SPACE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MemoryLayerOf(tt.memory); got != tt.want {
				t.Fatalf("MemoryLayerOf() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildMemoryRecallVectorOptionsUsesAccessibleKnowledgeOnly(t *testing.T) {
	opts, ok := buildMemoryRecallVectorOptions("space_1", []types.Memory{
		{
			ID:          "mem_user_a_private",
			SpaceID:     "space_1",
			UserID:      "user_a",
			KnowledgeID: "kg_user_a_private",
			Scope:       types.MEMORY_SCOPE_USER,
		},
		{
			ID:          "mem_user_b_shared",
			SpaceID:     "space_1",
			UserID:      "user_b",
			KnowledgeID: "kg_user_b_shared",
			Scope:       types.MEMORY_SCOPE_SPACE,
		},
		{
			ID:          "mem_user_a_global",
			SpaceID:     types.GLOBAL_MEMORY_SPACE_ID,
			UserID:      "user_a",
			KnowledgeID: "kg_user_a_global",
			Scope:       types.MEMORY_SCOPE_USER,
		},
		{
			ID:      "mem_without_knowledge",
			SpaceID: "space_1",
			UserID:  "user_a",
			Scope:   types.MEMORY_SCOPE_USER,
		},
	})
	if !ok {
		t.Fatal("buildMemoryRecallVectorOptions() returned ok=false")
	}

	if opts.UserID != "" {
		t.Fatalf("UserID = %q, want empty; vector recall must not re-filter by current user after memory access prefilter", opts.UserID)
	}
	if want := []string{types.GLOBAL_MEMORY_SPACE_ID, "space_1"}; !reflect.DeepEqual(opts.SpaceIDs, want) {
		t.Fatalf("SpaceIDs = %#v, want %#v", opts.SpaceIDs, want)
	}
	wantKnowledgeIDs := []string{"kg_user_a_private", "kg_user_b_shared", "kg_user_a_global"}
	if !reflect.DeepEqual(opts.KnowledgeIDs, wantKnowledgeIDs) {
		t.Fatalf("KnowledgeIDs = %#v, want %#v", opts.KnowledgeIDs, wantKnowledgeIDs)
	}
}

func TestBuildMemoryRecallVectorOptionsRejectsEmptyKnowledgeSet(t *testing.T) {
	opts, ok := buildMemoryRecallVectorOptions("space_1", []types.Memory{
		{ID: "mem_1", SpaceID: "space_1", UserID: "user_a", Scope: types.MEMORY_SCOPE_USER},
	})
	if ok {
		t.Fatalf("ok = true, want false; opts = %#v", opts)
	}
}

func TestKnowledgeSourceMemoryDefaults(t *testing.T) {
	if got := knowledgeSourceToMemorySource(types.KNOWLEDGE_SOURCE_PLATFORM); got != types.MEMORY_SOURCE_MANUAL {
		t.Fatalf("platform source mapped to %q, want manual", got)
	}
	if got := knowledgeSourceToMemorySource(types.KNOWLEDGE_SOURCE_MCP); got != types.MEMORY_SOURCE_MCP {
		t.Fatalf("mcp source mapped to %q, want mcp", got)
	}
	if got := knowledgeSourceToMemoryAuthor(types.KNOWLEDGE_SOURCE_PLATFORM); got != types.MEMORY_AUTHOR_HUMAN {
		t.Fatalf("platform author mapped to %q, want human", got)
	}
	if got := knowledgeSourceToMemoryAuthor(types.KNOWLEDGE_SOURCE_CHAT); got != types.MEMORY_AUTHOR_AGENT {
		t.Fatalf("chat author mapped to %q, want agent", got)
	}
}

func TestFilterMemoryBackingResource(t *testing.T) {
	resource, empty := filterMemoryBackingResource(nil)
	if empty {
		t.Fatal("nil resource should not produce an empty result")
	}
	if resource == nil || !reflect.DeepEqual(resource.Exclude, []string{types.MEMORY_BACKING_RESOURCE}) {
		t.Fatalf("resource = %#v, want memory backing resource excluded", resource)
	}

	resource, empty = filterMemoryBackingResource(&types.ResourceQuery{
		Include: []string{types.DEFAULT_RESOURCE, types.MEMORY_BACKING_RESOURCE},
	})
	if empty {
		t.Fatal("mixed include resource should not produce an empty result")
	}
	if !reflect.DeepEqual(resource.Include, []string{types.DEFAULT_RESOURCE}) {
		t.Fatalf("Include = %#v, want only default resource", resource.Include)
	}

	resource, empty = filterMemoryBackingResource(&types.ResourceQuery{
		Include: []string{types.MEMORY_BACKING_RESOURCE},
	})
	if !empty {
		t.Fatalf("memory-only include should produce empty result, resource = %#v", resource)
	}
}

func TestIsMemoryBackingKnowledge(t *testing.T) {
	if !isMemoryBackingKnowledge(&types.Knowledge{Resource: types.MEMORY_BACKING_RESOURCE}) {
		t.Fatal("memory backing resource should be treated as internal memory knowledge")
	}
	if isMemoryBackingKnowledge(&types.Knowledge{Resource: types.DEFAULT_RESOURCE}) {
		t.Fatal("default resource should not be treated as internal memory knowledge")
	}
}

func TestCanMutateMemory(t *testing.T) {
	privateMemory := &types.Memory{
		SpaceID: "space_1",
		UserID:  "user_a",
		Scope:   types.MEMORY_SCOPE_USER,
	}
	sharedMemory := &types.Memory{
		SpaceID: "space_1",
		UserID:  "user_a",
		Scope:   types.MEMORY_SCOPE_SPACE,
	}
	globalSharedMemory := &types.Memory{
		SpaceID: types.GLOBAL_MEMORY_SPACE_ID,
		UserID:  "user_a",
		Scope:   types.MEMORY_SCOPE_SPACE,
	}

	if !canMutateMemory(privateMemory, "user_a", false) {
		t.Fatal("owner should mutate private memory")
	}
	if canMutateMemory(privateMemory, "user_b", true) {
		t.Fatal("space edit should not mutate another user's private memory")
	}
	if canMutateMemory(sharedMemory, "user_b", false) {
		t.Fatal("space shared memory requires edit permission")
	}
	if !canMutateMemory(sharedMemory, "user_b", true) {
		t.Fatal("space editor should mutate shared memory")
	}
	if canMutateMemory(globalSharedMemory, "user_b", true) {
		t.Fatal("global space-shared memory is not a valid mutation target")
	}
}

func TestCanCreateMemory(t *testing.T) {
	if !canCreateMemory("space_1", types.MEMORY_SCOPE_USER, false) {
		t.Fatal("user-scoped memory should not require space edit permission")
	}
	if !canCreateMemory("space_1", "", false) {
		t.Fatal("empty scope should default to user-scoped memory")
	}
	if canCreateMemory("space_1", types.MEMORY_SCOPE_SPACE, false) {
		t.Fatal("space-shared memory should require space edit permission")
	}
	if !canCreateMemory("space_1", types.MEMORY_SCOPE_SPACE, true) {
		t.Fatal("space editor should create space-shared memory")
	}
	if canCreateMemory(types.GLOBAL_MEMORY_SPACE_ID, types.MEMORY_SCOPE_SPACE, true) {
		t.Fatal("global space-shared memory is not a valid create target")
	}
	if canCreateMemory("space_1", types.MEMORY_SCOPE_AGENT, true) {
		t.Fatal("unsupported memory scope should not be created through remember")
	}
}

func TestValidateRuntimeContext(t *testing.T) {
	if err := validateRuntimeContext(nil, false); err != nil {
		t.Fatalf("optional nil runtime context error = %v", err)
	}
	if err := validateRuntimeContext(nil, true); err == nil {
		t.Fatal("required nil runtime context should fail")
	}
	if err := validateRuntimeContext(&types.RuntimeContext{Type: types.MEMORY_CONTEXT_AGENT_RUN}, true); err == nil {
		t.Fatal("runtime context without id should fail")
	}
	if err := validateRuntimeContext(&types.RuntimeContext{Type: types.MemoryContextType("unknown"), ID: "run_1"}, true); err == nil {
		t.Fatal("unsupported runtime context type should fail")
	}
	runtimeContext := &types.RuntimeContext{Type: types.MEMORY_CONTEXT_AGENT_RUN, ID: " run_1 "}
	if err := validateRuntimeContext(runtimeContext, true); err != nil {
		t.Fatalf("valid agent_run runtime context error = %v", err)
	}
	if runtimeContext.ID != "run_1" {
		t.Fatalf("runtime context id = %q, want trimmed id", runtimeContext.ID)
	}
}

func TestFormatMemoryContextPart(t *testing.T) {
	part := formatMemoryContextPart("Working memory", MemoryRecallItem{
		Memory: &types.Memory{ID: "mem_1"},
		Knowledge: &types.Knowledge{
			Title:   "Pinned decision",
			Content: types.KnowledgeContent(`"Keep the migration user-scoped."`),
		},
	})

	want := "## Working memory: Pinned decision\nKeep the migration user-scoped."
	if part != want {
		t.Fatalf("formatMemoryContextPart() = %q, want %q", part, want)
	}

	part = formatMemoryContextPart("Relevant memory", MemoryRecallItem{
		Memory:    &types.Memory{ID: "mem_fallback"},
		Knowledge: &types.Knowledge{Content: types.KnowledgeContent(`"No title"`)},
	})
	want = "## Relevant memory: mem_fallback\nNo title"
	if part != want {
		t.Fatalf("formatMemoryContextPart() fallback = %q, want %q", part, want)
	}
}

func TestBuildHydratedMemoryContextBlock(t *testing.T) {
	if got := buildHydratedMemoryContextBlock(nil); got != "" {
		t.Fatalf("nil hydration block = %q, want empty", got)
	}

	block := buildHydratedMemoryContextBlock(&HydrateMemoryResult{
		AssembledContext: "## Working memory: Pinned decision\nKeep migration user-scoped.",
	})
	want := "<memory-context>\n" +
		"Relevant persistent memories were preloaded before this turn. Working memories are pinned to the current runtime context and should be prioritized when helpful.\n" +
		"## Working memory: Pinned decision\nKeep migration user-scoped.\n" +
		"</memory-context>"
	if block != want {
		t.Fatalf("buildHydratedMemoryContextBlock() = %q, want %q", block, want)
	}
}

func TestBuildHydrateMemoryContextItem(t *testing.T) {
	item := buildHydrateMemoryContextItem("working", MemoryRecallItem{
		Memory: &types.Memory{
			ID:              "mem_1",
			SpaceID:         types.GLOBAL_MEMORY_SPACE_ID,
			KnowledgeID:     "kn_1",
			MemoryType:      types.MEMORY_TYPE_CORE,
			Scope:           types.MEMORY_SCOPE_USER,
			EntityKey:       "preference:language",
			Confidence:      0.9,
			Importance:      80,
			AuthorType:      types.MEMORY_AUTHOR_HUMAN,
			EpistemicStatus: types.MEMORY_EPISTEMIC_STATED,
			SourceKind:      types.MEMORY_SOURCE_MANUAL,
			SourceRef:       "manual",
		},
		Knowledge: &types.Knowledge{
			ID:      "kn_1",
			Title:   "Language preference",
			Content: types.KnowledgeContent(`"Prefer Chinese responses."`),
		},
	})

	if item.Role != "working" {
		t.Fatalf("Role = %q, want working", item.Role)
	}
	if item.Layer != types.MEMORY_LAYER_USER_GLOBAL.String() {
		t.Fatalf("Layer = %q, want user_global", item.Layer)
	}
	if item.Content != "Prefer Chinese responses." {
		t.Fatalf("Content = %q, want normalized knowledge content", item.Content)
	}
	if item.EntityKey != "preference:language" || item.SourceKind != types.MEMORY_SOURCE_MANUAL.String() {
		t.Fatalf("metadata not preserved: %#v", item)
	}
}

func TestApplyHydrateTokenBudgetPrioritizesWorkingItems(t *testing.T) {
	items := []HydrateMemoryContextItem{
		{
			Role:       "working",
			MemoryID:   "mem_working",
			MemoryType: types.MEMORY_TYPE_CORE.String(),
			Title:      "Pinned",
			Content:    "Short pinned memory.",
		},
		{
			Role:       "recall",
			MemoryID:   "mem_recall",
			MemoryType: types.MEMORY_TYPE_SEMANTIC.String(),
			Title:      "Long recall",
			Content:    strings.Repeat("long memory ", 80),
		},
	}

	selected := applyHydrateTokenBudget(items, estimateTokenCount(formatHydrateContextItem(items[0]))+1)
	if len(selected) != 1 || selected[0].MemoryID != "mem_working" {
		t.Fatalf("selected = %#v, want only working item", selected)
	}
}

func TestHydrateResultRebuildHydrateViews(t *testing.T) {
	result := &HydrateMemoryResult{
		CoreMemories:           []string{"stale"},
		WorkingMemories:        []string{"stale"},
		RecentEpisodicMemories: []string{"stale"},
		SemanticMemories:       []string{"stale"},
		Items: []HydrateMemoryContextItem{
			{Role: "working", MemoryID: "mem_core", MemoryType: types.MEMORY_TYPE_CORE.String()},
			{Role: "recall", MemoryID: "mem_semantic", MemoryType: types.MEMORY_TYPE_SEMANTIC.String()},
			{Role: "recall", MemoryID: "mem_episode", MemoryType: types.MEMORY_TYPE_EPISODIC.String()},
		},
	}

	result.rebuildHydrateViews()

	if len(result.WorkingMemories) != 1 || result.WorkingMemories[0] != "mem_core" {
		t.Fatalf("WorkingMemories = %#v", result.WorkingMemories)
	}
	if len(result.CoreMemories) != 1 || result.CoreMemories[0] != "mem_core" {
		t.Fatalf("CoreMemories = %#v", result.CoreMemories)
	}
	if len(result.SemanticMemories) != 1 || result.SemanticMemories[0] != "mem_semantic" {
		t.Fatalf("SemanticMemories = %#v", result.SemanticMemories)
	}
	if len(result.RecentEpisodicMemories) != 1 || result.RecentEpisodicMemories[0] != "mem_episode" {
		t.Fatalf("RecentEpisodicMemories = %#v", result.RecentEpisodicMemories)
	}
}
