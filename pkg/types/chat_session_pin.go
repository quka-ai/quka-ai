package types

// ChatSessionPin 表结构体定义，请在 types 包中定义
type ChatSessionPin struct {
	SessionID string     `json:"session_id" db:"session_id"` // 唯一标识一个会话
	SpaceID   string     `json:"space_id" db:"space_id"`     // 所属空间的标识
	UserID    string     `json:"user_id" db:"user_id"`       // 用户的唯一标识
	Content   RawMessage `json:"content" db:"content"`       // 与会话关联的内容（JSON格式）
	Version   string     `json:"version" db:"version"`       // JSON内容格式版本号，向前兼容
	CreatedAt int64      `json:"created_at" db:"created_at"` // 记录的创建时间
	UpdatedAt int64      `json:"updated_at" db:"updated_at"` // 记录的更新时间
}

type ContentPinV1 struct {
	Knowledges []string `json:"knowledges"`
	Journals   []string `json:"journals"`
}

const CHAT_SESSION_PIN_VERSION_V1 = "v1"
