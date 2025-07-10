Q：下面我们来一起设计一下关于LLM 
  API的配置与初始化的功能,你可以先阅读一下app.core.srv目录下的ai.go中的代码, AI struct 是整个项目LLM API 
  Provider配置的基础结构,之前我是通过toml文件配置的providers,现在我想要用户可以通过web界面进行配置,所以在
  app.store.sqlstore中实现了两个关于存储Provider信息与模型信息的db操作: model_config.go 和 
  model_provider.go,你先阅读一下这些关联的文件,然后设计一下如何实现用户通过WEB 
  API对两个配置表进行增删改,然后再将db中的数据加载到AI 
  struct中供整个系统进行调用访问,先出方案,不着急写代码

A：
# LLM API配置与初始化功能设计文档

## 项目概述

本文档描述了QukaAI项目中LLM API配置与初始化功能的完整设计方案。该功能旨在将原有的TOML文件配置方式转换为用户友好的Web界面配置管理，支持动态配置和热重载。

## 目标与需求

### 主要目标
- 替换现有的TOML配置文件，提供Web界面配置管理
- 支持多种AI服务提供商的动态配置
- 实现配置的热重载，无需重启服务
- 提供完整的模型配置管理功能

### 功能需求
- 模型提供商的增删改查
- 模型配置的增删改查
- 使用配置的管理和切换
- 配置验证和错误处理
- 敏感信息的安全存储

## 现有架构分析

### 核心结构分析

#### AI struct (`app/core/srv/ai.go:81-102`)
```go
type AI struct {
    // 各种AI驱动器映射
    chatDrivers    map[string]ChatAI
    embedDrivers   map[string]EmbeddingAI
    enhanceDrivers map[string]ai.Enhance
    visionDrivers  map[string]VisionAI
    readerDrivers  map[string]ReaderAI
    rerankDrivers  map[string]RerankAI
    
    // 使用配置映射
    chatUsage    map[string]ChatAI
    enhanceUsage map[string]ai.Enhance
    embedUsage   map[string]EmbeddingAI
    readerUsage  map[string]ReaderAI
    visionUsage  map[string]VisionAI
    rerankUsage  map[string]RerankAI
    
    // 默认驱动器
    chatDefault    ChatAI
    enhanceDefault ai.Enhance
    embedDefault   EmbeddingAI
    readerDefault  ReaderAI
    visionDefault  VisionAI
    rerankDefault  RerankAI
}
```

#### 数据库表结构
- **ModelProvider** (`pkg/types/model_provider.go:20-31`)：存储AI服务提供商信息
- **ModelConfig** (`pkg/types/model_provider.go:33-48`)：存储具体的模型配置信息
- **现有CRUD操作**：已实现基础的数据库操作层

#### 当前配置方式
通过TOML文件配置（`cmd/service/etc/service-default.toml`）：
```toml
[ai.openai]
token = ""
endpoint = ""
embedding_model = ""
chat_model = ""

[ai.usage]
"embedding.query"=""
"query"=""
"summarize"=""
```

## 系统设计

### 整体架构图
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web前端界面   │────│   RESTful API   │────│   业务逻辑层    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
                       ┌─────────────────┐    ┌─────────────────┐
                       │   配置加载器    │────│   数据库存储层  │
                       └─────────────────┘    └─────────────────┘
                                │
                       ┌─────────────────┐
                       │   AI Core服务   │
                       └─────────────────┘
```

### 核心组件设计

#### 1. Web API层
##### 路由设计
```
# 模型提供商管理
GET    /api/v1/model/providers          # 获取提供商列表
POST   /api/v1/model/providers          # 创建提供商
GET    /api/v1/model/providers/{id}     # 获取单个提供商
PUT    /api/v1/model/providers/{id}     # 更新提供商
DELETE /api/v1/model/providers/{id}     # 删除提供商

# 模型配置管理
GET    /api/v1/model/configs             # 获取模型配置列表
POST   /api/v1/model/configs             # 创建模型配置
GET    /api/v1/model/configs/{id}        # 获取单个模型配置
PUT    /api/v1/model/configs/{id}        # 更新模型配置
DELETE /api/v1/model/configs/{id}        # 删除模型配置

# 使用配置管理
GET    /api/v1/usage/config              # 获取当前使用配置
PUT    /api/v1/usage/config              # 更新使用配置
POST   /api/v1/ai/reload                 # 重新加载AI配置
GET    /api/v1/ai/status                 # 获取AI服务状态
```

##### API响应格式
```json
{
  "code": 200,
  "message": "success",
  "data": {
    // 响应数据
  },
  "timestamp": 1640995200
}
```

#### 2. 业务逻辑层
需要在 `app/logic/v1/` 下创建以下文件：

##### `model_provider.go`
```go
type ModelProviderLogic struct {
    store *sqlstore.ModelProviderStore
}

func (l *ModelProviderLogic) Create(ctx context.Context, req CreateModelProviderRequest) error
func (l *ModelProviderLogic) Get(ctx context.Context, id string) (*types.ModelProvider, error)
func (l *ModelProviderLogic) Update(ctx context.Context, id string, req UpdateModelProviderRequest) error
func (l *ModelProviderLogic) Delete(ctx context.Context, id string) error
func (l *ModelProviderLogic) List(ctx context.Context, opts ListModelProviderRequest) ([]types.ModelProvider, int64, error)
func (l *ModelProviderLogic) ValidateConfig(ctx context.Context, provider *types.ModelProvider) error
```

##### `model_config.go`
```go
type ModelConfigLogic struct {
    store         *sqlstore.ModelConfigStore
    providerStore *sqlstore.ModelProviderStore
}

func (l *ModelConfigLogic) Create(ctx context.Context, req CreateModelConfigRequest) error
func (l *ModelConfigLogic) Get(ctx context.Context, id string) (*types.ModelConfig, error)
func (l *ModelConfigLogic) Update(ctx context.Context, id string, req UpdateModelConfigRequest) error
func (l *ModelConfigLogic) Delete(ctx context.Context, id string) error
func (l *ModelConfigLogic) List(ctx context.Context, opts ListModelConfigRequest) ([]types.ModelConfig, int64, error)
func (l *ModelConfigLogic) ValidateModelConfig(ctx context.Context, config *types.ModelConfig) error
```

##### `ai_config.go`
```go
type AIConfigLogic struct {
    loader        *AIConfigLoader
    usageStore    *sqlstore.UsageConfigStore
    configStore   *sqlstore.ModelConfigStore
    providerStore *sqlstore.ModelProviderStore
}

func (l *AIConfigLogic) GetUsageConfig(ctx context.Context) (*UsageConfig, error)
func (l *AIConfigLogic) UpdateUsageConfig(ctx context.Context, req UpdateUsageConfigRequest) error
func (l *AIConfigLogic) ReloadAIConfig(ctx context.Context) error
func (l *AIConfigLogic) GetAIStatus(ctx context.Context) (*AIStatus, error)
```

#### 3. 配置加载器
在 `app/core/srv/` 下创建 `ai_loader.go`：

```go
type AIConfigLoader struct {
    providerStore *sqlstore.ModelProviderStore
    configStore   *sqlstore.ModelConfigStore
    usageStore    *sqlstore.UsageConfigStore
}

func NewAIConfigLoader(stores *sqlstore.Stores) *AIConfigLoader {
    return &AIConfigLoader{
        providerStore: stores.ModelProviderStore,
        configStore:   stores.ModelConfigStore,
        usageStore:    stores.UsageConfigStore,
    }
}

func (l *AIConfigLoader) LoadFromDatabase(ctx context.Context) (*AI, error) {
    // 1. 从数据库加载启用的提供商
    providers, err := l.providerStore.List(ctx, types.ListModelProviderOptions{
        Status: &[]int{types.StatusEnabled}[0],
    }, 0, 0)
    if err != nil {
        return nil, err
    }

    // 2. 从数据库加载启用的模型配置
    configs, err := l.configStore.ListWithProvider(ctx, types.ListModelConfigOptions{
        Status: &[]int{types.StatusEnabled}[0],
    })
    if err != nil {
        return nil, err
    }

    // 3. 从数据库加载使用配置
    usageConfig, err := l.usageStore.GetDefault(ctx)
    if err != nil {
        return nil, err
    }

    // 4. 构建AI实例
    return l.buildAI(ctx, configs, usageConfig)
}

func (l *AIConfigLoader) buildAI(ctx context.Context, configs []types.ModelConfig, usage *UsageConfig) (*AI, error) {
    // 构建AI实例的具体逻辑
    // 类似于现有的 SetupAI 函数，但从数据库配置构建
}
```

#### 4. 数据库扩展
需要新增使用配置表：

```sql
CREATE TABLE usage_config (
    id VARCHAR(50) PRIMARY KEY DEFAULT 'default',
    chat VARCHAR(50),
    embedding VARCHAR(50),
    vision VARCHAR(50),
    rerank VARCHAR(50),
    reader VARCHAR(50),
    enhance VARCHAR(50),
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);
```

对应的Go结构：
```go
type UsageConfig struct {
    ID        string `json:"id" db:"id"`
    Chat      string `json:"chat" db:"chat"`
    Embedding string `json:"embedding" db:"embedding"`
    Vision    string `json:"vision" db:"vision"`
    Rerank    string `json:"rerank" db:"rerank"`
    Reader    string `json:"reader" db:"reader"`
    Enhance   string `json:"enhance" db:"enhance"`
    CreatedAt int64  `json:"created_at" db:"created_at"`
    UpdatedAt int64  `json:"updated_at" db:"updated_at"`
}
```

## 实现流程

### 阶段1：基础功能实现
1. **创建数据库表和Store**
   - 创建 `quka_usage_config` 表
   - 实现 `UsageConfigStore` 的CRUD操作

2. **实现业务逻辑层**
   - 创建 `model_provider.go`
   - 创建 `model_config.go`
   - 创建 `ai_config.go`

3. **实现API路由**
   - 添加路由处理函数
   - 实现请求验证和响应处理

### 阶段2：配置加载与初始化
1. **创建配置加载器**
   - 实现 `AIConfigLoader`
   - 集成到服务启动流程

2. **修改现有AI初始化**
   - 扩展 `SetupAI` 函数支持数据库配置
   - 实现配置验证逻辑

### 阶段3：高级功能
1. **配置热重载**
   - 实现 `/api/v1/ai/reload` 接口
   - 支持运行时配置更新

2. **错误处理和验证**
   - 配置验证机制
   - 错误回滚策略

3. **安全和监控**
   - API Key加密存储
   - 配置变更日志
   - 性能监控

## 安全考虑

### 敏感信息处理
- API Key等敏感信息需要加密存储
- 前端不返回敏感信息
- 配置变更需要权限验证

### 配置验证
- 提供商配置的有效性验证
- 模型配置的兼容性检查
- 使用配置的完整性验证

### 错误处理
- 配置加载失败的降级策略
- 详细的错误信息记录
- 配置回滚机制

## 测试策略

### 单元测试
- 各个Store的CRUD操作测试
- 业务逻辑层的功能测试
- 配置加载器的测试

### 集成测试
- API接口的端到端测试
- 配置热重载的测试
- 错误场景的测试

### 性能测试
- 配置加载的性能测试
- 高并发场景的测试
- 内存使用情况的监控

## 监控与维护

### 监控指标
- 配置加载成功率
- API接口响应时间
- 配置变更频率
- 错误率统计

### 日志记录
- 配置变更日志
- 错误日志详情
- 性能日志记录

### 维护建议
- 定期备份配置数据
- 监控配置完整性
- 定期清理无效配置

## 总结

本设计文档提供了一个完整的LLM API配置与初始化功能实现方案，从TOML文件配置迁移到Web界面配置管理。该方案考虑了：

1. **用户友好性**：提供直观的Web界面管理
2. **系统稳定性**：支持配置验证和热重载
3. **安全性**：敏感信息加密和权限控制
4. **可扩展性**：模块化设计，易于扩展新功能
5. **可维护性**：完整的测试和监控体系

通过分阶段实施，可以逐步完成从传统配置到现代化配置管理的转变，提升系统的可用性和用户体验。