# OCR 功能配置文档

## 功能概述

QukaAI 集成了 OCR（Optical Character Recognition，光学字符识别）功能，支持从图片和PDF文件中提取文本内容并转换为Markdown格式。该功能采用提供商级别的配置模式，与现有的Reader功能配置方式保持一致。

## 技术架构

### 1. 数据模型

#### 模型提供商配置 (ModelProviderConfig)
OCR功能通过 `ModelProvider` 表的 `config` 字段进行配置：

```go
type ModelProviderConfig struct {
    IsReader   bool `json:"is_reader"`   // 是否支持Reader功能
    IsOCR      bool `json:"is_ocr"`      // 是否支持OCR功能
    Timeout    int  `json:"timeout"`     // 请求超时时间（秒）
    MaxRetries int  `json:"max_retries"` // 最大重试次数
}
```

#### OCR使用配置 (Usage)
在AI系统配置中，通过 `Usage` 结构指定当前使用的OCR提供商：

```go
type Usage struct {
    Chat         string `json:"chat"`          // 聊天模型ID
    ChatThinking string `json:"chat_thinking"` // 思考聊天模型ID
    Embedding    string `json:"embedding"`     // 向量模型ID
    Vision       string `json:"vision"`        // 视觉模型ID
    Rerank       string `json:"rerank"`        // 重排序模型ID
    Enhance      string `json:"enhance"`       // 增强模型ID
    Reader       string `json:"reader"`        // Reader提供商ID
    OCR          string `json:"ocr"`           // OCR提供商ID (新增)
}
```

### 2. 支持的提供商

目前支持的OCR提供商：

#### Baidu OCR
- **提供商名称**: `baidu`
- **支持的文件类型**: PDF、PNG、JPEG、GIF、WebP、BMP
- **输出格式**: Markdown
- **特殊功能**:
  - 自动文档方向分类
  - 文档矫正
  - 图表识别
  - 布局解析

### 3. API 接口

#### 3.1 文件上传OCR识别
```http
POST /api/v1/ocr/file
Content-Type: multipart/form-data
Authorization: Bearer <token>

参数:
- file: 文件内容 (multipart/form-data)

响应:
{
  "code": 0,
  "data": {
    "title": "文档标题",
    "markdown_text": "识别的Markdown文本内容",
    "images": ["图片URL1", "图片URL2"],
    "model": "baidu",
    "tokens_used": 1234
  }
}
```

#### 3.2 URL文件OCR识别
```http
POST /api/v1/ocr/url
Content-Type: application/json
Authorization: Bearer <token>

请求体:
{
  "file_url": "https://example.com/document.pdf"
}

响应:
{
  "code": 0,
  "data": {
    "title": "文档标题",
    "markdown_text": "识别的Markdown文本内容",
    "images": ["图片URL1", "图片URL2"],
    "model": "baidu",
    "tokens_used": 1234
  }
}
```

#### 3.3 文件下载代理
```http
GET /api/v1/ocr/download?url=<encoded_url>
Authorization: Bearer <token>

响应: 文件二进制数据
```

### 4. 限制说明

- **文件大小限制**: 最大 50MB
- **支持的文件类型**: PDF、PNG、JPEG、GIF、WebP、BMP
- **速率限制**: 通过 `aiLimit("ocr")` 中间件控制

## 前端集成指南

### 1. 模型提供商管理

#### 1.1 创建OCR提供商

```typescript
interface CreateOCRProviderRequest {
  name: string;          // 提供商名称，如 "baidu"
  description: string;   // 提供商描述
  api_url: string;       // API地址
  api_key: string;       // API密钥
  status: number;        // 状态: 1-启用, 0-禁用
  config: {
    is_ocr: true;        // 必须设置为true
    timeout?: number;    // 超时时间（秒），可选
    max_retries?: number; // 最大重试次数，可选
  };
}

// API调用示例
const createProvider = async (data: CreateOCRProviderRequest) => {
  const response = await fetch('/api/v1/model-providers', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify(data)
  });
  return response.json();
};
```

#### 1.2 获取OCR提供商列表

```typescript
interface ListOCRProvidersParams {
  status?: number;      // 筛选状态
  name?: string;        // 筛选名称
  is_ocr?: boolean;     // 只获取OCR提供商
}

// API调用示例
const listOCRProviders = async (params: ListOCRProvidersParams) => {
  const query = new URLSearchParams({
    ...params,
    is_ocr: 'true'  // 过滤OCR提供商
  });

  const response = await fetch(`/api/v1/model-providers?${query}`, {
    headers: {
      'Authorization': `Bearer ${token}`
    }
  });
  return response.json();
};
```

#### 1.3 更新OCR提供商

```typescript
interface UpdateOCRProviderRequest {
  description?: string;
  api_url?: string;
  api_key?: string;
  status?: number;
  config?: {
    is_ocr?: boolean;
    timeout?: number;
    max_retries?: number;
  };
}

const updateProvider = async (id: string, data: UpdateOCRProviderRequest) => {
  const response = await fetch(`/api/v1/model-providers/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify(data)
  });
  return response.json();
};
```

### 2. OCR使用配置

#### 2.1 获取当前OCR配置

```typescript
// 获取AI系统配置
const getAISystemConfig = async () => {
  const response = await fetch('/api/v1/system/ai', {
    headers: {
      'Authorization': `Bearer ${token}`
    }
  });
  const data = await response.json();

  // data.usage.ocr 包含当前使用的OCR提供商ID
  return data;
};
```

#### 2.2 更新OCR使用配置

```typescript
interface UpdateAISystemConfigRequest {
  usage: {
    chat?: string;
    chat_thinking?: string;
    embedding?: string;
    vision?: string;
    rerank?: string;
    enhance?: string;
    reader?: string;
    ocr?: string;  // OCR提供商ID
  };
}

const updateOCRConfig = async (ocrProviderId: string) => {
  const response = await fetch('/api/v1/system/ai', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify({
      usage: {
        ocr: ocrProviderId  // 设置使用的OCR提供商ID
      }
    })
  });
  return response.json();
};
```

### 3. OCR功能调用

#### 3.1 文件上传OCR

```typescript
const processOCRFromFile = async (file: File) => {
  const formData = new FormData();
  formData.append('file', file);

  const response = await fetch('/api/v1/ocr/file', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`
    },
    body: formData
  });

  return response.json();
};

// React组件示例
const OCRUploader = () => {
  const [result, setResult] = useState<OCRResult | null>(null);
  const [loading, setLoading] = useState(false);

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    // 检查文件大小
    if (file.size > 50 * 1024 * 1024) {
      alert('文件大小不能超过50MB');
      return;
    }

    setLoading(true);
    try {
      const data = await processOCRFromFile(file);
      setResult(data.data);
    } catch (error) {
      console.error('OCR识别失败:', error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <input type="file" accept=".pdf,.png,.jpg,.jpeg" onChange={handleFileUpload} />
      {loading && <p>识别中...</p>}
      {result && (
        <div>
          <h3>{result.title}</h3>
          <div dangerouslySetInnerHTML={{ __html: marked(result.markdown_text) }} />
        </div>
      )}
    </div>
  );
};
```

#### 3.2 URL文件OCR

```typescript
const processOCRFromURL = async (fileUrl: string) => {
  const response = await fetch('/api/v1/ocr/url', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify({
      file_url: fileUrl
    })
  });

  return response.json();
};
```

### 4. UI组件设计建议

#### 4.1 OCR提供商配置页面

页面应包含以下功能：

1. **提供商列表**
   - 显示所有已配置的OCR提供商
   - 状态标识（启用/禁用）
   - 操作按钮（编辑、删除、测试）

2. **添加提供商表单**
   ```
   - 提供商名称: [下拉选择: Baidu]
   - 描述: [文本输入]
   - API地址: [文本输入]
   - API密钥: [密码输入]
   - 超时时间(秒): [数字输入, 默认: 30]
   - 最大重试次数: [数字输入, 默认: 3]
   - 状态: [开关]
   ```

3. **当前使用配置**
   - 显示当前使用的OCR提供商
   - 快速切换提供商

#### 4.2 OCR功能使用界面

1. **上传区域**
   - 支持拖拽上传
   - 文件类型提示：PDF、PNG、JPG等
   - 文件大小限制提示：最大50MB

2. **识别结果展示**
   - 文档标题
   - Markdown文本预览（带渲染）
   - 提取的图片列表
   - Token使用统计

3. **操作按钮**
   - 下载Markdown文件
   - 复制文本
   - 保存到知识库

### 5. 数据类型定义

```typescript
// OCR提供商
interface OCRProvider {
  id: string;
  name: string;
  description: string;
  api_url: string;
  status: number;  // 1-启用, 0-禁用
  config: {
    is_ocr: boolean;
    timeout?: number;
    max_retries?: number;
  };
  created_at: number;
  updated_at: number;
}

// OCR识别结果
interface OCRResult {
  title: string;
  markdown_text: string;
  images: string[];
  model: string;
  tokens_used?: number;
}

// AI系统配置
interface AISystemConfig {
  usage: {
    chat: string;
    chat_thinking: string;
    embedding: string;
    vision: string;
    rerank: string;
    enhance: string;
    reader: string;
    ocr: string;  // 当前使用的OCR提供商ID
  };
}
```

## 配置示例

### 完整配置流程

#### 1. 创建Baidu OCR提供商
```json
{
  "name": "baidu",
  "description": "百度OCR服务",
  "api_url": "https://aip.baidubce.com/rest/2.0/ocr/v1/doc_analysis",
  "api_key": "your-baidu-api-key",
  "status": 1,
  "config": {
    "is_ocr": true,
    "timeout": 60,
    "max_retries": 3
  }
}
```

#### 2. 设置为当前使用的OCR提供商
```json
{
  "usage": {
    "ocr": "provider-id-from-step-1"
  }
}
```

#### 3. 调用OCR识别
```bash
# 文件上传方式
curl -X POST https://api.example.com/api/v1/ocr/file \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "file=@document.pdf"

# URL方式
curl -X POST https://api.example.com/api/v1/ocr/url \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"file_url": "https://example.com/document.pdf"}'
```

## 错误处理

### 常见错误码

| 错误码 | 说明 | 处理建议 |
|--------|------|----------|
| `ERROR_UNSUPPORTED_FEATURE` | OCR功能未配置 | 请先配置OCR提供商 |
| `ERROR_INVALIDARGUMENT` | 参数错误 | 检查请求参数是否正确 |
| `FileSizeLimit` | 文件大小超限 | 文件不能超过50MB |
| `EmptyURL` | URL为空 | 请提供有效的文件URL |
| `DownloadFile` | 文件下载失败 | 检查URL是否可访问 |
| `ProcessOCR` | OCR处理失败 | 检查文件格式或提供商配置 |

### 错误处理示例

```typescript
try {
  const result = await processOCRFromFile(file);
  if (result.code !== 0) {
    switch (result.error?.code) {
      case 'ERROR_UNSUPPORTED_FEATURE':
        showError('OCR功能未配置，请联系管理员');
        break;
      case 'FileSizeLimit':
        showError('文件大小不能超过50MB');
        break;
      default:
        showError(result.error?.message || '识别失败');
    }
  } else {
    handleSuccess(result.data);
  }
} catch (error) {
  showError('网络错误，请稍后重试');
}
```

## 性能优化建议

### 1. 前端优化
- 使用文件预处理，压缩大图片
- 添加上传进度提示
- 实现断点续传（大文件）
- 缓存识别结果

### 2. 后端优化
- 异步处理大文件
- 实现任务队列
- 添加结果缓存
- 限流保护

## 安全考虑

1. **API密钥保护**: API密钥不返回给前端
2. **文件大小限制**: 防止资源耗尽
3. **速率限制**: 防止滥用
4. **文件类型验证**: 只允许指定类型
5. **URL白名单**: 限制可访问的URL域名（可选）

## 扩展计划

未来可能支持的功能：
- [ ] 更多OCR提供商（阿里云、腾讯云等）
- [ ] 批量处理
- [ ] 异步任务模式
- [ ] OCR结果后处理（去水印、格式优化等）
- [ ] 多语言支持
- [ ] 手写识别
- [ ] 表格结构化识别

## 相关文件

### 后端代码
- [app/core/srv/ai.go:522](app/core/srv/ai.go#L522) - OCR设置函数
- [pkg/ai/baidu/ocr.go](pkg/ai/baidu/ocr.go) - Baidu OCR实现
- [cmd/service/handler/ocr.go](cmd/service/handler/ocr.go) - OCR HTTP处理器
- [pkg/types/model_provider.go:36](pkg/types/model_provider.go#L36) - ModelProviderConfig定义

### 数据库
- [app/store/sqlstore/model_provider.sql](app/store/sqlstore/model_provider.sql) - 提供商表结构

### 路由配置
- [cmd/service/router.go:142-147](cmd/service/router.go#L142-L147) - OCR路由定义

## 更新记录

| 日期 | 版本 | 更新内容 |
|------|------|----------|
| 2024-12-31 | 1.0 | 初始版本，支持Baidu OCR |
