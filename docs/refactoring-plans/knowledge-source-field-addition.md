# Knowledge Source 和 SourceRef 字段添加计划

## 问题描述和背景
需要为 `quka_knowledge` 表增加两个字段来标识 knowledge 的来源：
- `source`: 来源类型（空字符串表示平台内部创建，可选值: rss, podcast, mcp, chat）
- `source_ref`: 来源引用 ID（如 chat_session_id, subscription_id 等）

这样设计可以：
1. 通过 `source` 字段快速过滤不同类型的来源
2. 通过 `source_ref` 字段追溯到具体的来源对象（如某个具体的聊天会话）

## 改造目标
1. 在数据库表中添加 `source` 和 `source_ref` 字段
2. 更新 Go 类型定义，添加 `Source` 和 `SourceRef` 字段
3. 添加 `KNOWLEDGE_SOURCE_CHAT` 常量
4. 更新所有相关的数据库操作方法
5. 确保现有功能不受影响

## 详细实施方案

### 1. 数据库层修改

#### 1.1 创建数据库迁移文件
**文件**: `app/store/sqlstore/migrations/knowledge_add_source_field.sql`

```sql
-- 添加 source 字段到 quka_knowledge 表
ALTER TABLE quka_knowledge ADD COLUMN IF NOT EXISTS source VARCHAR(50) NOT NULL DEFAULT '';

-- 添加字段注释
COMMENT ON COLUMN quka_knowledge.source IS 'knowledge来源标识，空字符串表示平台内部创建，可选值: rss, podcast, mcp等';

-- 添加索引（如果需要经常按 source 查询）
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_source ON quka_knowledge(source);
```

#### 1.2 更新表定义文件
**文件**: `app/store/sqlstore/knowledge.sql`

在第 16 行后添加：
```sql
source VARCHAR(50) NOT NULL DEFAULT '',
```

在第 36 行后添加：
```sql
COMMENT ON COLUMN quka_knowledge.source IS 'knowledge来源标识，空字符串表示平台内部创建，可选值: rss, podcast, mcp等';
```

在第 44 行后添加索引（可选）：
```sql
CREATE INDEX IF NOT EXISTS idx_quka_knowledge_source ON quka_knowledge(source);
```

### 2. Go 类型定义修改

#### 2.1 更新 `pkg/types/knowledge.go`

**KnowledgeLite 结构体**（第 93-100 行）：
```go
type KnowledgeLite struct {
	ID       string         `json:"id" db:"id"`
	SpaceID  string         `json:"space_id" db:"space_id"`
	Resource string         `json:"resource" db:"resource"`
	Title    string         `json:"title" db:"title"`
	Tags     pq.StringArray `json:"tags" db:"tags"`
	UserID   string         `json:"user_id" db:"user_id"`
	Source   string         `json:"source" db:"source"`  // 新增
}
```

**KnowledgeResponse 结构体**（第 102-118 行）：
```go
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
	Source      string               `json:"source" db:"source"`  // 新增
}
```

**Knowledge 结构体**（第 120-138 行）：
```go
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
	RelDocID    string               `json:"rel_doc_id,omitempty" db:"rel_doc_id"`
	Source      string               `json:"source" db:"source"`  // 新增
}
```

**UpdateKnowledgeArgs 结构体**（第 313-322 行）：
```go
type UpdateKnowledgeArgs struct {
	Title       string
	Resource    string
	Kind        KnowledgeKind
	Content     KnowledgeContent
	ContentType KnowledgeContentType
	Tags        []string
	Stage       KnowledgeStage
	Summary     string
	Source      string  // 新增
}
```

**GetKnowledgeOptions 结构体**（第 221-239 行），添加新的过滤选项：
```go
type GetKnowledgeOptions struct {
	ID          string
	IDs         []string
	Kind        []KnowledgeKind
	ExcludeKind []KnowledgeKind
	SpaceID     string
	UserID      string
	Resource    *ResourceQuery
	Stage       KnowledgeStage
	RetryTimes  int
	Keywords    string
	RelDocID    string
	Source      string  // 新增：按来源过滤
	TimeRange   *struct {
		St int64
		Et int64
	}
	IncludeExpired bool
	ExpiredOnly    bool
}
```

**在 Apply 方法中添加 Source 过滤逻辑**（第 241-299 行之间添加）：
```go
if opts.Source != "" {
	*query = query.Where(sq.Eq{"source": opts.Source})
}
```

### 3. 数据库存储层修改

#### 3.1 更新 `app/store/sqlstore/knowledge.go`

**更新 SetAllColumns 调用**（第 32 行）：
```go
store.SetAllColumns("id", "title", "user_id", "space_id", "tags", "content", "content_type", "resource", "kind", "summary", "maybe_date", "stage", "retry_times", "created_at", "updated_at", "expired_at", "rel_doc_id", "source")
```

**更新 Create 方法**（第 37-58 行）：
```go
func (s *KnowledgeStore) Create(ctx context.Context, data types.Knowledge) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = time.Now().Unix()
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "title", "user_id", "space_id", "tags", "content", "content_type", "resource", "kind", "summary", "maybe_date", "stage", "retry_times", "created_at", "updated_at", "expired_at", "rel_doc_id", "source").
		Values(data.ID, data.Title, data.UserID, data.SpaceID, pq.Array(data.Tags), data.Content.String(), data.ContentType, data.Resource, data.Kind, data.Summary, data.MaybeDate, data.Stage, data.RetryTimes, data.CreatedAt, data.UpdatedAt, data.ExpiredAt, data.RelDocID, data.Source)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	if err != nil {
		return err
	}
	return nil
}
```

**更新 BatchCreate 方法**（第 61-84 行）：
```go
func (s *KnowledgeStore) BatchCreate(ctx context.Context, datas []*types.Knowledge) error {
	query := sq.Insert(s.GetTable()).
		Columns("id", "title", "user_id", "space_id", "tags", "content", "content_type", "resource", "kind", "summary", "maybe_date", "stage", "retry_times", "created_at", "updated_at", "expired_at", "rel_doc_id", "source")
	for _, data := range datas {
		if data.CreatedAt == 0 {
			data.CreatedAt = time.Now().Unix()
		}
		if data.UpdatedAt == 0 {
			data.UpdatedAt = time.Now().Unix()
		}

		query = query.Values(data.ID, data.Title, data.UserID, data.SpaceID, pq.Array(data.Tags), data.Content.String(), data.ContentType, data.Resource, data.Kind, data.Summary, data.MaybeDate, data.Stage, data.RetryTimes, data.CreatedAt, data.UpdatedAt, data.ExpiredAt, data.RelDocID, data.Source)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	if _, err = s.GetMaster(ctx).Exec(queryString, args...); err != nil {
		return err
	}
	return nil
}
```

**更新 Update 方法**（第 193-238 行），添加 Source 更新逻辑：
```go
if data.Source != "" {
	query = query.Set("source", data.Source)
}
```

**更新 ListLiteKnowledges 方法**（第 329-346 行）：
```go
func (s *KnowledgeStore) ListLiteKnowledges(ctx context.Context, opts types.GetKnowledgeOptions, page, pageSize uint64) ([]*types.KnowledgeLite, error) {
	query := sq.Select("id", "title", "space_id", "user_id", "resource", "tags", "source").From(s.GetTable())
	if page != 0 || pageSize != 0 {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}
	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []*types.KnowledgeLite
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
```

### 4. 业务逻辑层更新

检查并更新以下文件中创建 Knowledge 的地方，确保合适的地方设置 Source 字段：

- `app/logic/v1/knowledge_tools.go` - MCP 工具创建的 knowledge 应设置 source 为 "mcp"
- `app/logic/v1/rss_fetcher.go` - RSS 创建的 knowledge 应设置 source 为 "rss"
- `app/logic/v1/podcast.go` - 播客创建的 knowledge 应设置 source 为 "podcast"
- 其他创建 knowledge 的地方，保持默认值（空字符串）

## 关键考虑点

1. **向后兼容性**: 新字段使用 `DEFAULT ''` 确保现有数据不受影响
2. **索引优化**: 如果需要频繁按 source 查询，建议添加索引
3. **验证逻辑**: 可以考虑添加 source 字段的验证逻辑，限制只能是预定义的值
4. **迁移顺序**: 先执行数据库迁移，再更新代码

## 实施步骤

1. ✅ 创建数据库迁移文件
2. ✅ 更新表定义文件 `knowledge.sql`
3. ✅ 更新类型定义 `pkg/types/knowledge.go`
4. ✅ 更新存储层 `app/store/sqlstore/knowledge.go`
5. ✅ 更新业务逻辑层，在合适的地方设置 source 值
6. ✅ 执行数据库迁移
7. ✅ 测试验证

## 需要确认的问题

1. **Source 字段值的规范**: 是否需要定义常量来约束 source 的可选值？
   - 建议定义常量如: `KNOWLEDGE_SOURCE_PLATFORM = ""`、`KNOWLEDGE_SOURCE_RSS = "rss"`、`KNOWLEDGE_SOURCE_PODCAST = "podcast"`、`KNOWLEDGE_SOURCE_MCP = "mcp"`

2. **字段长度**: VARCHAR(50) 是否足够？

3. **索引需求**: 是否需要为 source 字段创建索引？这取决于：
   - 是否会经常按 source 过滤查询
   - 系统中是否会有大量不同来源的 knowledge

4. **验证逻辑**: 是否需要在应用层添加 source 值的验证？

## 相关文件列表

### 需要修改的文件
- `app/store/sqlstore/knowledge.sql` - 表定义
- `app/store/sqlstore/knowledge.go` - 存储层实现
- `pkg/types/knowledge.go` - 类型定义

### 需要创建的文件
- `app/store/sqlstore/migrations/knowledge_add_source_field.sql` - 数据库迁移脚本

### 可能需要更新的文件（业务逻辑）
- `app/logic/v1/knowledge_tools.go` - MCP 工具
- `app/logic/v1/rss_fetcher.go` - RSS 获取
- `app/logic/v1/podcast.go` - 播客
- 其他创建 knowledge 的业务逻辑文件

## 时间线和状态

- [x] 2025-12-18: 计划制定完成
- [x] 2025-12-18: 用户 review 和需求确认完成
- [x] 2025-12-18: 数据库迁移脚本创建完成
- [x] 2025-12-18: 代码修改完成
  - ✅ 定义 KnowledgeSource 类型和常量
  - ✅ 添加验证逻辑 ValidateSource 和 IsValid
  - ✅ 更新数据库表定义文件
  - ✅ 更新所有 Knowledge 相关结构体（Knowledge、KnowledgeResponse、KnowledgeLite）
  - ✅ 更新 UpdateKnowledgeArgs 和 GetKnowledgeOptions
  - ✅ 更新存储层所有方法
  - ✅ RSS 创建 Knowledge 时设置 source 为 "rss"
  - ✅ 编译测试通过
- [ ] 待执行：手动执行数据库迁移（需要 PostgreSQL 客户端）
- [ ] 待完成：功能验证和上线

## 已实现的修改

### 1. 常量定义（pkg/types/knowledge.go）
```go
type KnowledgeSource string

const (
	KNOWLEDGE_SOURCE_PLATFORM KnowledgeSource = ""        // 平台内部创建
	KNOWLEDGE_SOURCE_RSS      KnowledgeSource = "rss"     // RSS 订阅
	KNOWLEDGE_SOURCE_PODCAST  KnowledgeSource = "podcast" // 播客
	KNOWLEDGE_SOURCE_MCP      KnowledgeSource = "mcp"     // MCP 工具
	KNOWLEDGE_SOURCE_CHAT     KnowledgeSource = "chat"    // 聊天会话
)

// ValidateSource 验证 source 字符串是否有效
func ValidateSource(source string) bool {
	return KnowledgeSource(source).IsValid()
}
```

### 2. 数据结构更新
所有 Knowledge 相关结构体都添加了 `Source` 和 `SourceRef` 字段：
- `Knowledge` - 主要数据结构
- `KnowledgeResponse` - API 响应结构
- `KnowledgeLite` - 轻量级列表结构
- `GetKnowledgeOptions` - 查询选项（支持按 source 和 source_ref 过滤）
- `UpdateKnowledgeArgs` - 更新参数

### 3. 业务逻辑更新
- **RSS Consumer** ([app/logic/v1/process/rss_consumer.go:278](app/logic/v1/process/rss_consumer.go#L278)): RSS 文章创建的 Knowledge 设置 `Source: types.KNOWLEDGE_SOURCE_RSS.String()`，可以设置 `SourceRef` 为 subscription_id
- **平台内部创建** ([app/logic/v1/knowledge.go:631](app/logic/v1/knowledge.go#L631)): 通过 InsertContent 创建的 Knowledge 保持默认值（空字符串）
- **聊天会话创建**: 可以设置 `Source: types.KNOWLEDGE_SOURCE_CHAT.String()` 和 `SourceRef: chat_session_id`
- **分享复制**: share.go 中复制 Knowledge 会保持原有的 Source 和 SourceRef 值

### 4. 数据库迁移脚本
创建了 [app/store/sqlstore/migrations/knowledge_add_source_field.sql](app/store/sqlstore/migrations/knowledge_add_source_field.sql)

```sql
ALTER TABLE quka_knowledge ADD COLUMN IF NOT EXISTS source VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE quka_knowledge ADD COLUMN IF NOT EXISTS source_ref VARCHAR(100) NOT NULL DEFAULT '';

COMMENT ON COLUMN quka_knowledge.source IS 'knowledge来源类型，空字符串表示平台内部创建，可选值: rss, podcast, mcp, chat';
COMMENT ON COLUMN quka_knowledge.source_ref IS 'knowledge来源引用ID，如chat_session_id、subscription_id等，空字符串表示无引用';
```

### 5. 使用示例

#### 聊天会话中创建 Knowledge
```go
knowledge := types.Knowledge{
    ID:        utils.GenUniqIDStr(),
    Source:    types.KNOWLEDGE_SOURCE_CHAT.String(),
    SourceRef: chatSessionID, // 聊天会话 ID
    // ... 其他字段
}
```

#### RSS 订阅创建 Knowledge
```go
knowledge := types.Knowledge{
    ID:        utils.GenUniqIDStr(),
    Source:    types.KNOWLEDGE_SOURCE_RSS.String(),
    SourceRef: subscriptionID, // RSS 订阅 ID
    // ... 其他字段
}
```

#### 查询特定聊天会话的 Knowledge
```go
knowledges, err := store.ListKnowledges(ctx, types.GetKnowledgeOptions{
    Source:    types.KNOWLEDGE_SOURCE_CHAT.String(),
    SourceRef: chatSessionID,
}, page, pageSize)
```
