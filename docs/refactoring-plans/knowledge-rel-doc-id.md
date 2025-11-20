# Knowledge 表添加 rel_doc_id 字段改造计划

## 背景

当前系统在删除文档任务(ContentTask)时，需要通过 `FileName` 进行模糊查询来找到相关的 knowledge 记录。这种方式存在以下问题：

1. **效率低**：需要通过 LIKE 查询匹配 title 字段
2. **不精确**：如果有同名文件，可能会误删
3. **维护困难**：依赖 title 的命名格式 `{FileName}-Chunk-{index}`

## 改造目标

在 `Knowledge` 表中添加 `rel_doc_id` 字段，用于存储关联的文档任务 ID：
- 对于通过文档上传生成的 knowledge，存储对应的 `task_id`
- 对于用户直接录入的 knowledge，该字段为空
- 删除文档任务时，可以直接通过 `rel_doc_id` 查询相关的 knowledge

## 改造步骤

### 1. 数据库层面

#### 1.1 创建迁移脚本

**文件**: `app/store/sqlstore/migrations/add_rel_doc_id_to_knowledge.sql`

```sql
-- 为 knowledge 表添加 rel_doc_id 字段
ALTER TABLE quka_knowledge
ADD COLUMN IF NOT EXISTS rel_doc_id VARCHAR(32) NOT NULL DEFAULT '';

-- 为 rel_doc_id 字段添加索引，提升查询性能
-- 使用部分索引，只索引非空值，节省存储空间
CREATE INDEX IF NOT EXISTS idx_knowledge_rel_doc_id ON quka_knowledge(rel_doc_id) WHERE rel_doc_id != '';

-- 添加注释
COMMENT ON COLUMN quka_knowledge.rel_doc_id IS '关联的文档任务ID，如果是用户直接录入则为空字符串';
```

### 2. 数据结构层面

#### 2.1 更新 Knowledge 结构体

**文件**: `pkg/types/knowledge.go:119`

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
	RelDocID    string               `json:"rel_doc_id,omitempty" db:"rel_doc_id"` // 新增：关联的文档任务ID
}
```

#### 2.2 更新 GetKnowledgeOptions

**文件**: `pkg/types/knowledge.go:219`

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
	RelDocID    string               // 新增：根据关联的文档任务ID查询
	TimeRange   *struct {
		St int64
		Et int64
	}
	IncludeExpired bool
	ExpiredOnly    bool
}
```

#### 2.3 更新 GetKnowledgeOptions.Apply 方法

**文件**: `pkg/types/knowledge.go:238`

```go
func (opts GetKnowledgeOptions) Apply(query *sq.SelectBuilder) {
	// ... 现有代码 ...

	// 新增：根据 RelDocID 过滤
	if opts.RelDocID != "" {
		*query = query.Where(sq.Eq{"rel_doc_id": opts.RelDocID})
	}

	// ... 其余代码保持不变 ...
}
```

### 3. 存储层面

#### 3.1 更新 KnowledgeStore 的 Create 方法

**文件**: `app/store/sqlstore/knowledge.go:36`

```go
func (s *KnowledgeStore) Create(ctx context.Context, data types.Knowledge) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = time.Now().Unix()
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "title", "user_id", "space_id", "tags", "content", "content_type", "resource", "kind", "summary", "maybe_date", "stage", "retry_times", "created_at", "updated_at", "expired_at", "rel_doc_id").
		Values(data.ID, data.Title, data.UserID, data.SpaceID, pq.Array(data.Tags), data.Content.String(), data.ContentType, data.Resource, data.Kind, data.Summary, data.MaybeDate, data.Stage, data.RetryTimes, data.CreatedAt, data.UpdatedAt, data.ExpiredAt, data.RelDocID)

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

#### 3.2 更新 KnowledgeStore 的 BatchCreate 方法

**文件**: `app/store/sqlstore/knowledge.go:61`

```go
func (s *KnowledgeStore) BatchCreate(ctx context.Context, datas []*types.Knowledge) error {
	query := sq.Insert(s.GetTable()).
		Columns("id", "title", "user_id", "space_id", "tags", "content", "content_type", "resource", "kind", "summary", "maybe_date", "stage", "retry_times", "created_at", "updated_at", "expired_at", "rel_doc_id")
	for _, data := range datas {
		if data.CreatedAt == 0 {
			data.CreatedAt = time.Now().Unix()
		}
		if data.UpdatedAt == 0 {
			data.UpdatedAt = time.Now().Unix()
		}

		query = query.Values(data.ID, data.Title, data.UserID, data.SpaceID, pq.Array(data.Tags), data.Content.String(), data.ContentType, data.Resource, data.Kind, data.Summary, data.MaybeDate, data.Stage, data.RetryTimes, data.CreatedAt, data.UpdatedAt, data.ExpiredAt, data.RelDocID)
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

#### 3.3 更新 KnowledgeStore 初始化

**文件**: `app/store/sqlstore/knowledge.go:27`

```go
func NewKnowledgeStore(provider SqlProviderAchieve) *KnowledgeStore {
	store := &KnowledgeStore{}
	store.SetProvider(provider)
	store.SetTable(types.TABLE_KNOWLEDGE)
	store.SetAllColumns("id", "title", "user_id", "space_id", "tags", "content", "content_type", "resource", "kind", "summary", "maybe_date", "stage", "retry_times", "created_at", "updated_at", "expired_at", "rel_doc_id")
	return store
}
```

### 4. 业务逻辑层面

#### 4.1 更新 ContentTask 处理逻辑

**文件**: `pkg/plugins/selfhost/logic/v1/process/content_task.go:319`

在创建 knowledge 时，设置 `RelDocID` 字段：

```go
inserts = append(inserts, &types.Knowledge{
	Title:       fmt.Sprintf("%s-Chunk-%d", task.FileName, i+1),
	ID:          id,
	SpaceID:     task.SpaceID,
	UserID:      task.UserID,
	Resource:    task.Resource,
	Content:     encryptContent,
	ContentType: types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN,
	Kind:        types.KNOWLEDGE_KIND_CHUNK,
	Stage:       types.KNOWLEDGE_STAGE_DONE,
	MaybeDate:   time.Now().Local().Format("2006-01-02 15:04"),
	RelDocID:    task.TaskID,  // 新增：设置关联的文档任务ID
	CreatedAt:   time.Now().Unix(),
	UpdatedAt:   time.Now().Unix(),
})
```

#### 4.2 更新 DeleteTask 方法

**文件**: `app/logic/v1/ai_file_dispose.go:52`

简化删除逻辑，直接通过 `RelDocID` 查询：

```go
func (l *AIFileDisposeLogic) DeleteTask(taskID string) error {
	task, err := l.core.Store().ContentTaskStore().GetTask(l.ctx, taskID)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("AIFileDisposeLogic.DeleteTask.ContentTaskStore.GetTask", i18n.ERROR_INTERNAL, err)
	}

	if task == nil {
		return nil
	}

	return l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		// 1. 查找所有相关的 knowledge IDs（通过 RelDocID）
		knowledgeIDs, err := l.core.Store().KnowledgeStore().ListKnowledgeIDs(ctx, types.GetKnowledgeOptions{
			SpaceID:  task.SpaceID,
			RelDocID: taskID,
		}, types.NO_PAGINATION, types.NO_PAGINATION)
		if err != nil && err != sql.ErrNoRows {
			return errors.New("AIFileDisposeLogic.DeleteTask.KnowledgeStore.ListKnowledgeIDs", i18n.ERROR_INTERNAL, err)
		}

		// 2. 如果存在 knowledge，则删除相关数据
		if len(knowledgeIDs) > 0 {
			// 2.1 删除 vectors（通过 knowledge IDs）
			for _, knowledgeID := range knowledgeIDs {
				if err := l.core.Store().VectorStore().BatchDelete(ctx, task.SpaceID, knowledgeID); err != nil {
					return errors.New("AIFileDisposeLogic.DeleteTask.VectorStore.BatchDelete", i18n.ERROR_INTERNAL, err)
				}
			}

			// 2.2 删除 chunks（通过 knowledge IDs）
			if err := l.core.Store().KnowledgeChunkStore().BatchDeleteByIDs(ctx, knowledgeIDs); err != nil {
				return errors.New("AIFileDisposeLogic.DeleteTask.KnowledgeChunkStore.BatchDeleteByIDs", i18n.ERROR_INTERNAL, err)
			}

			// 2.3 获取关联的 meta IDs
			relMetas, err := l.core.Store().KnowledgeRelMetaStore().ListKnowledgesMeta(ctx, knowledgeIDs)
			if err != nil && err != sql.ErrNoRows {
				return errors.New("AIFileDisposeLogic.DeleteTask.KnowledgeRelMetaStore.ListKnowledgesMeta", i18n.ERROR_INTERNAL, err)
			}

			// 提取 meta IDs（去重）
			metaIDs := make(map[string]bool)
			for _, relMeta := range relMetas {
				if relMeta.MetaID != "" {
					metaIDs[relMeta.MetaID] = true
				}
			}

			// 2.4 删除 knowledge_rel_meta
			for _, knowledgeID := range knowledgeIDs {
				if err := l.core.Store().KnowledgeRelMetaStore().Delete(ctx, knowledgeID); err != nil {
					return errors.New("AIFileDisposeLogic.DeleteTask.KnowledgeRelMetaStore.Delete", i18n.ERROR_INTERNAL, err)
				}
			}

			// 2.5 删除 knowledges
			if err := l.core.Store().KnowledgeStore().BatchDelete(ctx, knowledgeIDs); err != nil {
				return errors.New("AIFileDisposeLogic.DeleteTask.KnowledgeStore.BatchDelete", i18n.ERROR_INTERNAL, err)
			}

			// 2.6 删除 knowledge_meta
			for metaID := range metaIDs {
				if err := l.core.Store().KnowledgeMetaStore().Delete(ctx, metaID); err != nil {
					return errors.New("AIFileDisposeLogic.DeleteTask.KnowledgeMetaStore.Delete", i18n.ERROR_INTERNAL, err)
				}
			}
		}

		// 3. 删除 content task
		if err := l.core.Store().ContentTaskStore().Delete(ctx, taskID); err != nil {
			return errors.New("AIFileDisposeLogic.DeleteTask.ContentTaskStore.Delete", i18n.ERROR_INTERNAL, err)
		}

		// 4. 更新文件状态为待删除
		if err = l.core.Store().FileManagementStore().UpdateStatus(ctx, task.SpaceID, []string{task.FileURL}, types.FILE_UPLOAD_STATUS_NEED_TO_DELETE); err != nil {
			return errors.New("AIFileDisposeLogic.DeleteTask.FileManagementStore.UpdateStatus", i18n.ERROR_INTERNAL, err)
		}

		return nil
	})
}
```

## 改造优势

1. **查询效率高**：通过索引字段 `rel_doc_id` 直接查询，避免 LIKE 查询
2. **准确性高**：不依赖 title 命名格式，避免误删
3. **扩展性好**：未来可以支持其他类型的文档关联
4. **向后兼容**：对于已存在的 knowledge，`rel_doc_id` 为空字符串（`''`），不影响现有功能
5. **数据完整性**：使用 `NOT NULL DEFAULT ''` 确保数据一致性，避免 NULL 值处理的复杂性

## 实施注意事项

1. **数据迁移**：现有的 knowledge 记录的 `rel_doc_id` 将自动设置为空字符串（`''`），符合默认值
2. **索引性能**：为 `rel_doc_id` 添加部分索引（`WHERE rel_doc_id != ''`），只索引非空值，提升查询性能同时节省存储空间
3. **兼容性**：确保所有使用 Knowledge 结构体的地方都能正确处理 `rel_doc_id` 字段
4. **NOT NULL 约束**：使用 `NOT NULL DEFAULT ''` 而非 `DEFAULT NULL`，避免空值判断的复杂性，提高数据一致性

## 时间线

- [ ] 创建数据库迁移脚本
- [ ] 更新 Knowledge 数据结构
- [ ] 更新 KnowledgeStore 存储层
- [ ] 更新 ContentTask 处理逻辑
- [ ] 更新 DeleteTask 方法
- [ ] 运行数据库迁移
- [ ] 测试验证

## 相关文件

- `pkg/types/knowledge.go`
- `app/store/sqlstore/knowledge.go`
- `pkg/plugins/selfhost/logic/v1/process/content_task.go`
- `app/logic/v1/ai_file_dispose.go`
- `app/store/sqlstore/migrations/add_rel_doc_id_to_knowledge.sql` (新增)
