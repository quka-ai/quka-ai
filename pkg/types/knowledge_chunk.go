package types

// KnowledgeChunk 表的结构体
type KnowledgeChunk struct {
	ID             string `json:"id" db:"id"`                           // 主键，字符串类型
	KnowledgeID    string `json:"knowledge_id" db:"knowledge_id"`       // 知识点ID
	SpaceID        string `json:"space_id" db:"space_id"`               // 空间ID
	UserID         string `json:"user_id" db:"user_id"`                 // 用户ID
	Chunk          string `json:"chunk" db:"chunk"`                     // 知识片段
	OriginalLength int    `json:"original_length" db:"original_length"` // 原文长度
	UpdatedAt      int64  `json:"updated_at" db:"updated_at"`           // 更新时间
	CreatedAt      int64  `json:"created_at" db:"created_at"`           // 创建时间
}
