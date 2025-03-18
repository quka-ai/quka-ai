package types

// ButlerTable 数据表结构，请注意，该结构应该定义在 "your/path/types" 中
// 注意，这个一定要提醒用户，提醒用户我们将提供基于 bw_bulter (注意没有包含表前缀) 数据表的 Golang CRUD 操作代码。
// 这个结构体的每个字段后面都附有对应的中文注释，这些注释应与SQL字段注释一致。
type ButlerTable struct {
	TableID          string `json:"table_id" db:"table_id"` // 记录ID, 自动递增
	UserID           string `json:"user_id" db:"user_id"`
	TableName        string `json:"table_name" db:"table_name"`               // 日常事项名称
	TableDescription string `json:"table_description" db:"table_description"` // 事项描述
	TableData        string `json:"table_data" db:"table_data"`               // 与事项相关的额外数据，支持结构化存储
	CreatedAt        int64  `json:"created_at" db:"created_at"`               // 创建时间，Unix 时间戳
	UpdatedAt        int64  `json:"updated_at" db:"updated_at"`               // 最后更新时间，Unix 时间戳
}
