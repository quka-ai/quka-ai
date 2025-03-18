package types

type ChatSession struct {
	ID               string            `json:"id" db:"id"`
	SpaceID          string            `json:"space_id" db:"space_id"`
	UserID           string            `json:"user_id" db:"user_id"`
	Title            string            `json:"title" db:"title"`
	Type             ChatSessionType   `json:"session_type" db:"session_type"`
	Status           ChatSessionStatus `json:"status" db:"status"`
	CreatedAt        int64             `json:"created_at" db:"created_at"`
	LatestAccessTime int64             `json:"latest_access_time" db:"latest_access_time"`
}

type ChatSessionType int8
type ChatSessionStatus int8

const (
	CHAT_SESSION_TYPE_SINGLE ChatSessionType = 1
	CHAT_SESSION_TYPE_MANY   ChatSessionType = 2

	CHAT_SESSION_STATUS_OFFICIAL   ChatSessionStatus = 1
	CHAT_SESSION_STATUS_UNOFFICIAL ChatSessionStatus = 2
)
