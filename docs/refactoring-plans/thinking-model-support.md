# 模型思考功能支持增强计划

## 问题背景

当前系统的模型管理设计已经支持基本的模型配置，但缺乏对 AI 模型思考（thinking）功能的精细化管理。根据用户需求，不同的 AI 模型对思考功能有不同的支持方式：

1. **强制思考模型**：某些模型（如 Claude-4）必须开启思考功能，无法关闭
2. **可选思考模型**：某些模型支持思考功能，但可以选择是否开启
3. **不支持思考模型**：某些模型完全不支持思考功能

## 当前架构分析

### 现有数据结构

```go
// ModelConfig 当前结构
type ModelConfig struct {
    ID           string          `json:"id" db:"id"`
    ProviderID   string          `json:"provider_id" db:"provider_id"`
    ModelName    string          `json:"model_name" db:"model_name"`
    DisplayName  string          `json:"display_name" db:"display_name"`
    ModelType    string          `json:"model_type" db:"model_type"`         // chat/embedding/completion
    IsMultiModal bool            `json:"is_multi_modal" db:"is_multi_modal"` // 仅对chat有效
    Status       int             `json:"status" db:"status"`
    Config       json.RawMessage `json:"config" db:"config"`
    CreatedAt    int64           `json:"created_at" db:"created_at"`
    UpdatedAt    int64           `json:"updated_at" db:"updated_at"`
    Provider     *ModelProvider  `json:"provider,omitempty" db:"-"`
}
```

### 现有数据库表结构

```sql
CREATE TABLE IF NOT EXISTS quka_model_config (
    id VARCHAR(64) PRIMARY KEY,
    provider_id VARCHAR(64) NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    model_type VARCHAR(50) NOT NULL,
    is_multi_modal BOOLEAN NOT NULL DEFAULT FALSE,
    status INTEGER NOT NULL DEFAULT 1,
    config JSONB,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);
```

## 改造目标

1. **增加思考功能支持字段**：为 chat 类型的模型添加思考功能支持类型
2. **增强查询过滤功能**：支持按思考功能需求筛选模型
3. **完善 API 接口**：在模型配置和使用配置中支持思考功能
4. **保持向后兼容**：确保现有功能不受影响

## 详细实施方案

### 1. 数据库结构修改

#### 1.1 添加思考支持字段

在 `quka_model_config` 表中添加新字段：

```sql
-- 思考功能支持类型：0-不支持，1-可选，2-强制
ALTER TABLE quka_model_config
ADD COLUMN thinking_support INTEGER NOT NULL DEFAULT 0;

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_quka_model_config_thinking_support
ON quka_model_config (thinking_support);

-- 创建复合索引用于优化按模型类型和思考支持查询
CREATE INDEX IF NOT EXISTS idx_quka_model_config_type_thinking
ON quka_model_config (model_type, thinking_support)
WHERE model_type = 'chat';
```

#### 1.2 思考支持类型常量定义

```go
// 思考功能支持类型常量
const (
    ThinkingSupportNone     = 0 // 不支持思考
    ThinkingSupportOptional = 1 // 可选思考
    ThinkingSupportForced   = 2 // 强制思考
)
```

### 2. Go 结构体修改

#### 2.1 ModelConfig 结构体增强

```go
type ModelConfig struct {
    ID             string          `json:"id" db:"id"`
    ProviderID     string          `json:"provider_id" db:"provider_id"`
    ModelName      string          `json:"model_name" db:"model_name"`
    DisplayName    string          `json:"display_name" db:"display_name"`
    ModelType      string          `json:"model_type" db:"model_type"`
    IsMultiModal   bool            `json:"is_multi_modal" db:"is_multi_modal"`
    ThinkingSupport int            `json:"thinking_support" db:"thinking_support"` // 新增：思考功能支持类型
    Status         int             `json:"status" db:"status"`
    Config         json.RawMessage `json:"config" db:"config"`
    CreatedAt      int64           `json:"created_at" db:"created_at"`
    UpdatedAt      int64           `json:"updated_at" db:"updated_at"`
    Provider       *ModelProvider  `json:"provider,omitempty" db:"-"`
}
```

#### 2.2 查询选项增强

```go
type ListModelConfigOptions struct {
    ProviderID      string
    ModelType       string
    IsMultiModal    *bool
    ThinkingSupport *int  // 新增：思考功能支持过滤
    ThinkingRequired *bool // 新增：是否需要思考功能（用于筛选支持思考的模型）
    Status          *int
    ModelName       string
}

func (opt ListModelConfigOptions) Apply(query *sq.SelectBuilder) {
    if opt.ProviderID != "" {
        *query = query.Where(sq.Eq{"provider_id": opt.ProviderID})
    }
    if opt.ModelType != "" {
        *query = query.Where(sq.Eq{"model_type": opt.ModelType})
    }
    if opt.IsMultiModal != nil {
        *query = query.Where(sq.Eq{"is_multi_modal": *opt.IsMultiModal})
    }
    if opt.ThinkingSupport != nil {
        *query = query.Where(sq.Eq{"thinking_support": *opt.ThinkingSupport})
    }
    if opt.ThinkingRequired != nil {
        if *opt.ThinkingRequired {
            // 筛选支持思考的模型（可选或强制）
            *query = query.Where(sq.Gt{"thinking_support": ThinkingSupportNone})
        } else {
            // 筛选不需要思考的模型（不支持或可选）
            *query = query.Where(sq.Lt{"thinking_support": ThinkingSupportForced})
        }
    }
    if opt.Status != nil {
        *query = query.Where(sq.Eq{"status": *opt.Status})
    }
    if opt.ModelName != "" {
        *query = query.Where(sq.Like{"model_name": "%" + opt.ModelName + "%"})
    }
}
```

### 3. 数据库操作层修改

#### 3.1 ModelConfigStore 修改

更新 SQL 操作中的字段列表：

```go
func NewModelConfigStore(provider SqlProviderAchieve) *ModelConfigStore {
    repo := &ModelConfigStore{}
    repo.SetProvider(provider)
    repo.SetTable(types.TABLE_MODEL_CONFIG)
    // 添加thinking_support字段
    repo.SetAllColumns("id", "provider_id", "model_name", "display_name",
                       "model_type", "is_multi_modal", "thinking_support",
                       "status", "config", "created_at", "updated_at")
    return repo
}
```

#### 3.2 CRUD 操作更新

在 Create、Update 等方法中添加 thinking_support 字段的处理。

### 4. 业务逻辑层修改

#### 4.1 ModelConfigLogic 增强

```go
// 新增：根据思考需求获取可用模型
func (l *ModelConfigLogic) GetAvailableThinkingModels(needsThinking bool) ([]*types.ModelConfig, error) {
    opts := types.ListModelConfigOptions{
        ModelType:        types.ModelTypeChat,
        Status:           &[]int{types.StatusEnabled}[0],
        ThinkingRequired: &needsThinking,
    }
    return l.ListModelsWithProvider(""), nil
}

// 新增：验证模型的思考功能配置
func (l *ModelConfigLogic) ValidateThinkingConfig(modelID string, enableThinking bool) error {
    model, err := l.GetModel(modelID)
    if err != nil {
        return err
    }

    // 验证思考配置是否合法
    switch model.ThinkingSupport {
    case types.ThinkingSupportNone:
        if enableThinking {
            return errors.New("model does not support thinking", i18n.ERROR_MODEL_THINKING_NOT_SUPPORTED, nil)
        }
    case types.ThinkingSupportForced:
        if !enableThinking {
            return errors.New("model requires thinking to be enabled", i18n.ERROR_MODEL_THINKING_REQUIRED, nil)
        }
    case types.ThinkingSupportOptional:
        // 可选，无需验证
    }

    return nil
}
```

### 5. API 接口修改

#### 5.1 模型配置 API 增强

```go
// CreateModelRequest 创建模型请求增强
type CreateModelRequest struct {
    ProviderID      string `json:"provider_id" binding:"required"`
    ModelName       string `json:"model_name" binding:"required"`
    DisplayName     string `json:"display_name" binding:"required"`
    ModelType       string `json:"model_type" binding:"required"`
    IsMultiModal    bool   `json:"is_multi_modal"`
    ThinkingSupport int    `json:"thinking_support"` // 新增
    Status          int    `json:"status"`
    Config          string `json:"config"`
}

// GetAvailableModels API增强
func (s *HttpSrv) GetAvailableModels(c *gin.Context) {
    modelType := c.Query("model_type")
    isMultiModalStr := c.Query("is_multi_modal")
    thinkingRequiredStr := c.Query("thinking_required") // 新增

    var isMultiModal *bool
    if isMultiModalStr != "" {
        val := isMultiModalStr == "true"
        isMultiModal = &val
    }

    var thinkingRequired *bool // 新增
    if thinkingRequiredStr != "" {
        val := thinkingRequiredStr == "true"
        thinkingRequired = &val
    }

    logic := v1.NewModelConfigLogic(c, s.Core)
    models, err := logic.GetAvailableModels(modelType, isMultiModal, thinkingRequired)
    if err != nil {
        response.APIError(c, err)
        return
    }

    response.APISuccess(c, models)
}
```

#### 5.2 AI 使用配置 API 增强

```go
// AIUsageRequest 增加思考模型配置
type AIUsageRequest struct {
    Chat           string `json:"chat" binding:"required"`
    ChatThinking   string `json:"chat_thinking,omitempty"`   // 新增：思考聊天模型
    Embedding      string `json:"embedding" binding:"required"`
    Vision         string `json:"vision,omitempty"`
    Rerank         string `json:"rerank,omitempty"`
    Reader         string `json:"reader,omitempty"`
    Enhance        string `json:"enhance,omitempty"`
}
```

### 6. 配置常量增强

#### 6.1 AI 使用配置常量

```go
const (
    // 现有常量
    AI_USAGE_CHAT      = "ai_usage_chat"
    AI_USAGE_EMBEDDING = "ai_usage_embedding"
    AI_USAGE_VISION    = "ai_usage_vision"
    AI_USAGE_RERANK    = "ai_usage_rerank"
    AI_USAGE_READER    = "ai_usage_reader"
    AI_USAGE_ENHANCE   = "ai_usage_enhance"

    // 新增：思考模型配置
    AI_USAGE_CHAT_THINKING = "ai_usage_chat_thinking"

    // 描述常量
    AI_USAGE_CHAT_THINKING_DESC = "思考聊天模型配置"
)
```

### 7. 国际化支持

#### 7.1 错误信息常量

```go
const (
    // 现有错误码...

    // 新增：思考功能相关错误
    ERROR_MODEL_THINKING_NOT_SUPPORTED = "error.model.thinking.not_supported"
    ERROR_MODEL_THINKING_REQUIRED      = "error.model.thinking.required"
    ERROR_AI_THINKING_MODEL_NOT_FOUND  = "error.ai.thinking_model.not_found"
)
```

## 实施时间线

### 阶段 1：数据库和结构体修改（2 小时）

1. 修改数据库表结构，添加 thinking_support 字段
2. 更新 ModelConfig 结构体和相关常量定义
3. 修改 ListModelConfigOptions 查询选项

### 阶段 2：数据库操作层修改（1 小时）

1. 更新 ModelConfigStore 的字段列表
2. 修改 Create、Update 等 CRUD 操作

### 阶段 3：业务逻辑层修改（2 小时）

1. 增强 ModelConfigLogic，添加思考功能相关方法
2. 添加思考配置验证逻辑
3. 更新现有查询方法支持思考功能过滤

### 阶段 4：API 接口修改（2 小时）

1. 修改模型配置相关 API 接口
2. 增强 AI 使用配置 API
3. 添加思考模型配置支持

### 阶段 5：测试和验证（1 小时）

1. 单元测试覆盖新增功能
2. 集成测试验证端到端功能
3. 向后兼容性测试

## 关键考虑点

### 1. 向后兼容性

- 新字段添加默认值，确保现有数据不受影响
- API 接口保持现有参数的兼容性
- 渐进式迁移，避免破坏性改动

### 2. 数据迁移

- 为现有的 chat 模型设置合适的 thinking_support 默认值
- 根据模型名称或 provider 判断思考支持类型

### 3. 性能考虑

- 添加合适的数据库索引优化查询
- 考虑缓存策略减少数据库查询

### 4. 错误处理

- 完善的错误消息和国际化支持
- 详细的参数验证和错误反馈

## 需要确认的问题

1. **思考功能支持类型的判断标准**：如何确定现有模型应该设置为哪种思考支持类型？
2. **默认行为**：当用户没有明确指定思考需求时，系统应该如何选择模型？
3. **配置优先级**：如果用户同时配置了普通聊天模型和思考聊天模型，系统应该如何选择？
4. **API 版本控制**：是否需要新增 API 版本来支持新功能，还是直接扩展现有接口？

## 相关文件列表

### 需要修改的文件

1. `/app/store/sqlstore/model_config.sql` - 数据库表结构
2. `/pkg/types/model_provider.go` - 数据结构定义
3. `/app/store/sqlstore/model_config.go` - 数据库操作层
4. `/app/logic/v1/model_config.go` - 业务逻辑层
5. `/cmd/service/handler/model_config.go` - API 处理器
6. `/cmd/service/handler/ai_system.go` - AI 系统配置 API
7. `/pkg/types/constant.go` - 常量定义
8. `/pkg/i18n/constant.go` - 国际化常量

### 可能需要修改的文件

1. `/app/core/srv/ai.go` - AI 服务核心逻辑
2. `/app/logic/v1/auto_assistant.go` - 自动助手逻辑
3. 相关的测试文件

---

## 状态追踪

- [ ] 数据库结构修改
- [ ] Go 结构体和常量定义
- [ ] 数据库操作层实现
- [ ] 业务逻辑层实现
- [ ] API 接口实现
- [ ] 测试用例编写
- [ ] 文档更新
