package types

type ChatSummary struct {
	ID        string `db:"id" json:"id"`
	SpaceID   string `db:"space_id" json:"space_id"`
	MessageID string `db:"message_id" json:"message_id"`
	SessionID string `db:"session_id" json:"session_id"`
	Content   string `db:"content" json:"content"`
	CreatedAt int64  `db:"created_at" json:"created_at"`
}
