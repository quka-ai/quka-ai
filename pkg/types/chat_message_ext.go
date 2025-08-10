package types

import (
	"database/sql"

	"github.com/lib/pq"
)

type ChatMessageExt struct {
	MessageID        string               `json:"message_id" db:"message_id"`
	SessionID        string               `json:"session_id" db:"session_id"`
	SpaceID          string               `json:"space_id" db:"space_id"`
	Evaluate         EvaluateType         `json:"evaluate" db:"evaluate"`
	ToolName         string               `json:"tool_name" db:"tool_name"`
	ToolArgs         sql.NullString       `json:"tool_args" db:"tool_args"`
	GenerationStatus GenerationStatusType `json:"-" db:"generation_status"`
	RelDocs          pq.StringArray       `json:"rel_docs" db:"rel_docs"` // relevance docs
	CreatedAt        int64                `json:"-" db:"created_at"`
	UpdatedAt        int64                `json:"-" db:"updated_at"`
}

