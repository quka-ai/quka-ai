package types

import (
	"github.com/lib/pq"
)

type ChatMessageExt struct {
	MessageID        string               `db:"message_id"`
	SessionID        string               `db:"session_id"`
	SpaceID          string               `db:"space_id"`
	Evaluate         EvaluateType         `db:"evaluate"`
	GenerationStatus GenerationStatusType `db:"generation_status"`
	RelDocs          pq.StringArray       `db:"rel_docs"` // relevance docs
	CreatedAt        int64                `db:"created_at"`
	UpdatedAt        int64                `db:"updated_at"`
}
