<!-- 
====================================================================
📋 原始需求 (用户需求记录)
====================================================================

用户需求描述:
"我现在需要优化一下ListKnowledge的获取，之前不管是用户记录的knowledge还是file chunk出来的knowledge都会无差别的被ListKnowledge方法获取，现在有个想法，ListKnowledge这个方法只获取用户录入的内容，也就是常规状态的knowledge，然后再提供一个接口来获取某个space下以及这个space下的某个资源下的chunk类型的knowledge。"

关键需求点:
1. 现有ListKnowledge方法无差别返回所有knowledge
2. 希望ListKnowledge只返回用户录入的常规knowledge  
3. 新增专门接口获取chunk类型的knowledge
4. 支持按space和resource过滤chunk knowledge

用户补充建议:
"步骤1的查询方案是否可以加个包含和不包含kind两种，这样设置条件是不是更快捷，而不用把所有其他kind都列举一遍"

====================================================================
-->

# Knowledge List Optimization Plan

**计划ID**: knowledge-list-optimization  
**日期**: 2025-01-17  
**状态**: 待审核  
**优先级**: 高  
**作者**: Claude Code  

## 📋 概述

此计划旨在优化 `ListKnowledge` API，使其能够正确区分用户创建的知识和文件切分的知识。

## 🔍 问题描述

目前，`ListKnowledge` 方法返回所有知识条目，无法区分：
- **用户创建的知识**: 用户手动输入的内容（文本、图片、视频、URL）
- **文件切分的知识**: 文件处理后自动生成的知识片段（chunk 类型）

Knowledge 结构使用 `Kind` 字段来标识不同类型：
- `KNOWLEDGE_KIND_TEXT` - 用户文本输入
- `KNOWLEDGE_KIND_IMAGE` - 用户图片输入
- `KNOWLEDGE_KIND_VIDEO` - 用户视频输入
- `KNOWLEDGE_KIND_URL` - 用户 URL 输入
- `KNOWLEDGE_KIND_CHUNK` - 文件处理后的切分片段
- `KNOWLEDGE_KIND_UNKNOWN` - 未知类型

## 🎯 改造目标

1. **优化现有 API**: 修改 `ListKnowledge` 只返回用户创建的知识（排除 chunk 类型）
2. **新增文件任务列表 API**: 创建获取 ContentTask 列表的接口，展示用户的文件处理任务
3. **新增任务详情 API**: 根据 task 获取该任务下所有 chunk knowledge 的接口
4. **保持兼容性**: 确保更改不会破坏现有功能

## 📝 实施步骤

### 步骤1: 增强 GetKnowledgeOptions 结构体
**文件**: `pkg/types/knowledge.go:203`

首先需要修改 `GetKnowledgeOptions` 结构体，添加包含和排除 Kind 的选项：

```go
type GetKnowledgeOptions struct {
    ID         string
    IDs        []string
    Kind       []KnowledgeKind      // 包含指定的 Kind
    ExcludeKind []KnowledgeKind     // 排除指定的 Kind (新增)
    SpaceID    string
    UserID     string
    Resource   *ResourceQuery
    Stage      KnowledgeStage
    RetryTimes int
    Keywords   string
    TimeRange  *struct {
        St int64
        Et int64
    }
}
```

然后更新 `Apply` 方法来处理排除逻辑：

```go
func (opts GetKnowledgeOptions) Apply(query *sq.SelectBuilder) {
    // ... 其他字段的处理保持不变 ...
    
    if len(opts.Kind) > 0 {
        *query = query.Where(sq.Eq{"kind": opts.Kind})
    }
    if len(opts.ExcludeKind) > 0 {
        *query = query.Where(sq.NotEq{"kind": opts.ExcludeKind})
    }
    
    // ... 其他字段的处理保持不变 ...
}
```

### 步骤2: 修改 ListKnowledges 方法
**文件**: `app/logic/v1/knowledge.go:127`

```go
func (l *KnowledgeLogic) ListKnowledges(spaceID string, keywords string, resource *types.ResourceQuery, page, pagesize uint64) ([]*types.Knowledge, uint64, error) {
    opts := types.GetKnowledgeOptions{
        SpaceID:     spaceID,
        Resource:    resource,
        Keywords:    keywords,
        ExcludeKind: []types.KnowledgeKind{types.KNOWLEDGE_KIND_CHUNK}, // 排除 chunk 类型
    }
    // ... 其余实现保持不变
}
```

### 步骤3: 新增 ListChunkKnowledges 方法
**文件**: `app/logic/v1/knowledge.go`

```go
func (l *KnowledgeLogic) ListChunkKnowledges(spaceID string, resource *types.ResourceQuery, page, pagesize uint64) ([]*types.Knowledge, uint64, error) {
    opts := types.GetKnowledgeOptions{
        SpaceID:  spaceID,
        Resource: resource,
        Kind:     []types.KnowledgeKind{types.KNOWLEDGE_KIND_CHUNK},
    }
    
    list, err := l.core.Store().KnowledgeStore().ListKnowledges(l.ctx, opts, page, pagesize)
    if err != nil && err != sql.ErrNoRows {
        return nil, 0, errors.New("KnowledgeLogic.ListChunkKnowledges.KnowledgeStore.ListKnowledges", i18n.ERROR_INTERNAL, err)
    }

    for _, v := range list {
        if v.Content, err = l.core.DecryptData(v.Content); err != nil {
            return nil, 0, errors.New("KnowledgeLogic.ListChunkKnowledges.DecryptData", i18n.ERROR_INTERNAL, err)
        }
    }

    total, err := l.core.Store().KnowledgeStore().Total(l.ctx, opts)
    if err != nil {
        return nil, 0, errors.New("KnowledgeLogic.ListChunkKnowledges.KnowledgeStore.Total", i18n.ERROR_INTERNAL, err)
    }

    return list, total, nil
}
```

### 步骤4: 新增 HTTP Handler
**File**: `cmd/service/handler/knowledge.go`

```go
type ListChunkKnowledgeRequest struct {
    Resource string `json:"resource" form:"resource"`
    Page     uint64 `json:"page" form:"page" binding:"required"`
    PageSize uint64 `json:"pagesize" form:"pagesize" binding:"required,lte=50"`
}

type ListChunkKnowledgeResponse struct {
    List  []*types.KnowledgeResponse `json:"list"`
    Total uint64                     `json:"total"`
}

func (s *HttpSrv) ListChunkKnowledge(c *gin.Context) {
    var req ListChunkKnowledgeRequest

    if err := utils.BindArgsWithGin(c, &req); err != nil {
        response.APIError(c, err)
        return
    }

    var resource *types.ResourceQuery
    if req.Resource != "" {
        resource = &types.ResourceQuery{
            Include: []string{req.Resource},
        }
    }

    spaceID, _ := v1.InjectSpaceID(c)
    list, total, err := v1.NewKnowledgeLogic(c, s.Core).ListChunkKnowledges(spaceID, resource, req.Page, req.PageSize)
    if err != nil {
        response.APIError(c, err)
        return
    }

    knowledgeList := lo.Map(list, func(item *types.Knowledge, index int) *types.KnowledgeResponse {
        liteContent := KnowledgeToKnowledgeResponseLite(item)
        liteContent.Content = utils.ReplaceMarkdownStaticResourcesWithPresignedURL(liteContent.Content, s.Core.Plugins.FileStorage())
        return liteContent
    })

    response.APISuccess(c, ListChunkKnowledgeResponse{
        List:  knowledgeList,
        Total: total,
    })
}
```

### 步骤5: 添加路由
**文件**: `cmd/service/router.go` (第164行附近)

```go
viewScope := knowledge.Group("")
{
    viewScope.Use(middleware.VerifySpaceIDPermission(s.Core, srv.PermissionView))
    viewScope.GET("", s.GetKnowledge)
    viewScope.GET("/list", spaceLimit("knowledge_list"), s.ListKnowledge)
    viewScope.GET("/chunk/list", spaceLimit("knowledge_list"), s.ListChunkKnowledge) // 新增路由
    viewScope.POST("/query", spaceLimit("chat_message"), s.Query)
    viewScope.GET("/time/list", spaceLimit("knowledge_list"), s.GetDateCreatedKnowledge)
}
```

## 🔄 优化后的效果

使用 `ExcludeKind` 字段后，查询逻辑变得更加清晰和高效：

1. **用户知识查询**: 只需要排除 chunk 类型，一个条件搞定
2. **文件切分知识查询**: 只需要包含 chunk 类型，简洁明了
3. **扩展性**: 未来如果需要排除或包含其他类型，都很容易实现
4. **性能优化**: 避免了枚举所有非 chunk 类型，减少了代码冗余

## 🛡️ 关键考虑点

### 1. 向后兼容性
- **风险**: 现有 `ListKnowledge` API 行为将发生变化
- **缓解措施**: 需要验证前端是否依赖 chunk 类型数据
- **建议**: 考虑添加查询参数来控制过滤行为

### 2. 性能影响
- **分析**: 添加 `Kind` 过滤器应该通过减少结果集来提高性能
- **数据库**: 确保 `kind` 列上有适当的索引
- **监控**: 跟踪更改前后的查询性能

### 3. 安全性和权限
- **访问控制**: 两个 API 使用相同的权限级别 (`PermissionView`)
- **限流**: 两个 API 共享相同的限流设置 (`knowledge_list`)
- **数据加密**: 两个 API 都一致地处理加密内容

### 4. 测试要求
- `ListKnowledges` 和 `ListChunkKnowledges` 方法的单元测试
- HTTP 端点的集成测试
- 数据库查询的性能测试
- 前端集成测试

## 📊 最终 API 设计

```http
GET /api/v1/{spaceid}/knowledge/list          # User-created knowledge (excludes chunks)
GET /api/v1/{spaceid}/knowledge/chunk/list    # File-chunked knowledge (chunks only)
```

### 请求参数
两个端点都支持:
- `resource` (可选): 按资源类型过滤
- `page` (必填): 页码
- `pagesize` (必填): 页面大小 (最大 50)

### 响应格式
```json
{
  "list": [
    {
      "id": "string",
      "space_id": "string",
      "title": "string",
      "content": "string",
      "content_type": "string",
      "kind": "string",
      "resource": "string",
      "user_id": "string",
      "stage": "string",
      "created_at": "number",
      "updated_at": "number"
    }
  ],
  "total": "number"
}
```

## ❓ 需要确认的问题

1. **过滤策略**: 是否应该在 `ListKnowledge` 中完全排除 chunk 类型，还是添加查询参数以保持向后兼容性？

2. **附加过滤器**: chunk 知识端点是否需要额外的过滤选项（如按文件类型、处理日期等）？

3. **响应增强**: 是否应该在响应中添加元数据来标识知识来源类型？

4. **性能要求**: chunk 知识列表是否有特定的性能要求？

5. **前端影响**: 使用这些 API 的前端应用的预期影响是什么？

## 📅 时间线

- **计划创建**: 2025-01-17
- **当前状态**: 待审核
- **预期实施**: 待定（审核通过后）
- **测试阶段**: 待定
- **部署上线**: 待定

## 🔗 相关文件

- `app/logic/v1/knowledge.go` - Main business logic
- `cmd/service/handler/knowledge.go` - HTTP handlers
- `cmd/service/router.go` - Route definitions
- `pkg/types/knowledge.go` - Data structures

---

**注意**: 此计划需要在开始实施前进行审核和批准。