package types

type FixedPin struct {
	ID          string               `json:"id" db:"id"`
	SpaceID     string               `json:"space_id" db:"space_id"`
	UserID      string               `json:"user_id" db:"user_id"`
	Content     KnowledgeContent     `json:"content" db:"content"`
	ContentType KnowledgeContentType `json:"content_type" db:"content_type"`
	CreatedAt   int64                `json:"created_at" db:"created_at"`
	UpdatedAt   int64                `json:"updated_at" db:"updated_at"`
}
