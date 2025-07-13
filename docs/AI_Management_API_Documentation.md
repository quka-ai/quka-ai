# AI 管理接口文档

## 变更日志

### v2.0.0 - Reader功能重构
- **重大变更**: Reader功能从模型类型重构为厂商特有功能
- **新增**: ModelProvider.config.is_reader字段，用于标识厂商是否支持Reader功能
- **新增**: ModelProvider.config.timeout和max_retries字段
- **新增**: 提供商列表API支持`is_reader`查询参数
- **新增**: 模型列表API自动包含Reader虚拟模型，统一前端处理体验
- **恢复**: MODEL_TYPE_READER模型类型（用于Reader虚拟模型标识）
- **变更**: AI使用配置中的reader字段现在指向provider_id而不是model_id
- **简化**: Reader虚拟模型ID直接使用provider_id，简化系统逻辑

## 概述

这是 QukaAI 项目的 AI 管理接口文档，包含模型提供商管理、模型配置管理和 AI 系统管理功能。这些接口用于 Web 界面的后端管理功能，支持动态配置管理和热重载。

## 基础信息

- **基础路径**: `/api/v1/admin`
- **认证方式**: JWT Token + 管理员权限
- **Content-Type**: `application/json`

## 接口分类

### 1. 模型提供商管理 (Model Providers)

#### 1.1 创建模型提供商

**接口**: `POST /admin/model/providers`

**功能**: 创建新的AI模型提供商配置

**请求体**:
```json
{
  "name": "Jina",
  "description": "Jina Reader API提供商",
  "api_url": "https://r.jina.ai",
  "api_key": "jina_xxx...",
  "config": {
    "is_reader": true,
    "timeout": 30,
    "max_retries": 3
  }
}
```

**字段说明**:
- `name` (string, required): 提供商名称，必须唯一
- `description` (string): 提供商描述
- `api_url` (string, required): API 基础地址
- `api_key` (string, required): API 密钥
- `config` (object): 额外配置参数
  - `is_reader` (bool): 是否支持Reader功能（厂商特有功能）
  - `timeout` (int): 请求超时时间（秒）
  - `max_retries` (int): 最大重试次数

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "provider_123",
    "name": "Jina",
    "description": "Jina Reader API提供商",
    "api_url": "https://r.jina.ai",
    "status": 1,
    "config": {
      "is_reader": true,
      "timeout": 30,
      "max_retries": 3
    },
    "created_at": 1640995200,
    "updated_at": 1640995200
  }
}
```

#### 1.2 获取提供商列表

**接口**: `GET /admin/model/providers`

**功能**: 获取所有模型提供商列表

**查询参数**:
- `page` (int): 页码，默认 1
- `limit` (int): 每页条数，默认 20
- `name` (string): 按名称过滤
- `status` (int): 按状态过滤 (0=禁用, 1=启用)
- `is_reader` (bool): 按Reader功能过滤 (true=支持Reader, false=不支持Reader)

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "list": [
      {
        "id": "provider_123",
        "name": "OpenAI",
        "description": "OpenAI API提供商",
        "api_url": "https://api.openai.com/v1",
        "status": 1,
        "created_at": 1640995200,
        "updated_at": 1640995200
      }
    ],
    "total": 1,
    "page": 1,
    "limit": 20
  }
}
```

#### 1.3 获取提供商详情

**接口**: `GET /admin/model/providers/:id`

**功能**: 获取指定提供商的详细信息

**路径参数**:
- `id` (string): 提供商ID

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "provider_123",
    "name": "OpenAI",
    "description": "OpenAI API提供商",
    "api_url": "https://api.openai.com/v1",
    "status": 1,
    "config": {
      "timeout": 30,
      "max_retries": 3
    },
    "created_at": 1640995200,
    "updated_at": 1640995200
  }
}
```

#### 1.4 更新提供商信息

**接口**: `PUT /admin/model/providers/:id`

**功能**: 更新指定提供商的信息

**路径参数**:
- `id` (string): 提供商ID

**请求体**:
```json
{
  "name": "OpenAI Updated",
  "description": "更新后的OpenAI API提供商",
  "api_url": "https://api.openai.com/v1",
  "api_key": "sk-new-key...",
  "status": 1,
  "config": {
    "timeout": 60,
    "max_retries": 5
  }
}
```

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "provider_123",
    "name": "OpenAI Updated",
    "description": "更新后的OpenAI API提供商",
    "api_url": "https://api.openai.com/v1",
    "status": 1,
    "config": {
      "timeout": 60,
      "max_retries": 5
    },
    "created_at": 1640995200,
    "updated_at": 1640995260
  }
}
```

#### 1.5 删除提供商

**接口**: `DELETE /admin/model/providers/:id`

**功能**: 删除指定的模型提供商

**路径参数**:
- `id` (string): 提供商ID

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "message": "Provider deleted successfully"
  }
}
```

### 2. 模型配置管理 (Model Configs)

#### 2.1 创建模型配置

**接口**: `POST /admin/model/configs`

**功能**: 为提供商创建具体的模型配置

**请求体**:
```json
{
  "provider_id": "provider_123",
  "model_name": "gpt-4",
  "display_name": "GPT-4",
  "model_type": "chat",
  "is_multi_modal": true,
  "config": {
    "max_tokens": 4096,
    "temperature": 0.7,
    "top_p": 1.0
  }
}
```

**字段说明**:
- `provider_id` (string, required): 提供商ID
- `model_name` (string, required): 模型名称
- `display_name` (string): 显示名称
- `model_type` (string, required): 模型类型 (chat/embedding/vision/rerank/enhance)
- `is_multi_modal` (bool): 是否支持多模态
- `config` (object): 模型配置参数

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "config_456",
    "provider_id": "provider_123",
    "model_name": "gpt-4",
    "display_name": "GPT-4",
    "model_type": "chat",
    "is_multi_modal": true,
    "status": 1,
    "config": {
      "max_tokens": 4096,
      "temperature": 0.7,
      "top_p": 1.0
    },
    "created_at": 1640995200,
    "updated_at": 1640995200,
    "provider": {
      "id": "provider_123",
      "name": "OpenAI"
    }
  }
}
```

#### 2.2 获取模型配置列表

**接口**: `GET /admin/model/configs`

**功能**: 获取所有模型配置列表，包含真实模型配置和Reader虚拟模型

**查询参数**:
- `page` (int): 页码，默认 1
- `limit` (int): 每页条数，默认 20
- `provider_id` (string): 按提供商ID过滤
- `model_type` (string): 按模型类型过滤（支持reader类型）
- `status` (int): 按状态过滤

**说明**: 
- 此接口会返回真实的模型配置，同时将支持Reader功能的提供商作为虚拟模型返回
- Reader虚拟模型的ID直接使用provider_id
- Reader虚拟模型的model_type为 "reader"
- Reader虚拟模型使用提供商名称作为模型名称

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "list": [
      {
        "id": "config_456",
        "provider_id": "provider_123",
        "model_name": "gpt-4",
        "display_name": "GPT-4",
        "model_type": "chat",
        "is_multi_modal": true,
        "status": 1,
        "created_at": 1640995200,
        "updated_at": 1640995200,
        "provider": {
          "id": "provider_123",
          "name": "OpenAI"
        }
      },
      {
        "id": "provider_456",
        "provider_id": "provider_456",
        "model_name": "Jina",
        "display_name": "Jina Reader",
        "model_type": "reader",
        "is_multi_modal": false,
        "status": 1,
        "created_at": 1640995200,
        "updated_at": 1640995200,
        "provider": {
          "id": "provider_456",
          "name": "Jina"
        }
      }
    ],
    "total": 2,
    "page": 1,
    "limit": 20
  }
}
```

#### 2.3 获取模型配置详情

**接口**: `GET /admin/model/configs/:id`

**功能**: 获取指定模型配置的详细信息

**路径参数**:
- `id` (string): 模型配置ID

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "config_456",
    "provider_id": "provider_123",
    "model_name": "gpt-4",
    "display_name": "GPT-4",
    "model_type": "chat",
    "is_multi_modal": true,
    "status": 1,
    "config": {
      "max_tokens": 4096,
      "temperature": 0.7,
      "top_p": 1.0
    },
    "created_at": 1640995200,
    "updated_at": 1640995200,
    "provider": {
      "id": "provider_123",
      "name": "OpenAI",
      "api_url": "https://api.openai.com/v1"
    }
  }
}
```

#### 2.4 更新模型配置

**接口**: `PUT /admin/model/configs/:id`

**功能**: 更新指定模型配置的信息

**路径参数**:
- `id` (string): 模型配置ID

**请求体**:
```json
{
  "display_name": "GPT-4 Updated",
  "model_type": "chat",
  "is_multi_modal": true,
  "status": 1,
  "config": {
    "max_tokens": 8192,
    "temperature": 0.8,
    "top_p": 0.9
  }
}
```

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "config_456",
    "provider_id": "provider_123",
    "model_name": "gpt-4",
    "display_name": "GPT-4 Updated",
    "model_type": "chat",
    "is_multi_modal": true,
    "status": 1,
    "config": {
      "max_tokens": 8192,
      "temperature": 0.8,
      "top_p": 0.9
    },
    "created_at": 1640995200,
    "updated_at": 1640995320
  }
}
```

#### 2.5 删除模型配置

**接口**: `DELETE /admin/model/configs/:id`

**功能**: 删除指定的模型配置

**路径参数**:
- `id` (string): 模型配置ID

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "message": "Model config deleted successfully"
  }
}
```

### 3. AI 系统管理 (AI System)

#### 3.1 重新加载 AI 配置

**接口**: `POST /admin/ai/system/reload`

**功能**: 重新加载 AI 系统配置，实现热重载

**请求体**: 无

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "message": "AI配置重载成功",
    "time": 1640995200
  }
}
```

#### 3.2 获取 AI 系统状态

**接口**: `GET /admin/ai/system/status`

**功能**: 获取 AI 系统当前状态信息

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "chat_drivers_count": 3,
    "embed_drivers_count": 2,
    "vision_drivers_count": 1,
    "rerank_drivers_count": 1,
    "reader_drivers_count": 1,
    "enhance_drivers_count": 1,
    "last_reload_time": 1640995200
  }
}
```

#### 3.3 更新 AI 使用配置

**接口**: `PUT /admin/ai/system/usage`

**功能**: 更新 AI 各功能模块使用的模型配置

**请求体**:
```json
{
  "chat": "config_456",
  "embedding": "config_789",
  "vision": "config_101",
  "rerank": "config_112",
  "reader": "provider_123",
  "enhance": "config_415"
}
```

**字段说明**:
- `chat` (string, required): 聊天功能使用的模型配置ID
- `embedding` (string, required): 向量化功能使用的模型配置ID
- `vision` (string): 视觉功能使用的模型配置ID
- `rerank` (string): 重排序功能使用的模型配置ID
- `reader` (string): 阅读功能使用的提供商ID（注意：这里是provider_id，不是model_id）
- `enhance` (string): 增强功能使用的模型配置ID

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "message": "AI使用配置更新成功",
    "configs": [
      {
        "name": "ai_usage_chat",
        "category": "ai_usage",
        "value": "\"config_456\"",
        "description": "聊天功能使用的模型",
        "status": 1
      },
      {
        "name": "ai_usage_embedding",
        "category": "ai_usage",
        "value": "\"config_789\"",
        "description": "向量化功能使用的模型",
        "status": 1
      }
    ]
  }
}
```

#### 3.4 获取 AI 使用配置

**接口**: `GET /admin/ai/system/usage`

**功能**: 获取当前 AI 各功能模块的使用配置

**响应示例**:
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "chat": "config_456",
    "embedding": "config_789",
    "vision": "config_101",
    "rerank": "config_112",
    "reader": "provider_123",
    "enhance": "config_415"
  }
}
```

## 错误代码

| 错误代码 | HTTP状态码 | 说明 |
|---------|-----------|------|
| 200 | 200 | 成功 |
| 400 | 400 | 请求参数错误 |
| 401 | 401 | 未经授权 |
| 403 | 403 | 权限不足 |
| 404 | 404 | 资源不存在 |
| 409 | 409 | 资源冲突 |
| 500 | 500 | 服务器内部错误 |

## 错误响应格式

```json
{
  "code": 400,
  "message": "error.invalidargument",
  "data": null
}
```

## 认证说明

所有接口都需要在请求头中包含 JWT Token：

```
Authorization: Bearer <JWT_TOKEN>
```

用户必须具有管理员权限才能访问这些接口。

## 支持的 AI 服务提供商

- OpenAI
- Azure OpenAI
- Qwen (通义千问)
- Baishan (白山)
- 其他兼容 OpenAI API 的服务

## 模型类型说明

- `chat`: 聊天对话模型
- `embedding`: 文本向量化模型
- `vision`: 视觉理解模型
- `rerank`: 文档重排序模型
- `enhance`: 内容增强模型

## 厂商特有功能说明

- `Reader功能`: 文档阅读功能是厂商特有的能力，不属于模型类型。通过ModelProvider的config.is_reader字段标识该厂商是否支持Reader功能。

## 使用示例

### 创建支持Reader功能的Jina提供商
```bash
curl -X POST "https://api.example.com/api/v1/admin/model/providers" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Jina",
    "description": "Jina Reader API提供商",
    "api_url": "https://r.jina.ai",
    "api_key": "jina_xxx...",
    "config": {
      "is_reader": true,
      "timeout": 30,
      "max_retries": 3
    }
  }'
```

### 查询支持Reader功能的提供商
```bash
curl -X GET "https://api.example.com/api/v1/admin/model/providers?is_reader=true&status=1" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### 创建 GPT-4 模型配置
```bash
curl -X POST "https://api.example.com/api/v1/admin/model/configs" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider_id": "provider_123",
    "model_name": "gpt-4",
    "display_name": "GPT-4",
    "model_type": "chat",
    "is_multi_modal": true,
    "config": {
      "max_tokens": 4096,
      "temperature": 0.7
    }
  }'
```

### 更新 AI 使用配置
```bash
curl -X PUT "https://api.example.com/api/v1/admin/ai/system/usage" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "chat": "config_456",
    "embedding": "config_789",
    "reader": "provider_123"
  }'
```

**注意**: Reader配置使用的是provider_id，而其他功能使用的是model_id。

## 前端开发指南

基于此 API 文档，您可以使用 React 构建以下管理界面：

### 1. 提供商管理页面
- 提供商列表展示（支持按Reader功能筛选）
- 创建/编辑提供商表单（包含Reader功能配置）
- 提供商状态切换
- 删除确认对话框
- Reader功能标识展示

### 2. 模型配置管理页面
- 模型配置列表展示
- 按提供商分组展示
- 创建/编辑模型配置表单
- 模型类型选择器

### 3. AI 系统管理页面
- 系统状态仪表板
- AI 使用配置设置
- 配置重载按钮
- 实时状态监控

### 4. 推荐的 React 组件结构
```
src/
├── components/
│   ├── providers/
│   │   ├── ProviderList.jsx
│   │   ├── ProviderForm.jsx
│   │   └── ProviderCard.jsx
│   └── models/
│       ├── ModelList.jsx
│       ├── ModelForm.jsx
│       ├── ModelUsageSetting.jsx
│       └── ModelCard.jsx
├── pages/
│   ├── ProvidersPage.jsx
│   ├── ModelsPage.jsx
│   └── ModelUsagePage.jsx
├── hooks/
│   ├── useProviders.js
│   ├── useModels.js
│   └── useSystem.js
└── api/
    ├── providers.js
    ├── models.js
    └── system.js
```

这个 API 文档提供了完整的接口规范，可以直接用于前端开发和后端对接。