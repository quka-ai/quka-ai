# 模型列表接口筛选逻辑优化方案

## 问题描述

当前 `ListModelConfigs` 接口存在性能问题：

- 先从数据库获取所有模型数据
- 然后在内存中进行筛选过滤
- 这种方式在数据量大时会造成不必要的查询和内存开销

### 当前实现问题

在 [cmd/service/handler/model_config.go:53-129](cmd/service/handler/model_config.go#L53-L129) 中：

```go
// 获取所有模型
models, err = logic.ListModelsWithProvider(providerID)

// 然后在内存中过滤
for _, model := range models {
    if status != nil && model.Status != *status {
        continue
    }
    if modelType != "" && model.ModelType != modelType {
        continue
    }
    if modelName != "" && model.ModelName != modelName {  // 这里应该搜索 display_name 且支持模糊搜索
        continue
    }
    // ...
}
```

**主要问题：**

1. `model_name` 参数应该搜索数据库的 `display_name` 字段，而不是 `model_name` 字段
2. 应该支持模糊搜索（LIKE），而不是精确匹配
3. `model_type` 等其他筛选条件应该在 SQL 层面完成，而不是在内存中过滤

## 改造目标

1. **使用结构体绑定查询参数**：统一使用 Gin 的结构体绑定机制，提高代码规范性
2. **将所有筛选条件应用到 SQL 查询中**：减少数据库查询量和内存消耗
3. **正确的字段映射**：
   - `model_name` 查询参数对应数据库的 `display_name` 字段，支持模糊搜索
   - `model_type` 查询参数对应数据库的 `model_type` 字段，精确匹配
4. **保持 API 接口不变**：对前端透明

## 实施方案

### 1. 扩展 ListModelConfigOptions (pkg/types/model_provider.go)

在 `ListModelConfigOptions` 中添加 `DisplayName` 字段：

```go
type ListModelConfigOptions struct {
    ProviderID       string
    ModelType        string
    ModelName        string  // 保留，用于精确匹配 model_name 字段
    DisplayName      string  // 新增，用于模糊搜索 display_name 字段
    IsMultiModal     *bool
    ThinkingSupport  *int
    ThinkingRequired *bool
    Status           *int
}

func (opt ListModelConfigOptions) Apply(query *sq.SelectBuilder) {
    // ... 现有逻辑 ...

    if opt.ModelName != "" {
        *query = query.Where(sq.Like{"model_name": "%" + opt.ModelName + "%"})
    }

    // 新增 DisplayName 模糊搜索
    if opt.DisplayName != "" {
        *query = query.Where(sq.Like{"display_name": "%" + opt.DisplayName + "%"})
    }
}
```

### 2. 创建请求结构体 (app/logic/v1/model_config.go 或 handler 层)

定义 `ListModelConfigsRequest` 结构体用于参数绑定：

```go
// ListModelConfigsRequest 获取模型配置列表请求
type ListModelConfigsRequest struct {
    ProviderID       string `form:"provider_id"`        // 提供商ID
    ModelType        string `form:"model_type"`         // 模型类型
    ModelName        string `form:"model_name"`         // 模型名称（用于搜索 display_name）
    Status           *int   `form:"status"`             // 状态
    IsMultiModal     *bool  `form:"is_multi_modal"`     // 是否多模态
    ThinkingSupport  *int   `form:"thinking_support"`   // 思考功能支持类型
    ThinkingRequired *bool  `form:"thinking_required"`  // 是否需要思考功能
}
```

### 3. 修改 Handler 层 (cmd/service/handler/model_config.go)

使用结构体绑定参数，移除内存过滤逻辑：

```go
// ListModelConfigs 获取模型配置列表
func (s *HttpSrv) ListModelConfigs(c *gin.Context) {
    var req v1.ListModelConfigsRequest
    if err := utils.BindArgsWithGin(c, &req); err != nil {
        response.APIError(c, err)
        return
    }

    logic := v1.NewModelConfigLogic(c, s.Core)

    // 构建筛选选项
    opts := types.ListModelConfigOptions{
        ProviderID:       req.ProviderID,
        ModelType:        req.ModelType,
        DisplayName:      req.ModelName,  // 注意：前端的 model_name 参数映射到 DisplayName 字段
        Status:           req.Status,
        IsMultiModal:     req.IsMultiModal,
        ThinkingSupport:  req.ThinkingSupport,
        ThinkingRequired: req.ThinkingRequired,
    }

    // 直接使用筛选条件查询，不再在内存中过滤
    models, err := logic.ListModelsWithProviderFiltered(opts)
    if err != nil {
        response.APIError(c, err)
        return
    }

    response.APISuccess(c, map[string]interface{}{
        "list": models,
    })
}
```

### 4. 修改 Logic 层 (app/logic/v1/model_config.go)

新增 `ListModelsWithProviderFiltered` 方法：

```go
// ListModelsWithProviderFiltered 列出模型配置（包含提供商信息），支持完整筛选
func (l *ModelConfigLogic) ListModelsWithProviderFiltered(opts types.ListModelConfigOptions) ([]*types.ModelConfig, error) {
    models, err := l.core.Store().ModelConfigStore().ListWithProvider(l.ctx, opts)
    if err != nil && err != sql.ErrNoRows {
        return nil, errors.New("ModelConfigLogic.ListModelsWithProviderFiltered.ListWithProvider", i18n.ERROR_INTERNAL, err)
    }

    // 添加支持 Reader 功能的提供商作为虚拟模型（如果需要）
    // 注意：Reader 模型需要根据筛选条件决定是否包含
    if opts.ModelType == "" || opts.ModelType == types.MODEL_TYPE_READER {
        readerModels, err := l.getReaderProviderAsModels(opts.ProviderID)
        if err != nil {
            return nil, err
        }

        // 如果有 DisplayName 筛选条件，需要对 Reader 模型进行过滤
        if opts.DisplayName != "" {
            filteredReaderModels := make([]*types.ModelConfig, 0)
            for _, model := range readerModels {
                // 模糊匹配 DisplayName
                if strings.Contains(strings.ToLower(model.DisplayName), strings.ToLower(opts.DisplayName)) {
                    filteredReaderModels = append(filteredReaderModels, model)
                }
            }
            readerModels = filteredReaderModels
        }

        // 合并真实模型和 Reader 虚拟模型
        models = append(models, readerModels...)
    }

    return models, nil
}
```

### 5. Store 层无需修改

当前 `ListWithProvider` 方法已经通过 `opts.Apply(&query)` 支持筛选，只需要在 `ListModelConfigOptions` 中添加 `DisplayName` 字段即可。

## 关键考虑点

### 1. 结构体参数绑定

- 使用 Gin 的 `form` 标签进行查询参数绑定
- 参考 Knowledge 接口的实现方式，保持代码风格一致
- 提高代码可维护性和可读性

### 2. API 兼容性

- 前端查询参数 `model_name` 映射到后端的 `display_name` 字段搜索
- 所有其他参数保持不变
- URL 接口不变，对前端透明

### 3. Reader 虚拟模型处理

- Reader 模型是根据 Provider 动态生成的虚拟模型
- 需要根据 `model_type` 筛选条件决定是否包含 Reader 模型
- Reader 模型的 `DisplayName` 筛选在内存中完成（因为是虚拟数据）
- 其他筛选条件（如 status）也需要在内存中进行

### 4. 性能提升

- 数据库层面过滤：减少网络传输和内存占用
- 真实模型的筛选在 SQL 层完成
- 虚拟模型的筛选在内存中完成（数量少，影响小）
- 索引优化：确保 `display_name` 和 `model_type` 字段有适当的索引

### 5. 模糊搜索

- `display_name` 使用 LIKE 模糊搜索（SQL 层）
- Reader 模型使用 `strings.Contains` 进行模糊匹配（内存层）
- 需要注意 SQL 注入防护（squirrel 库已处理）

## 需要确认的问题

1. **前端影响**：前端目前传递的 `model_name` 参数是用来搜索显示名称还是模型名称？

   - 如果是显示名称：按方案实施 ✅
   - 如果是模型名称：需要调整前端，使用不同的参数名

2. **数据库索引**：是否需要为 `display_name` 字段添加索引以提升模糊搜索性能？
   - 建议添加普通索引，提升 LIKE 查询性能

## 实施步骤

1. ✅ 分析当前代码结构和数据流
2. ✅ 制定优化方案（包含结构体绑定设计）
3. ✅ 在 `app/logic/v1/model_config.go` 中定义 `ListModelConfigsRequest` 结构体
4. ✅ 修改 `pkg/types/model_provider.go` 添加 `DisplayName` 字段到 `ListModelConfigOptions`
5. ✅ 修改 `cmd/service/handler/model_config.go` 使用结构体绑定参数，移除内存过滤
6. ✅ 修改 `app/logic/v1/model_config.go` 添加 `ListModelsWithProviderFiltered` 方法
7. ✅ 编译通过，代码检查通过（go build, go vet）
8. ⏳ 测试各种筛选场景（包括真实模型和 Reader 虚拟模型）
9. ⏳ 确认与前端的参数对接

## 相关文件

- [cmd/service/handler/model_config.go](cmd/service/handler/model_config.go) - Handler 层
- [app/logic/v1/model_config.go](app/logic/v1/model_config.go) - Logic 层
- [app/store/sqlstore/model_config.go](app/store/sqlstore/model_config.go) - Store 层
- [pkg/types/model_provider.go](pkg/types/model_provider.go) - 类型定义

## 实施总结

### 代码修改

✅ 所有代码修改已完成，编译和静态检查通过

### 修改内容

1. **新增结构体** [model_config.go:55-64](app/logic/v1/model_config.go#L55-L64)

   - `ListModelConfigsRequest` 用于参数绑定

2. **扩展选项结构** [model_provider.go:88-97](pkg/types/model_provider.go#L88-L97)

   - `ListModelConfigOptions` 添加 `DisplayName` 字段
   - 更新 `Apply` 方法支持 `DisplayName` 模糊搜索

3. **新增业务方法** [model_config.go:268-339](app/logic/v1/model_config.go#L268-L339)

   - `ListModelsWithProviderFiltered` 支持完整筛选
   - `filterReaderModels` 对虚拟模型进行内存过滤

4. **简化 Handler 层** [model_config.go:53-84](cmd/service/handler/model_config.go#L53-L84)
   - 从 73 行代码简化到 32 行代码
   - 使用结构体绑定替代手动解析
   - 移除内存过滤逻辑

### 性能提升

- ✅ 数据库层面筛选，减少网络传输
- ✅ 减少内存占用和 CPU 消耗
- ✅ 支持 `display_name` 模糊搜索

### 代码质量

- ✅ 代码更简洁（减少 41 行代码）
- ✅ 参数绑定更规范
- ✅ 逻辑更清晰易维护

## 状态追踪

- **状态**: ✅ 实施完成
- **创建时间**: 2025-10-15
- **完成时间**: 2025-10-15
- **待测试**: 需要真实环境测试各种筛选场景
