package types

import (
	"encoding/json"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
)

type MemoryType string

const (
	MEMORY_TYPE_CORE     MemoryType = "core"
	MEMORY_TYPE_EPISODIC MemoryType = "episodic"
	MEMORY_TYPE_SEMANTIC MemoryType = "semantic"
	MEMORY_TYPE_WORKING  MemoryType = "working"
)

type MemoryScope string

const (
	GLOBAL_MEMORY_SPACE_ID = "-"

	MEMORY_SCOPE_USER    MemoryScope = "user"
	MEMORY_SCOPE_AGENT   MemoryScope = "agent"
	MEMORY_SCOPE_SESSION MemoryScope = "session"
	MEMORY_SCOPE_SPACE   MemoryScope = "space"
)

type MemoryLayer string

const (
	MEMORY_LAYER_USER_GLOBAL  MemoryLayer = "user_global"
	MEMORY_LAYER_USER_SPACE   MemoryLayer = "user_space"
	MEMORY_LAYER_SPACE_SHARED MemoryLayer = "space_shared"
)

type MemoryStatus string

const (
	MEMORY_STATUS_ACTIVE     MemoryStatus = "active"
	MEMORY_STATUS_ARCHIVED   MemoryStatus = "archived"
	MEMORY_STATUS_SUPERSEDED MemoryStatus = "superseded"
	MEMORY_STATUS_DELETED    MemoryStatus = "deleted"
)

type MemoryAuthorType string

const (
	MEMORY_AUTHOR_HUMAN  MemoryAuthorType = "human"
	MEMORY_AUTHOR_AGENT  MemoryAuthorType = "agent"
	MEMORY_AUTHOR_SHARED MemoryAuthorType = "shared"
	MEMORY_AUTHOR_SYSTEM MemoryAuthorType = "system"
)

type MemoryEpistemicStatus string

const (
	MEMORY_EPISTEMIC_STATED     MemoryEpistemicStatus = "stated"
	MEMORY_EPISTEMIC_OBSERVED   MemoryEpistemicStatus = "observed"
	MEMORY_EPISTEMIC_INFERRED   MemoryEpistemicStatus = "inferred"
	MEMORY_EPISTEMIC_SUMMARIZED MemoryEpistemicStatus = "summarized"
	MEMORY_EPISTEMIC_CONFIRMED  MemoryEpistemicStatus = "confirmed"
	MEMORY_EPISTEMIC_TENTATIVE  MemoryEpistemicStatus = "tentative"
)

type MemoryConflictState string

const (
	MEMORY_CONFLICT_NONE      MemoryConflictState = "none"
	MEMORY_CONFLICT_SUSPECTED MemoryConflictState = "suspected"
	MEMORY_CONFLICT_CONFIRMED MemoryConflictState = "confirmed"
)

type MemorySourceKind string

const (
	MEMORY_SOURCE_CHAT       MemorySourceKind = "chat"
	MEMORY_SOURCE_MCP        MemorySourceKind = "mcp"
	MEMORY_SOURCE_RSS        MemorySourceKind = "rss"
	MEMORY_SOURCE_MANUAL     MemorySourceKind = "manual"
	MEMORY_SOURCE_REFLECTION MemorySourceKind = "reflection"
	MEMORY_SOURCE_IMPORT     MemorySourceKind = "import"
)

type MemoryRelation string

const (
	MEMORY_RELATION_DERIVED_FROM MemoryRelation = "derived_from"
	MEMORY_RELATION_SUPPORTS     MemoryRelation = "supports"
	MEMORY_RELATION_CONTRADICTS  MemoryRelation = "contradicts"
	MEMORY_RELATION_RELATED_TO   MemoryRelation = "related_to"
	MEMORY_RELATION_SUPERSEDES   MemoryRelation = "supersedes"
	MEMORY_RELATION_PINNED_TO    MemoryRelation = "pinned_to"
)

type MemoryContextType string

const (
	MEMORY_CONTEXT_CHAT_SESSION MemoryContextType = "chat_session"
	MEMORY_CONTEXT_AGENT_RUN    MemoryContextType = "agent_run"
	MEMORY_CONTEXT_TASK         MemoryContextType = "task"
	MEMORY_CONTEXT_WORKSPACE    MemoryContextType = "workspace"
)

type MemoryBindingType string

const (
	MEMORY_BINDING_PIN             MemoryBindingType = "pin"
	MEMORY_BINDING_WORKING         MemoryBindingType = "working"
	MEMORY_BINDING_HYDRATION_CACHE MemoryBindingType = "hydration_cache"
)

type MemoryPinnedBy string

const (
	MEMORY_PINNED_BY_HUMAN  MemoryPinnedBy = "human"
	MEMORY_PINNED_BY_AGENT  MemoryPinnedBy = "agent"
	MEMORY_PINNED_BY_SYSTEM MemoryPinnedBy = "system"
)

type Memory struct {
	ID              string                `json:"id" db:"id"`
	SpaceID         string                `json:"space_id" db:"space_id"`
	UserID          string                `json:"user_id" db:"user_id"`
	KnowledgeID     string                `json:"knowledge_id" db:"knowledge_id"`
	MemoryType      MemoryType            `json:"memory_type" db:"memory_type"`
	Scope           MemoryScope           `json:"scope" db:"scope"`
	Status          MemoryStatus          `json:"status" db:"status"`
	Title           string                `json:"title" db:"title"`
	Content         KnowledgeContent      `json:"content" db:"content"`
	ContentType     KnowledgeContentType  `json:"content_type" db:"content_type"`
	Importance      int16                 `json:"importance" db:"importance"`
	Confidence      float64               `json:"confidence" db:"confidence"`
	AuthorType      MemoryAuthorType      `json:"author_type" db:"author_type"`
	EpistemicStatus MemoryEpistemicStatus `json:"epistemic_status" db:"epistemic_status"`
	SourceKind      MemorySourceKind      `json:"source_kind" db:"source_kind"`
	SourceRef       string                `json:"source_ref" db:"source_ref"`
	EntityKey       string                `json:"entity_key" db:"entity_key"`
	DedupeKey       string                `json:"dedupe_key" db:"dedupe_key"`
	ConflictState   MemoryConflictState   `json:"conflict_state" db:"conflict_state"`
	ValidFrom       int64                 `json:"valid_from" db:"valid_from"`
	ValidTo         int64                 `json:"valid_to" db:"valid_to"`
	LastAccessedAt  int64                 `json:"last_accessed_at" db:"last_accessed_at"`
	AccessCount     int64                 `json:"access_count" db:"access_count"`
	CreatedAt       int64                 `json:"created_at" db:"created_at"`
	UpdatedAt       int64                 `json:"updated_at" db:"updated_at"`
}

func (m Memory) HasInlineContent() bool {
	return strings.TrimSpace(m.Content.String()) != ""
}

type UpdateMemoryArgs struct {
	Status          MemoryStatus
	Title           *string
	Content         *KnowledgeContent
	ContentType     KnowledgeContentType
	Importance      *int16
	Confidence      *float64
	AuthorType      MemoryAuthorType
	EpistemicStatus MemoryEpistemicStatus
	EntityKey       *string
	DedupeKey       *string
	ConflictState   MemoryConflictState
	ValidFrom       *int64
	ValidTo         *int64
	LastAccessedAt  *int64
	AccessCount     *int64
}

type GetMemoryOptions struct {
	ID           string
	IDs          []string
	KnowledgeID  string
	KnowledgeIDs []string
	SpaceID      string
	UserID       string
	MemoryTypes  []MemoryType
	Scopes       []MemoryScope
	Statuses     []MemoryStatus
	EntityKeys   []string
	DedupeKey    string
	SourceKind   MemorySourceKind
	SourceRef    string
	ContextType  MemoryContextType
	ContextID    string
	ConflictOnly bool

	AccessibleUserID         string
	AccessibleSpaceID        string
	IncludeGlobalUserMemory  bool
	IncludeSpaceUserMemory   bool
	IncludeSpaceSharedMemory bool
}

func (opts GetMemoryOptions) Apply(query *sq.SelectBuilder) {
	if opts.ID != "" {
		*query = query.Where(sq.Eq{"id": opts.ID})
	}
	if len(opts.IDs) > 0 {
		*query = query.Where(sq.Eq{"id": opts.IDs})
	}
	if opts.KnowledgeID != "" {
		*query = query.Where(sq.Eq{"knowledge_id": opts.KnowledgeID})
	}
	if len(opts.KnowledgeIDs) > 0 {
		*query = query.Where(sq.Eq{"knowledge_id": opts.KnowledgeIDs})
	}
	if opts.SpaceID != "" {
		*query = query.Where(sq.Eq{"space_id": opts.SpaceID})
	}
	if opts.UserID != "" {
		*query = query.Where(sq.Eq{"user_id": opts.UserID})
	}
	if opts.AccessibleUserID != "" {
		accessScope := sq.Or{}
		if opts.IncludeGlobalUserMemory {
			accessScope = append(accessScope, sq.And{
				sq.Eq{"space_id": GLOBAL_MEMORY_SPACE_ID},
				sq.Eq{"scope": MEMORY_SCOPE_USER},
				sq.Eq{"user_id": opts.AccessibleUserID},
			})
		}
		if opts.AccessibleSpaceID != "" && opts.IncludeSpaceUserMemory {
			accessScope = append(accessScope, sq.And{
				sq.Eq{"space_id": opts.AccessibleSpaceID},
				sq.Eq{"scope": MEMORY_SCOPE_USER},
				sq.Eq{"user_id": opts.AccessibleUserID},
			})
		}
		if opts.AccessibleSpaceID != "" && opts.IncludeSpaceSharedMemory {
			accessScope = append(accessScope, sq.And{
				sq.Eq{"space_id": opts.AccessibleSpaceID},
				sq.Eq{"scope": MEMORY_SCOPE_SPACE},
			})
		}
		if len(accessScope) > 0 {
			*query = query.Where(accessScope)
		}
	}
	if len(opts.MemoryTypes) > 0 {
		*query = query.Where(sq.Eq{"memory_type": opts.MemoryTypes})
	}
	if len(opts.Scopes) > 0 {
		*query = query.Where(sq.Eq{"scope": opts.Scopes})
	}
	if len(opts.Statuses) > 0 {
		*query = query.Where(sq.Eq{"status": opts.Statuses})
	}
	if len(opts.EntityKeys) > 0 {
		*query = query.Where(sq.Eq{"entity_key": opts.EntityKeys})
	}
	if opts.DedupeKey != "" {
		*query = query.Where(sq.Eq{"dedupe_key": opts.DedupeKey})
	}
	if opts.SourceKind != "" {
		*query = query.Where(sq.Eq{"source_kind": opts.SourceKind})
	}
	if opts.SourceRef != "" {
		*query = query.Where(sq.Eq{"source_ref": opts.SourceRef})
	}
	if opts.ConflictOnly {
		*query = query.Where(sq.NotEq{"conflict_state": MEMORY_CONFLICT_NONE})
	}
}

type MemoryEdge struct {
	ID           string         `json:"id" db:"id"`
	SpaceID      string         `json:"space_id" db:"space_id"`
	FromMemoryID string         `json:"from_memory_id" db:"from_memory_id"`
	ToMemoryID   string         `json:"to_memory_id" db:"to_memory_id"`
	Relation     MemoryRelation `json:"relation" db:"relation"`
	Weight       float64        `json:"weight" db:"weight"`
	CreatedAt    int64          `json:"created_at" db:"created_at"`
}

type GetMemoryEdgeOptions struct {
	ID            string
	SpaceID       string
	FromMemoryID  string
	FromMemoryIDs []string
	ToMemoryID    string
	ToMemoryIDs   []string
	Relation      MemoryRelation
}

func (opts GetMemoryEdgeOptions) Apply(query *sq.SelectBuilder) {
	if opts.ID != "" {
		*query = query.Where(sq.Eq{"id": opts.ID})
	}
	if opts.SpaceID != "" {
		*query = query.Where(sq.Eq{"space_id": opts.SpaceID})
	}
	if opts.FromMemoryID != "" {
		*query = query.Where(sq.Eq{"from_memory_id": opts.FromMemoryID})
	}
	if len(opts.FromMemoryIDs) > 0 {
		*query = query.Where(sq.Eq{"from_memory_id": opts.FromMemoryIDs})
	}
	if opts.ToMemoryID != "" {
		*query = query.Where(sq.Eq{"to_memory_id": opts.ToMemoryID})
	}
	if len(opts.ToMemoryIDs) > 0 {
		*query = query.Where(sq.Eq{"to_memory_id": opts.ToMemoryIDs})
	}
	if opts.Relation != "" {
		*query = query.Where(sq.Eq{"relation": opts.Relation})
	}
}

type MemoryBinding struct {
	ID          string            `json:"id" db:"id"`
	SpaceID     string            `json:"space_id" db:"space_id"`
	UserID      string            `json:"user_id" db:"user_id"`
	MemoryID    string            `json:"memory_id" db:"memory_id"`
	ContextType MemoryContextType `json:"context_type" db:"context_type"`
	ContextID   string            `json:"context_id" db:"context_id"`
	BindingType MemoryBindingType `json:"binding_type" db:"binding_type"`
	PinnedBy    MemoryPinnedBy    `json:"pinned_by" db:"pinned_by"`
	CreatedAt   int64             `json:"created_at" db:"created_at"`
	UpdatedAt   int64             `json:"updated_at" db:"updated_at"`
}

type RuntimeContext struct {
	Type       MemoryContextType         `json:"type"`
	ID         string                    `json:"id"`
	Extraction *RuntimeContextExtraction `json:"extraction,omitempty"`
}

type RuntimeContextExtraction struct {
	Summary  string                     `json:"summary,omitempty"`
	Messages []RuntimeContextMessage    `json:"messages,omitempty"`
	Metadata map[string]json.RawMessage `json:"metadata,omitempty"`
}

type RuntimeContextMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	Name       string          `json:"name,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Reasoning  string          `json:"reasoning,omitempty"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
}

func (e *RuntimeContextExtraction) ReflectionContent() string {
	if e == nil {
		return ""
	}
	if summary := strings.TrimSpace(e.Summary); summary != "" {
		return summary
	}

	parts := make([]string, 0, len(e.Messages))
	for _, msg := range e.Messages {
		role := strings.TrimSpace(msg.Role)
		content := strings.TrimSpace(msg.Content)
		switch {
		case role != "" && content != "":
			parts = append(parts, fmt.Sprintf("%s: %s", role, content))
		case content != "":
			parts = append(parts, content)
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

type GetMemoryBindingOptions struct {
	ID          string
	SpaceID     string
	UserID      string
	MemoryID    string
	ContextType MemoryContextType
	ContextID   string
	BindingType MemoryBindingType
}

func (opts GetMemoryBindingOptions) Apply(query *sq.SelectBuilder) {
	if opts.ID != "" {
		*query = query.Where(sq.Eq{"id": opts.ID})
	}
	if opts.SpaceID != "" {
		*query = query.Where(sq.Eq{"space_id": opts.SpaceID})
	}
	if opts.UserID != "" {
		*query = query.Where(sq.Eq{"user_id": opts.UserID})
	}
	if opts.MemoryID != "" {
		*query = query.Where(sq.Eq{"memory_id": opts.MemoryID})
	}
	if opts.ContextType != "" {
		*query = query.Where(sq.Eq{"context_type": opts.ContextType})
	}
	if opts.ContextID != "" {
		*query = query.Where(sq.Eq{"context_id": opts.ContextID})
	}
	if opts.BindingType != "" {
		*query = query.Where(sq.Eq{"binding_type": opts.BindingType})
	}
}

func (t MemoryType) String() string            { return string(t) }
func (s MemoryScope) String() string           { return string(s) }
func (l MemoryLayer) String() string           { return string(l) }
func (s MemoryStatus) String() string          { return string(s) }
func (a MemoryAuthorType) String() string      { return string(a) }
func (e MemoryEpistemicStatus) String() string { return string(e) }
func (c MemoryConflictState) String() string   { return string(c) }
func (s MemorySourceKind) String() string      { return string(s) }
func (r MemoryRelation) String() string        { return string(r) }
func (c MemoryContextType) String() string     { return string(c) }
func (b MemoryBindingType) String() string     { return string(b) }
func (p MemoryPinnedBy) String() string        { return string(p) }
func (m Memory) String() string                { return fmt.Sprintf("%s:%s", m.ID, m.KnowledgeID) }
func (m MemoryBinding) String() string         { return fmt.Sprintf("%s:%s", m.ContextType, m.ContextID) }
