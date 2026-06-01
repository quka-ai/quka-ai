package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/types"
)

const (
	FUNCTION_NAME_SEARCH_MEMORIES   = "SearchUserMemories"
	FUNCTION_NAME_REMEMBER_MEMORY   = "RememberUserMemory"
	FUNCTION_NAME_FORGET_MEMORY     = "ForgetUserMemory"
	defaultMemoryRememberImportance = 70
)

type MemoryLogicFunctions struct {
	Recall   func(ctx context.Context, spaceID string, args RecallArgs) ([]RecallItem, error)
	Remember func(ctx context.Context, spaceID string, args RememberArgs) (string, string, error)
	Forget   func(ctx context.Context, spaceID, id string, deleteKnowledge bool) error
}

type RecallArgs struct {
	Query       string
	Limit       int
	Scopes      []types.MemoryScope
	MemoryTypes []types.MemoryType
}

type RecallItem struct {
	Memory    *types.Memory
	Knowledge *types.Knowledge
}

type RememberArgs struct {
	Resource        string
	Title           string
	Content         types.KnowledgeContent
	ContentType     types.KnowledgeContentType
	Kind            types.KnowledgeKind
	MemoryType      types.MemoryType
	Scope           types.MemoryScope
	AuthorType      types.MemoryAuthorType
	EpistemicStatus types.MemoryEpistemicStatus
	EntityKey       string
	Importance      int16
	Confidence      float64
	SourceKind      types.MemorySourceKind
	SourceRef       string
}

func GetMemoryToolsWithLogic(core *core.Core, spaceID, sessionID, userID string, memoryFuncs MemoryLogicFunctions) []tool.InvokableTool {
	return []tool.InvokableTool{
		NewSearchMemoryTool(core, spaceID, sessionID, userID, memoryFuncs),
		NewRememberMemoryTool(core, spaceID, sessionID, userID, memoryFuncs),
		NewForgetMemoryTool(core, spaceID, sessionID, userID, memoryFuncs),
	}
}

type SearchMemoryTool struct {
	core        *core.Core
	spaceID     string
	sessionID   string
	userID      string
	memoryFuncs MemoryLogicFunctions
}

func NewSearchMemoryTool(core *core.Core, spaceID, sessionID, userID string, memoryFuncs MemoryLogicFunctions) *SearchMemoryTool {
	return &SearchMemoryTool{core: core, spaceID: spaceID, sessionID: sessionID, userID: userID, memoryFuncs: memoryFuncs}
}

var _ tool.InvokableTool = (*SearchMemoryTool)(nil)

func (t *SearchMemoryTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := map[string]*schema.ParameterInfo{
		"query": {Type: schema.String, Desc: "Search query for persistent user memories.", Required: true},
		"limit": {Type: schema.Integer, Desc: "Maximum memories to return. Use 5-8 for normal chat.", Required: false},
	}
	return &schema.ToolInfo{
		Name:        FUNCTION_NAME_SEARCH_MEMORIES,
		Desc:        `Search persistent memories for this user. Use this when the user asks about prior preferences, saved facts, past decisions, project context, or anything they likely expect you to remember. The search covers user_global memories, user_space memories for the current space, and shared memories for the current space.`,
		ParamsOneOf: schema.NewParamsOneOfByParams(params),
	}, nil
}

func (t *SearchMemoryTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("invalid memory search arguments: %w", err)
	}
	if strings.TrimSpace(params.Query) == "" {
		return "No memory search query provided.", nil
	}
	if params.Limit <= 0 {
		params.Limit = 6
	}
	items, err := t.memoryFuncs.Recall(ctx, t.spaceID, RecallArgs{
		Query:       params.Query,
		Limit:       params.Limit,
		Scopes:      []types.MemoryScope{types.MEMORY_SCOPE_USER},
		MemoryTypes: []types.MemoryType{types.MEMORY_TYPE_CORE, types.MEMORY_TYPE_SEMANTIC, types.MEMORY_TYPE_EPISODIC},
	})
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "No relevant memories found.", nil
	}
	var b strings.Builder
	b.WriteString("Relevant memories:\n")
	for _, item := range items {
		layer := types.MEMORY_LAYER_USER_SPACE.String()
		if item.Memory.SpaceID == types.GLOBAL_MEMORY_SPACE_ID {
			layer = types.MEMORY_LAYER_USER_GLOBAL.String()
		} else if item.Memory.Scope == types.MEMORY_SCOPE_SPACE {
			layer = types.MEMORY_LAYER_SPACE_SHARED.String()
		}
		b.WriteString(fmt.Sprintf("- memory_id: %s\n  layer: %s\n  type: %s\n  title: %s\n  content: %s\n",
			item.Memory.ID, layer, item.Memory.MemoryType, item.Knowledge.Title, strings.TrimSpace(item.Knowledge.Content.String())))
	}
	return b.String(), nil
}

type RememberMemoryTool struct {
	core        *core.Core
	spaceID     string
	sessionID   string
	userID      string
	memoryFuncs MemoryLogicFunctions
}

func NewRememberMemoryTool(core *core.Core, spaceID, sessionID, userID string, memoryFuncs MemoryLogicFunctions) *RememberMemoryTool {
	return &RememberMemoryTool{core: core, spaceID: spaceID, sessionID: sessionID, userID: userID, memoryFuncs: memoryFuncs}
}

var _ tool.InvokableTool = (*RememberMemoryTool)(nil)

func (t *RememberMemoryTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := map[string]*schema.ParameterInfo{
		"layer":       {Type: schema.String, Desc: "Memory layer: user_global for cross-space user preferences, user_space for private facts only valid in this space, or space_shared for memories intentionally shared with this space.", Required: true},
		"content":     {Type: schema.String, Desc: "A concise durable memory to save. Do not include hidden reasoning, raw logs, secrets, or temporary task steps.", Required: true},
		"title":       {Type: schema.String, Desc: "Short title for the memory.", Required: false},
		"memory_type": {Type: schema.String, Desc: "core, semantic, or episodic. Use core for stable preferences, semantic for durable facts, episodic for notable events.", Required: false},
		"entity_key":  {Type: schema.String, Desc: "Optional entity key such as project, repo, person, or topic.", Required: false},
		"importance":  {Type: schema.Integer, Desc: "Importance from 1 to 100. Default 70.", Required: false},
	}
	return &schema.ToolInfo{
		Name: FUNCTION_NAME_REMEMBER_MEMORY,
		Desc: `Create or update a persistent memory.
WHEN TO REMEMBER:
- The user explicitly asks you to remember something.
- The user states stable preferences, repeated corrections, identity-level facts, or durable project conventions.
- A decision or constraint will reduce future repetition.
DO NOT REMEMBER:
- Temporary task steps, raw logs, large code blocks, speculative guesses, secrets, or short-lived state.
Choose user_global only for cross-space user preferences. Choose user_space for current-space/project-specific private memories. Choose space_shared only when the user explicitly wants the memory shared with space members or the content is clearly a shared team/project agreement.`,
		ParamsOneOf: schema.NewParamsOneOfByParams(params),
	}, nil
}

func (t *RememberMemoryTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params struct {
		Layer      string `json:"layer"`
		Content    string `json:"content"`
		Title      string `json:"title"`
		MemoryType string `json:"memory_type"`
		EntityKey  string `json:"entity_key"`
		Importance int16  `json:"importance"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("invalid remember arguments: %w", err)
	}
	content := strings.TrimSpace(params.Content)
	if content == "" {
		return "No durable memory content provided.", nil
	}
	targetSpaceID, layer, scope := resolveLayer(t.spaceID, params.Layer)
	memoryType := types.MemoryType(params.MemoryType)
	if memoryType == "" {
		memoryType = types.MEMORY_TYPE_SEMANTIC
	}
	importance := params.Importance
	if importance == 0 {
		importance = defaultMemoryRememberImportance
	}
	memoryID, knowledgeID, err := t.memoryFuncs.Remember(ctx, targetSpaceID, RememberArgs{
		Resource:        types.DEFAULT_RESOURCE,
		Title:           params.Title,
		Content:         types.KnowledgeContent(content),
		ContentType:     types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN,
		Kind:            types.KNOWLEDGE_KIND_TEXT,
		MemoryType:      memoryType,
		Scope:           scope,
		AuthorType:      types.MEMORY_AUTHOR_AGENT,
		EpistemicStatus: types.MEMORY_EPISTEMIC_OBSERVED,
		EntityKey:       params.EntityKey,
		Importance:      importance,
		Confidence:      0.85,
		SourceKind:      types.MEMORY_SOURCE_CHAT,
		SourceRef:       t.sessionID,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Memory saved. memory_id=%s knowledge_id=%s layer=%s", memoryID, knowledgeID, layer), nil
}

type ForgetMemoryTool struct {
	core        *core.Core
	spaceID     string
	sessionID   string
	userID      string
	memoryFuncs MemoryLogicFunctions
}

func NewForgetMemoryTool(core *core.Core, spaceID, sessionID, userID string, memoryFuncs MemoryLogicFunctions) *ForgetMemoryTool {
	return &ForgetMemoryTool{core: core, spaceID: spaceID, sessionID: sessionID, userID: userID, memoryFuncs: memoryFuncs}
}

var _ tool.InvokableTool = (*ForgetMemoryTool)(nil)

func (t *ForgetMemoryTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := map[string]*schema.ParameterInfo{
		"memory_id": {Type: schema.String, Desc: "The memory_id to forget. Search memories first if you do not know it.", Required: true},
	}
	return &schema.ToolInfo{
		Name:        FUNCTION_NAME_FORGET_MEMORY,
		Desc:        "Forget an obsolete or incorrect memory. This removes the memory and its backing knowledge/chunk/vector content so it cannot pollute future context. Use only when the memory is clearly wrong, outdated, duplicated, or the user asks you to forget it.",
		ParamsOneOf: schema.NewParamsOneOfByParams(params),
	}, nil
}

func (t *ForgetMemoryTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params struct {
		MemoryID string `json:"memory_id"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("invalid forget arguments: %w", err)
	}
	if strings.TrimSpace(params.MemoryID) == "" {
		return "No memory_id provided.", nil
	}
	if err := t.memoryFuncs.Forget(ctx, t.spaceID, params.MemoryID, true); err != nil {
		return "", err
	}
	return "Memory forgotten and backing knowledge removed.", nil
}

func resolveLayer(currentSpaceID, layer string) (string, string, types.MemoryScope) {
	switch strings.ToLower(strings.TrimSpace(layer)) {
	case types.MEMORY_LAYER_USER_GLOBAL.String(), "global", "user":
		return types.GLOBAL_MEMORY_SPACE_ID, types.MEMORY_LAYER_USER_GLOBAL.String(), types.MEMORY_SCOPE_USER
	case types.MEMORY_LAYER_SPACE_SHARED.String(), "shared", "space":
		return currentSpaceID, types.MEMORY_LAYER_SPACE_SHARED.String(), types.MEMORY_SCOPE_SPACE
	default:
		return currentSpaceID, types.MEMORY_LAYER_USER_SPACE.String(), types.MEMORY_SCOPE_USER
	}
}
