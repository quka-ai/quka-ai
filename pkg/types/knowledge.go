package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

const (
	DEFAULT_RESOURCE = "knowledge"
)

// export const cards = pgTable('cards', {
//   id: uuid('id').primaryKey().notNull().defaultRandom(),
//   spaceID: uuid('spaceID')
//     .notNull()
//     .references(() => spaces.id),
//   kind: cardKindEnum('kind').notNull(),
//   // tags is a string slice
//   tags: jsonb('tags').default([]),
//   content: text('content'),
//   authorID: varchar('authorID', { length: 256 }).notNull(),
//   createdAt: timestamp('createdAt', {
//     mode: 'string',
//     withTimezone: true,
//   }).defaultNow(),
//   updatedAt: timestamp('updatedAt', {
//     mode: 'string',
//     withTimezone: true,
//   }).defaultNow(),
// })

type KnowledgeKind string

const (
	KNOWLEDGE_KIND_TEXT    KnowledgeKind = "text"
	KNOWLEDGE_KIND_IMAGE                 = "image"
	KNOWLEDGE_KIND_VIDEO                 = "video"
	KNOWLEDGE_KIND_URL                   = "url"
	KNOWLEDGE_KIND_CHUNK                 = "chunk"
	KNOWLEDGE_KIND_UNKNOWN               = "unknown"
)

func KindNewFromString(s string) KnowledgeKind {
	switch strings.ToLower(s) {
	case string(KNOWLEDGE_KIND_TEXT):
		return KNOWLEDGE_KIND_TEXT
	case string(KNOWLEDGE_KIND_IMAGE):
		return KNOWLEDGE_KIND_IMAGE
	case string(KNOWLEDGE_KIND_VIDEO):
		return KNOWLEDGE_KIND_VIDEO
	default:
		return KNOWLEDGE_KIND_UNKNOWN
	}
}

func (k KnowledgeKind) String() string {
	return string(k)
}

type KnowledgeStage int8

const (
	KNOWLEDGE_STAGE_NONE      KnowledgeStage = 0
	KNOWLEDGE_STAGE_SUMMARIZE KnowledgeStage = 1
	KNOWLEDGE_STAGE_EMBEDDING KnowledgeStage = 2
	KNOWLEDGE_STAGE_DONE      KnowledgeStage = 3
)

var namesForKnowledgeStage = map[KnowledgeStage]string{
	KNOWLEDGE_STAGE_NONE:      "None",
	KNOWLEDGE_STAGE_SUMMARIZE: "Summarize",
	KNOWLEDGE_STAGE_EMBEDDING: "Embedding",
	KNOWLEDGE_STAGE_DONE:      "Done",
}

func (v KnowledgeStage) String() string {
	if n, ok := namesForKnowledgeStage[v]; ok {
		return n
	}
	return fmt.Sprintf("KnowledgeStage(%d)", v)
}

func (v KnowledgeStage) int8() int8 {
	return int8(v)
}

type KnowledgeLite struct {
	ID       string         `json:"id" db:"id"`
	SpaceID  string         `json:"space_id" db:"space_id"`
	Resource string         `json:"resource" db:"resource"`
	Title    string         `json:"title" db:"title"`
	Tags     pq.StringArray `json:"tags" db:"tags"`
	UserID   string         `json:"user_id" db:"user_id"`
}

type KnowledgeResponse struct {
	ID          string               `json:"id" db:"id"`
	SpaceID     string               `json:"space_id" db:"space_id"`
	Kind        KnowledgeKind        `json:"kind" db:"kind"`
	Resource    string               `json:"resource" db:"resource"`
	Title       string               `json:"title" db:"title"`
	Tags        pq.StringArray       `json:"tags" db:"tags"`
	Content     string               `json:"content" db:"content"`
	Blocks      json.RawMessage      `json:"blocks,omitempty" db:"-"`
	ContentType KnowledgeContentType `json:"content_type" db:"content_type"`
	UserID      string               `json:"user_id" db:"user_id"`
	Stage       KnowledgeStage       `json:"stage" db:"stage"`
	CreatedAt   int64                `json:"created_at" db:"created_at"`
	UpdatedAt   int64                `json:"updated_at" db:"updated_at"`
	ExpiredAt   int64                `json:"expired_at" db:"expired_at"`
	IsExpired   bool                 `json:"is_expired,omitempty" db:"-"`
}

type Knowledge struct {
	ID          string               `json:"id" db:"id"`
	SpaceID     string               `json:"space_id" db:"space_id"`
	Kind        KnowledgeKind        `json:"kind" db:"kind"`
	Resource    string               `json:"resource" db:"resource"`
	Title       string               `json:"title" db:"title"`
	Tags        pq.StringArray       `json:"tags" db:"tags"`
	Content     KnowledgeContent     `json:"content" db:"content"`
	ContentType KnowledgeContentType `json:"content_type" db:"content_type"`
	UserID      string               `json:"user_id" db:"user_id"`
	Summary     string               `json:"summary" db:"summary"`
	MaybeDate   string               `json:"maybe_date" db:"maybe_date"`
	Stage       KnowledgeStage       `json:"stage" db:"stage"`
	CreatedAt   int64                `json:"created_at" db:"created_at"`
	UpdatedAt   int64                `json:"updated_at" db:"updated_at"`
	RetryTimes  int                  `json:"retry_times" db:"retry_times"`
	ExpiredAt   int64                `json:"expired_at" db:"expired_at"`
}

type RawMessage = KnowledgeContent

// StringArray represents a one-dimensional array of the PostgreSQL character types.
type KnowledgeContent json.RawMessage

func (m KnowledgeContent) String() string {
	var str string
	// 尝试解析为字符串
	if err := json.Unmarshal(m, &str); err == nil {
		// 如果成功，说明内容是一个 JSON 字符串
		return str
	}
	return string(m)
}

func (m KnowledgeContent) MarshalJSON() ([]byte, error) {
	// 自定义 Timestamp 字段的格式
	if m == nil {
		return []byte("\"\""), nil
	}
	return m, nil
}

func (m *KnowledgeContent) UnmarshalJSON(data []byte) error {
	*m = data
	return nil
}

// Scan implements the sql.Scanner interface.
func (a *KnowledgeContent) Scan(src interface{}) error {
	switch src := src.(type) {
	case []byte:
		return a.scanBytes(src)
	case string:
		return a.scanBytes([]byte(src))
	case nil:
		return nil
	}

	return fmt.Errorf("pq: cannot convert %T to json.RawMessage", src)
}

func (a *KnowledgeContent) scanBytes(src []byte) error {
	*a = KnowledgeContent(src)
	return nil
}

type KnowledgeContentType string

const (
	KNOWLEDGE_CONTENT_TYPE_MARKDOWN KnowledgeContentType = "markdown"
	KNOWLEDGE_CONTENT_TYPE_HTML     KnowledgeContentType = "html"
	KNOWLEDGE_CONTENT_TYPE_BLOCKS   KnowledgeContentType = "blocks"
	KNOWLEDGE_CONTENT_TYPE_UNKNOWN  KnowledgeContentType = "unknown"
)

func StringToKnowledgeContentType(str string) KnowledgeContentType {
	switch strings.ToLower(str) {
	case string(KNOWLEDGE_CONTENT_TYPE_BLOCKS):
		return KNOWLEDGE_CONTENT_TYPE_BLOCKS
	case string(KNOWLEDGE_CONTENT_TYPE_MARKDOWN):
		return KNOWLEDGE_CONTENT_TYPE_MARKDOWN
	case string(KNOWLEDGE_CONTENT_TYPE_HTML):
		return KNOWLEDGE_CONTENT_TYPE_HTML
	default:
		return KNOWLEDGE_CONTENT_TYPE_UNKNOWN
	}
}

type GetKnowledgeOptions struct {
	ID              string
	IDs             []string
	Kind            []KnowledgeKind
	ExcludeKind     []KnowledgeKind
	SpaceID         string
	UserID          string
	Resource        *ResourceQuery
	Stage           KnowledgeStage
	RetryTimes      int
	Keywords        string
	TimeRange       *struct {
		St int64
		Et int64
	}
	IncludeExpired  bool   // 是否包含过期内容，默认false
	ExpiredOnly     bool   // 只返回过期内容
}

func (opts GetKnowledgeOptions) Apply(query *sq.SelectBuilder) {
	if opts.ID != "" {
		*query = query.Where(sq.Eq{"id": opts.ID})
	} else if len(opts.IDs) > 0 {
		*query = query.Where(sq.Eq{"id": opts.IDs})
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
	if len(opts.Kind) > 0 {
		*query = query.Where(sq.Eq{"kind": opts.Kind})
	}
	if len(opts.ExcludeKind) > 0 {
		*query = query.Where(sq.NotEq{"kind": opts.ExcludeKind})
	}
	if opts.Stage > 0 {
		*query = query.Where(sq.Eq{"stage": opts.Stage})
	}
	if opts.RetryTimes > 0 {
		*query = query.Where(sq.Eq{"retry_times": opts.RetryTimes})
	}

	if opts.Keywords != "" {
		or := sq.Or{}
		if len(opts.Keywords) == 32 {
			or = append(or, sq.Eq{"id": opts.Keywords})
		}
		*query = query.Where(append(or, sq.Like{"title": fmt.Sprintf("%%%s%%", opts.Keywords)}))
	}
	if opts.TimeRange != nil {
		*query = query.Where(sq.And{sq.GtOrEq{"created_at": opts.TimeRange.St}, sq.LtOrEq{"created_at": opts.TimeRange.Et}})
	}
	
	// 过期检查逻辑（预计算方案，默认排除过期内容）
	now := GetCurrentTimestamp()
	if opts.ExpiredOnly {
		// 只返回过期内容：WHERE expired_at > 0 AND expired_at <= NOW()
		*query = query.Where(sq.And{
			sq.Gt{"expired_at": 0},
			sq.LtOrEq{"expired_at": now},
		})
	} else if !opts.IncludeExpired {
		// 默认排除过期内容：WHERE (expired_at = 0 OR expired_at > NOW())
		*query = query.Where(sq.Or{
			sq.Eq{"expired_at": 0},
			sq.Gt{"expired_at": now},
		})
	}
	// 如果 IncludeExpired=true 且 ExpiredOnly=false，则不添加过期条件，返回所有内容
}

type ResourceQuery struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

func (r *ResourceQuery) ToQuery() sq.Sqlizer {
	if len(r.Include) > 0 {
		return sq.Eq{"resource": r.Include}
	}
	return sq.NotEq{"resource": r.Exclude}
}

type UpdateKnowledgeArgs struct {
	Title       string
	Resource    string
	Kind        KnowledgeKind
	Content     KnowledgeContent
	ContentType KnowledgeContentType
	Tags        []string
	Stage       KnowledgeStage
	Summary     string
}

// 过期相关工具函数

// CalculateExpiredAt 根据资源周期计算过期时间
func CalculateExpiredAt(createdAt int64, cycle int) int64 {
	if cycle <= 0 {
		return 0 // 永不过期
	}
	return createdAt + int64(cycle*24*3600)
}

// IsExpired 检查知识是否过期
func (k *Knowledge) IsExpired() bool {
	if k.ExpiredAt == 0 {
		return false // 永不过期
	}
	return k.ExpiredAt <= GetCurrentTimestamp()
}

// SetExpiredAt 设置过期时间
func (k *Knowledge) SetExpiredAt(cycle int) {
	k.ExpiredAt = CalculateExpiredAt(k.CreatedAt, cycle)
}

// GetCurrentTimestamp 获取当前时间戳（便于测试时mock）
var GetCurrentTimestamp = func() int64 {
	return time.Now().Unix()
}
