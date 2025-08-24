package types

// KnowledgeMeta 知识元信息表结构
type KnowledgeMeta struct {
	ID        string `json:"id" db:"id"` // 元数据ID，唯一标识每条记录
	SpaceID   string `json:"space_id" db:"space_id"`
	MetaInfo  string `json:"meta_info" db:"meta_info"`   // 元数据信息，存储关于知识的元数据内容
	CreatedAt int64  `json:"created_at" db:"created_at"` // 记录创建时间，Unix时间戳
}

// MergeDataQuery 用于合并数据查询的结构
type MergeDataQuery struct {
	MetaID   string
	ChunkIDs []int
}

// RelMetaWithKnowledge 关联元数据与知识内容的结构
type RelMetaWithKnowledge struct {
	MetaID      string               `json:"meta_id" db:"meta_id"`
	ChunkIndex  int                  `json:"chunk_index" db:"chunk_index"`
	Content     KnowledgeContent     `json:"content" db:"content"`
	ContentType KnowledgeContentType `json:"content_type" db:"content_type"`
}

// KnowledgeRelMeta 知识关联元数据表结构
// 用于保存长文 chunk 的 meta 信息
type KnowledgeRelMeta struct {
	SpaceID     string `json:"space_id" db:"space_id"`
	KnowledgeID string `json:"knowledge_id" db:"knowledge_id"` // 长文的唯一标识，关联 metadata 表的主键
	MetaID      string `json:"meta_id" db:"meta_id"`           // 关联的 meta 的主键
	ChunkIndex  int    `json:"chunk_index" db:"chunk_index"`   // chunk 的顺序编号，默认从1开始
	CreatedAt   int64  `json:"created_at" db:"created_at"`     // 创建时间，使用 UNIX 时间戳表示
}
