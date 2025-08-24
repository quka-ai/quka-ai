<!-- 
====================================================================
📋 原始需求 (用户需求记录)
====================================================================

用户需求描述:
"我的想法是当用户设置某个resource下内容的过期时间为30天时,就是从knowledge创建时间起,30天后过期,你可以帮我设计一下这个功能的实现方式"

关键需求点:
1. 用户可以为每个resource设置内容有效期
2. 以knowledge的创建时间为基准计算过期时间
3. 支持灵活的过期时间设置（如30天）
4. 过期的知识内容需要被自动处理

====================================================================
-->

# Resource Content Expiration Feature Design

**计划ID**: resource-content-expiration  
**日期**: 2025-01-17  
**状态**: 待审核  
**优先级**: 高  
**作者**: Claude Code  

## 📋 概述

此计划旨在为QukaAI系统设计和实现资源内容有效期功能，允许用户为每个resource设置内容的有效期，并自动处理过期的knowledge内容。

## 🔍 问题分析

### 现有数据结构分析

**Resource 表结构** (`pkg/types/resource.go`):
- `Cycle` 字段当前表示资源周期（0为不限制）
- 缺少明确的有效期时间单位定义
- 缺少有效期计算逻辑

**Knowledge 表结构** (`pkg/types/knowledge.go`):
- `Resource` 字段关联到resource ID
- `CreatedAt` 字段记录知识创建时间（UNIX时间戳）
- 当前无过期时间相关字段

### 业务场景分析

1. **用户设置有效期**: 用户在创建/编辑resource时设置内容有效期
2. **过期判断**: 系统根据knowledge创建时间 + resource有效期判断是否过期
3. **过期处理**: 过期的knowledge需要标记、隐藏或删除

## 🎯 设计目标

1. **灵活的有效期配置**: 支持多种时间单位（天、周、月、年）
2. **自动过期检测**: 提供查询接口过滤过期内容
3. **过期处理策略**: 支持软删除、隐藏、硬删除等策略
4. **API兼容性**: 确保现有API的向后兼容
5. **性能优化**: 避免过期检查影响正常查询性能

## 🛠 技术方案

### 1. 数据结构设计

#### 1.1 扩展Knowledge结构（预计算存储方案）

在Knowledge表中新增`expired_at`字段，在创建时根据resource的cycle预先计算过期时间：

```go
type Knowledge struct {
    // 现有字段...
    ID          string               `json:"id" db:"id"`
    SpaceID     string               `json:"space_id" db:"space_id"`
    Kind        KnowledgeKind        `json:"kind" db:"kind"`
    Resource    string               `json:"resource" db:"resource"`
    Title       string               `json:"title" db:"title"`
    // ... 其他现有字段
    CreatedAt   int64                `json:"created_at" db:"created_at"`
    UpdatedAt   int64                `json:"updated_at" db:"updated_at"`
    
    // 新增字段
    ExpiredAt   *int64               `json:"expired_at,omitempty" db:"expired_at"` // 过期时间戳，NULL表示永不过期
}

type KnowledgeResponse struct {
    // 现有字段...
    // 新增字段
    ExpiredAt   *int64 `json:"expired_at,omitempty" db:"expired_at"`
    IsExpired   bool   `json:"is_expired,omitempty" db:"-"`            // 计算字段，是否已过期
}
```

#### 1.2 过期时间计算函数

```go
// 根据resource计算过期时间
func CalculateExpiredAt(createdAt int64, cycle int) *int64 {
    if cycle <= 0 {
        return nil // 永不过期
    }
    expiredAt := createdAt + int64(cycle*24*3600)
    return &expiredAt
}

// 检查是否过期
func (k *Knowledge) IsExpired() bool {
    if k.ExpiredAt == nil {
        return false // 永不过期
    }
    return time.Now().Unix() > *k.ExpiredAt
}
```

#### 1.3 扩展GetKnowledgeOptions

```go
type GetKnowledgeOptions struct {
    // ... 现有字段 ...
    IncludeExpired   bool   `json:"include_expired"`   // 是否包含过期内容，默认false
    ExpiredOnly      bool   `json:"expired_only"`      // 只返回过期内容
    ExpirationCheck  bool   `json:"expiration_check"`  // 是否进行过期检查
}
```

### 2. 数据库设计

#### 2.1 Knowledge表结构更新

```sql
-- 添加过期时间字段
ALTER TABLE quka_knowledge 
ADD COLUMN expired_at BIGINT DEFAULT NULL;

-- 添加过期时间索引（查询性能关键）
CREATE INDEX idx_knowledge_expired_at ON quka_knowledge(expired_at);

-- 添加复合索引用于按resource和过期状态查询
CREATE INDEX idx_knowledge_resource_expired_at ON quka_knowledge(resource, expired_at);
```

#### 2.2 数据迁移脚本

```sql
-- 为现有knowledge计算并设置过期时间
UPDATE quka_knowledge k 
SET expired_at = (
    SELECT CASE 
        WHEN r.cycle > 0 THEN k.created_at + r.cycle * 86400
        ELSE NULL 
    END
    FROM quka_resource r 
    WHERE r.id = k.resource
)
WHERE k.resource IS NOT NULL AND k.resource != '';
```

### 3. API设计

#### 3.1 Resource管理API增强

Resource接口保持不变，仍然使用现有的cycle字段：

**更新Resource接口**:
```
PUT /api/v1/resource/{id}
{
  "title": "资源标题",
  "description": "资源描述",
  "tag": "标签",
  "cycle": 30  // 有效期天数，0为永不过期
}
```

**获取Resource接口响应**:
```json
{
  "id": "resource_id",
  "title": "资源标题", 
  "cycle": 30,
  "created_at": 1642665600
}
```

#### 3.2 Knowledge查询API增强

**ListKnowledge接口增加过期控制参数**:
```
GET /api/v1/knowledge?include_expired=false&expired_only=false
```

**Knowledge响应增加过期信息**:
```json
{
  "id": "knowledge_id",
  "title": "知识标题",
  "expired_at": 1645257600,
  "is_expired": false,
  "created_at": 1642665600
}
```

#### 3.3 新增过期管理API

**获取过期Knowledge列表**:
```
GET /api/v1/knowledge/expired?space_id={space_id}&resource={resource_id}
```

**批量清理过期Knowledge**:
```
DELETE /api/v1/knowledge/expired
{
  "space_id": "space_id",
  "resource_ids": ["resource_id1", "resource_id2"],
  "strategy": "soft_delete" // soft_delete, hard_delete
}
```

### 4. 业务逻辑实现

#### 4.1 Knowledge查询逻辑（预计算方案）

```go
func (s *KnowledgeStore) ListKnowledges(ctx context.Context, opts types.GetKnowledgeOptions, page, pageSize uint64) ([]*types.Knowledge, error) {
    query := sq.Select(s.GetAllColumns()...).From(s.GetTable())
    
    // 应用现有过滤条件
    opts.Apply(&query)
    
    // 过期检查逻辑（超简单！）
    if opts.ExpirationCheck {
        now := time.Now().Unix()
        if !opts.IncludeExpired {
            // 排除过期内容：WHERE (expired_at IS NULL OR expired_at > NOW())
            query = query.Where(sq.Or{
                sq.Eq{"expired_at": nil},
                sq.Gt{"expired_at": now},
            })
        } else if opts.ExpiredOnly {
            // 只返回过期内容：WHERE expired_at IS NOT NULL AND expired_at <= NOW()
            query = query.Where(sq.And{
                sq.NotEq{"expired_at": nil},
                sq.LtOrEq{"expired_at": now},
            })
        }
    }
    
    // 分页和排序
    if page > 0 && pageSize > 0 {
        query = query.Limit(pageSize).Offset((page - 1) * pageSize)
    }
    query = query.OrderBy("created_at DESC")
    
    // 执行查询...
}
```

#### 4.2 Knowledge创建时自动设置过期时间

```go
func (s *KnowledgeStore) Create(ctx context.Context, knowledge *types.Knowledge) error {
    // 如果指定了resource，自动计算过期时间
    if knowledge.Resource != "" {
        resource, err := s.getResourceByID(ctx, knowledge.Resource)
        if err == nil && resource.Cycle > 0 {
            expiredAt := knowledge.CreatedAt + int64(resource.Cycle*24*3600)
            knowledge.ExpiredAt = &expiredAt
        }
    }
    
    // 执行创建...
    query := sq.Insert(s.GetTable()).
        Columns("id", "space_id", "resource", "title", "content", "created_at", "expired_at").
        Values(knowledge.ID, knowledge.SpaceID, knowledge.Resource, knowledge.Title, 
               knowledge.Content, knowledge.CreatedAt, knowledge.ExpiredAt)
    
    // 执行SQL...
}
```

#### 4.3 Resource.cycle变更时的一致性处理

```go
// Resource业务逻辑层：更新cycle时同步更新相关knowledge
func (l *ResourceLogic) UpdateResourceCycle(ctx context.Context, resourceID string, newCycle int) error {
    // 1. 更新resource
    err := l.core.Store().ResourceStore().Update(ctx, resourceID, newCycle)
    if err != nil {
        return err
    }
    
    // 2. 批量更新相关knowledge的过期时间
    err = l.core.Store().KnowledgeStore().UpdateExpiredAtByResource(ctx, resourceID, newCycle)
    if err != nil {
        return err
    }
    
    return nil
}

// Knowledge存储层：批量更新过期时间
func (s *KnowledgeStore) UpdateExpiredAtByResource(ctx context.Context, resourceID string, cycle int) error {
    var query sq.UpdateBuilder
    
    if cycle > 0 {
        // 重新计算过期时间：created_at + cycle * 86400
        query = sq.Update(s.GetTable()).
            Set("expired_at", sq.Expr("created_at + ? * 86400", cycle)).
            Where(sq.Eq{"resource": resourceID})
    } else {
        // 设置为永不过期
        query = sq.Update(s.GetTable()).
            Set("expired_at", nil).
            Where(sq.Eq{"resource": resourceID})
    }
    
    queryString, args, err := query.ToSql()
    if err != nil {
        return err
    }
    
    _, err = s.GetMaster(ctx).Exec(queryString, args...)
    return err
}
```

### 5. 定时任务设计

#### 5.1 过期内容清理任务（高性能版本）

```go
type ExpirationCleanupTask struct {
    core *core.Core
}

func (t *ExpirationCleanupTask) CleanupExpiredKnowledge(ctx context.Context) error {
    now := time.Now().Unix()
    
    // 1. 直接通过SQL查找过期knowledge（无需JOIN，极快！）
    query := sq.Select("id", "space_id", "title").
        From("quka_knowledge").
        Where(sq.And{
            sq.NotEq{"expired_at": nil},
            sq.LtOrEq{"expired_at": now},
        }).
        Limit(1000) // 批量处理
    
    queryString, args, err := query.ToSql()
    if err != nil {
        return err
    }
    
    var expiredKnowledges []struct {
        ID      string `db:"id"`
        SpaceID string `db:"space_id"`
        Title   string `db:"title"`
    }
    
    err = t.core.Store().GetReplica(ctx).Select(&expiredKnowledges, queryString, args...)
    if err != nil {
        return err
    }
    
    // 2. 根据策略批量处理过期内容
    switch t.getCleanupStrategy() {
    case "soft_delete":
        err = t.batchSoftDelete(ctx, expiredKnowledges)
    case "hard_delete":
        err = t.batchHardDelete(ctx, expiredKnowledges)
    case "archive":
        err = t.batchArchive(ctx, expiredKnowledges)
    }
    
    return err
}

// 批量硬删除（示例）
func (t *ExpirationCleanupTask) batchHardDelete(ctx context.Context, knowledges []struct{ID, SpaceID, Title string}) error {
    if len(knowledges) == 0 {
        return nil
    }
    
    ids := make([]string, len(knowledges))
    for i, k := range knowledges {
        ids[i] = k.ID
    }
    
    // 批量删除
    query := sq.Delete("quka_knowledge").Where(sq.Eq{"id": ids})
    queryString, args, err := query.ToSql()
    if err != nil {
        return err
    }
    
    _, err = t.core.Store().GetMaster(ctx).Exec(queryString, args...)
    return err
}
```

## 📋 实施计划

### 阶段1：数据库结构更新 (1天)
1. ✅ 为Knowledge表添加expired_at字段
2. ✅ 创建expired_at相关索引
3. ✅ 编写数据迁移脚本为现有数据设置过期时间

### 阶段2：核心业务逻辑实现 (2天)
1. ✅ 更新Knowledge创建逻辑，自动计算expired_at
2. ✅ 实现Resource.cycle变更时的一致性更新逻辑
3. ✅ 优化Knowledge查询方法，使用expired_at字段

### 阶段3：API接口更新 (1-2天)
1. ✅ 更新Resource CRUD接口
2. ✅ 更新Knowledge查询接口
3. ✅ 新增过期管理接口

### 阶段4：定时任务和清理策略 (1-2天)
1. ✅ 实现过期内容清理任务
2. ✅ 配置定时任务调度
3. ✅ 实现多种清理策略

### 阶段5：测试和优化 (1-2天)
1. ✅ 单元测试编写
2. ✅ 性能测试和优化
3. ✅ 集成测试

## 🔍 关键考虑点

### 1. 性能优势（预计算方案）
- **查询性能**: 无需JOIN，直接通过expired_at索引查询，性能极佳
- **清理效率**: 简单的时间戳对比，批量操作高效
- **索引简单**: 只需单字段索引，维护成本低
- **扩展性强**: 未来可支持更复杂的过期策略

### 2. 数据一致性保障
- **创建时计算**: Knowledge创建时自动根据Resource.cycle计算expired_at
- **变更同步**: Resource.cycle变更时批量更新相关Knowledge的expired_at
- **事务保证**: 所有相关更新在同一事务中完成
- **数据迁移**: 现有数据平滑迁移到新结构

### 3. 用户体验
- **透明操作**: 用户无需感知expired_at字段，仍通过cycle设置
- **即时生效**: 过期设置变更立即对所有相关内容生效
- **清晰状态**: API响应明确显示过期状态和时间

### 4. 安全和可靠性
- **权限控制**: 只有resource owner可以设置过期时间
- **审计日志**: 记录过期处理和cycle变更操作
- **数据备份**: 重要数据的备份和恢复机制
- **错误处理**: 一致性更新失败时的回滚机制

## 📝 需要确认的问题

1. **过期策略**: 是否需要支持软删除、归档等多种过期处理策略？
2. **通知机制**: 是否需要在内容即将过期时通知用户？
3. **批量操作**: 是否需要支持批量设置多个resource的过期时间？
4. **历史记录**: 是否需要记录resource过期设置的变更历史？
5. **数据迁移**: 是否需要在生产环境中逐步迁移现有数据？

## 🔗 相关文件

- `pkg/types/resource.go` - Resource数据结构
- `pkg/types/knowledge.go` - Knowledge数据结构  
- `app/store/sqlstore/resource.go` - Resource数据库操作
- `app/store/sqlstore/knowledge.go` - Knowledge数据库操作
- `app/logic/v1/resource.go` - Resource业务逻辑
- `app/logic/v1/knowledge.go` - Knowledge业务逻辑
- `cmd/service/handler/resource.go` - Resource API处理
- `cmd/service/handler/knowledge.go` - Knowledge API处理