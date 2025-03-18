package types

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/pgvector/pgvector-go"
)

type Vector struct {
	ID             string          `json:"id" db:"id"`                           // 主键，关联knowledge_chunk_id
	KnowledgeID    string          `json:"knowledge_id" db:"knowledge_id"`       // 关联 knowledge_id
	SpaceID        string          `json:"space_id" db:"space_id"`               // 空间ID，用于标识所属空间
	Resource       string          `json:"resource" db:"resource"`               // 关联 knowledge resource
	UserID         string          `json:"user_id" db:"user_id"`                 // 用户ID，用于标识向量所属用户
	Embedding      pgvector.Vector `json:"embedding" db:"embedding"`             // 文本向量，存储经过编码后的文本向量表示
	OriginalLength int             `json:"original_length" db:"original_length"` // 原文长度
	CreatedAt      int64           `json:"created_at" db:"created_at"`           // 创建时间，UNIX时间戳
	UpdatedAt      int64           `json:"updated_at" db:"updated_at"`           // 更新时间，UNIX时间戳
}

type QueryResult struct {
	ID             string  `json:"id" db:"id"`
	KnowledgeID    string  `json:"knowledge_id" db:"knowledge_id"`
	Cos            float32 `json:"cos" db:"cos"`
	OriginalLength int     `json:"original_length" db:"original_length"`
}

type GetVectorsOptions struct {
	ID          string
	SpaceID     string
	UserID      string
	KnowledgeID string
	Resource    *ResourceQuery
}

func (opts GetVectorsOptions) Apply(query *sq.SelectBuilder) {
	if opts.ID != "" {
		*query = query.Where(sq.Eq{"id": opts.ID})
	}
	if opts.KnowledgeID != "" {
		*query = query.Where(sq.Eq{"knowledge_id": opts.ID})
	}
	if opts.SpaceID != "" {
		*query = query.Where(sq.Eq{"space_id": opts.SpaceID})
	}
	if opts.UserID != "" {
		*query = query.Where(sq.Eq{"user_id": opts.UserID})
	}
	if opts.Resource != nil {
		*query = query.Where(opts.Resource.ToQuery())
	}
}
