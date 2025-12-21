# Podcast API 完整前端对接文档

## 目录

- [概述](#概述)
- [重要变更说明](#重要变更说明)
- [API 列表](#api-列表)
- [数据模型](#数据模型)
- [API 详情](#api-详情)
  - [获取播客列表](#获取播客列表)
  - [获取单个播客](#获取单个播客)
  - [根据源获取播客](#根据源获取播客)
  - [创建播客](#创建播客)
  - [批量创建播客](#批量创建播客)
  - [重新生成播客](#重新生成播客)
  - [删除播客](#删除播客)
- [前端集成示例](#前端集成示例)
- [最佳实践](#最佳实践)
- [错误处理](#错误处理)

## 概述

Podcast API 允许用户将知识库、日记或 RSS 摘要转换为播客音频。支持多种 TTS（文本转语音）提供商，并提供完整的生命周期管理。

## 重要变更说明

### v2024.12 - TTS 配置简化

为了简化播客生成的配置，以下 TTS 字段已被移除：

- ❌ `tts_voice` - TTS 语音
- ❌ `tts_speed` - TTS 语速
- ❌ `tts_language` - TTS 语言

**保留字段：**

- ✅ `tts_provider` - TTS 提供商
- ✅ `tts_model` - TTS 模型

**影响范围：**

- API 响应中将不再包含这三个字段
- 前端代码需要相应更新，移除对这些字段的引用
- 数据库迁移脚本：`podcast_remove_tts_fields.sql`

**升级指南：**
如果您的应用依赖这些字段，请考虑：

1. 使用默认的 TTS 设置
2. 在应用层实现语音、语速和语言的控制逻辑
3. 联系开发团队了解替代方案

### 核心功能

- 📝 支持多种源类型（Knowledge、Journal、RSS Digest）
- 🎙️ 多种 TTS 提供商（OpenAI、Azure OpenAI、Qwen 等）
- 📊 实时状态追踪
- 🔄 重新生成功能
- 📦 批量处理支持

## API 列表

| 方法   | 路径                                       | 描述           | 权限 |
| ------ | ------------------------------------------ | -------------- | ---- |
| GET    | `/api/v1/:spaceid/podcasts`                | 获取播客列表   | view |
| GET    | `/api/v1/:spaceid/podcasts/:id`            | 获取单个播客   | view |
| GET    | `/api/v1/:spaceid/podcasts/source`         | 根据源获取播客 | view |
| POST   | `/api/v1/:spaceid/podcasts`                | 创建播客       | edit |
| POST   | `/api/v1/:spaceid/podcasts/batch`          | 批量创建播客   | edit |
| POST   | `/api/v1/:spaceid/podcasts/:id/regenerate` | 重新生成播客   | edit |
| DELETE | `/api/v1/:spaceid/podcasts/:id`            | 删除播客       | edit |

## 数据模型

### Podcast 对象

```typescript
interface Podcast {
  id: string; // 播客唯一标识
  user_id: string; // 用户ID
  space_id: string; // Space ID

  // 来源信息
  source_type: "knowledge" | "journal" | "rss_digest";
  source_id: string; // 源内容ID

  // 基本信息
  title: string; // 标题
  description: string; // 描述
  tags: string[]; // 标签

  // 音频信息
  audio_url: string; // 音频文件URL（预签名）
  audio_duration: number; // 音频时长（秒）
  audio_size: number; // 音频文件大小（字节）
  audio_format: string; // 音频格式（mp3, wav等）

  // TTS 配置
  tts_provider: string; // TTS 提供商
  tts_model: string; // TTS 模型

  // 状态信息
  status: "pending" | "processing" | "completed" | "failed";
  error_message?: string; // 错误信息
  retry_times: number; // 重试次数

  // 时间戳
  created_at: number; // 创建时间
  updated_at: number; // 更新时间
  generated_at?: number; // 生成完成时间
}
```

### Source Type 说明

- `knowledge`: 知识库文章
- `journal`: 日记条目
- `rss_digest`: RSS 每日摘要

### Status 说明

- `pending`: 等待处理
- `processing`: 处理中
- `completed`: 生成完成
- `failed`: 生成失败

## API 详情

### 获取播客列表

**请求**

```http
GET /api/v1/:spaceid/podcasts?source_type=knowledge&status=completed&page=1&page_size=20
```

**查询参数**

| 参数        | 类型   | 必需 | 说明                        |
| ----------- | ------ | ---- | --------------------------- |
| source_type | string | 否   | 按源类型过滤                |
| status      | string | 否   | 按状态过滤                  |
| page        | number | 是   | 页码，默认 1                |
| page_size   | number | 是   | 每页数量，默认 20，最大 100 |

**响应**

```json
{
  "meta": {
    "code": 200,
    "message": "success",
    "request_id": "xxx"
  },
  "data": {
    "podcasts": [
      {
        "id": "podcast-uuid",
        "title": "播客标题",
        "status": "completed"
        // ... 其他字段
      }
    ],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

### 获取单个播客

**请求**

```http
GET /api/v1/:spaceid/podcasts/:id
```

**路径参数**

| 参数 | 类型   | 说明    |
| ---- | ------ | ------- |
| id   | string | 播客 ID |

**响应**

返回完整的 Podcast 对象，详见[数据模型](#podcast-对象)。

**完整响应示例：**

```json
{
  "meta": {
    "code": 200,
    "message": "success",
    "request_id": "xxx"
  },
  "data": {
    "id": "podcast-uuid",
    "user_id": "user-uuid",
    "space_id": "space-uuid",
    "source_type": "knowledge",
    "source_id": "knowledge-uuid-123",
    "title": "播客标题",
    "description": "播客描述",
    "tags": ["tag1", "tag2"],
    "audio_url": "https://s3.amazonaws.com/bucket/audio.mp3",
    "audio_duration": 1800,
    "audio_size": 28800000,
    "audio_format": "mp3",
    "tts_provider": "openai",
    "tts_model": "tts-1",
    "status": "completed",
    "error_message": "",
    "retry_times": 0,
    "created_at": 1703123456,
    "updated_at": 1703123456,
    "generated_at": 1703123556
  }
}
```

### 根据源获取播客

**请求**

```http
GET /api/v1/:spaceid/podcasts/source?source_type=knowledge&source_id=knowledge-uuid-123
```

**查询参数**

| 参数        | 类型   | 必需 | 说明      |
| ----------- | ------ | ---- | --------- |
| source_type | string | 是   | 源类型    |
| source_id   | string | 是   | 源内容 ID |

**使用场景**

在前端知识库详情页检查是否有对应播客：

```javascript
async function checkKnowledgeHasPodcast(spaceId, knowledgeId) {
  try {
    const response = await fetch(
      `/api/v1/${spaceId}/podcasts/source?source_type=knowledge&source_id=${knowledgeId}`,
      {
        headers: { Authorization: `Bearer ${token}` },
      }
    );
    const result = await response.json();
    return result.success ? result.data : null;
  } catch (error) {
    console.error("检查播客失败:", error);
    return null;
  }
}
```

**响应**

- 找到：返回 Podcast 对象（包装在 `data` 字段中）
- 未找到：返回 404 错误

**成功响应示例：**

```json
{
  "meta": {
    "code": 200,
    "message": "success",
    "request_id": "xxx"
  },
  "data": {
    "id": "podcast-uuid",
    "status": "completed"
    // ... Podcast 完整对象
  }
}
```

### 创建播客

**请求**

```http
POST /api/v1/:spaceid/podcasts
Content-Type: application/json

{
  "source_type": "knowledge",
  "source_id": "knowledge-uuid-123"
}
```

**请求体**

| 字段        | 类型   | 必需 | 说明      |
| ----------- | ------ | ---- | --------- |
| source_type | string | 是   | 源类型    |
| source_id   | string | 是   | 源内容 ID |

**响应**

```json
{
  "meta": {
    "code": 200,
    "message": "success",
    "request_id": "xxx"
  },
  "data": {
    "id": "podcast-uuid",
    "status": "pending"
  }
}
```

实际返回的是 `CreatePodcastResponse` 对象，包含：

- `id`: 播客唯一标识
- `status`: 当前状态（pending/processing/completed/failed）

**前端示例**

```javascript
async function createPodcast(spaceId, sourceType, sourceId) {
  try {
    const response = await fetch(`/api/v1/${spaceId}/podcasts`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        source_type: sourceType,
        source_id: sourceId,
      }),
    });

    const result = await response.json();
    if (result.success) {
      console.log("播客创建成功:", result.data);
      return result.data;
    }
  } catch (error) {
    console.error("创建播客失败:", error);
  }
}
```

### 批量创建播客

**请求**

```http
POST /api/v1/:spaceid/podcasts/batch
Content-Type: application/json

{
  "source_type": "knowledge",
  "source_ids": ["id1", "id2", "id3"]
}
```

**请求体**

| 字段        | 类型   | 必需 | 说明           |
| ----------- | ------ | ---- | -------------- |
| source_type | string | 是   | 源类型         |
| source_ids  | array  | 是   | 源内容 ID 列表 |

**响应**

```json
{
  "meta": {
    "code": 200,
    "message": "success",
    "request_id": "xxx"
  },
  "data": {
    "created_count": 3,
    "podcast_ids": ["id1", "id2", "id3"]
  }
}
```

### 重新生成播客

**请求**

```http
POST /api/v1/:spaceid/podcasts/:id/regenerate
```

**响应**

```json
{
  "meta": {
    "code": 200,
    "message": "success",
    "request_id": "xxx"
  },
  "data": null
}
```

**前端示例**

```javascript
async function regeneratePodcast(spaceId, podcastId) {
  try {
    const response = await fetch(
      `/api/v1/${spaceId}/podcasts/${podcastId}/regenerate`,
      {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      }
    );
    return await response.json();
  } catch (error) {
    console.error("重新生成失败:", error);
  }
}
```

### 删除播客

**请求**

```http
DELETE /api/v1/:spaceid/podcasts/:id
```

**响应**

```json
{
  "meta": {
    "code": 200,
    "message": "success",
    "request_id": "xxx"
  },
  "data": null
}
```

## 前端集成示例

### 播客列表页面

```javascript
import React, { useState, useEffect } from "react";

function PodcastList({ spaceId }) {
  const [podcasts, setPodcasts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [filters, setFilters] = useState({
    source_type: "",
    status: "",
    page: 1,
    page_size: 20,
  });

  useEffect(() => {
    fetchPodcasts();
  }, [filters]);

  const fetchPodcasts = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (filters.source_type)
        params.append("source_type", filters.source_type);
      if (filters.status) params.append("status", filters.status);
      params.append("page", filters.page);
      params.append("page_size", filters.page_size);

      const response = await fetch(`/api/v1/${spaceId}/podcasts?${params}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const result = await response.json();
      if (result.success) {
        setPodcasts(result.data.podcasts);
      }
    } catch (error) {
      console.error("获取播客列表失败:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      {/* 过滤器 */}
      <div className="filters">
        <select
          value={filters.source_type}
          onChange={(e) =>
            setFilters({ ...filters, source_type: e.target.value })
          }
        >
          <option value="">所有源类型</option>
          <option value="knowledge">知识库</option>
          <option value="journal">日记</option>
          <option value="rss_digest">RSS摘要</option>
        </select>

        <select
          value={filters.status}
          onChange={(e) => setFilters({ ...filters, status: e.target.value })}
        >
          <option value="">所有状态</option>
          <option value="pending">等待中</option>
          <option value="processing">处理中</option>
          <option value="completed">已完成</option>
          <option value="failed">失败</option>
        </select>
      </div>

      {/* 播客列表 */}
      {loading ? (
        <div>加载中...</div>
      ) : (
        <div className="podcast-grid">
          {podcasts.map((podcast) => (
            <PodcastCard
              key={podcast.id}
              podcast={podcast}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function PodcastCard({ podcast }) {
  const getStatusColor = (status) => {
    const colors = {
      pending: "#f59e0b",
      processing: "#3b82f6",
      completed: "#10b981",
      failed: "#ef4444",
    };
    return colors[status] || "#6b7280";
  };

  return (
    <div className="podcast-card">
      <div className="podcast-header">
        <h3>{podcast.title}</h3>
        <span
          className="status-badge"
          style={{ backgroundColor: getStatusColor(podcast.status) }}
        >
          {podcast.status}
        </span>
      </div>

      <p className="podcast-description">{podcast.description}</p>

      <div className="podcast-meta">
        <span>来源: {podcast.source_type}</span>
        <span>时长: {Math.floor(podcast.audio_duration / 60)}分钟</span>
      </div>

      {podcast.status === "completed" && podcast.audio_url && (
        <audio
          controls
          src={podcast.audio_url}
        >
          您的浏览器不支持音频播放
        </audio>
      )}

      <div className="podcast-actions">
        <button onClick={() => regeneratePodcast(podcast.id)}>重新生成</button>
        <button onClick={() => deletePodcast(podcast.id)}>删除</button>
      </div>
    </div>
  );
}
```

### 知识库详情页集成

```javascript
function KnowledgeDetail({ spaceId, knowledgeId }) {
  const [knowledge, setKnowledge] = useState(null);
  const [podcast, setPodcast] = useState(null);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    fetchKnowledge();
    checkPodcast();
  }, [knowledgeId]);

  const checkPodcast = async () => {
    const existingPodcast = await checkKnowledgeHasPodcast(
      spaceId,
      knowledgeId
    );
    setPodcast(existingPodcast);
  };

  const handleCreatePodcast = async () => {
    setCreating(true);
    try {
      const result = await createPodcast(spaceId, "knowledge", knowledgeId);
      if (result) {
        // 开始轮询检查状态
        pollPodcastStatus(result.id);
      }
    } finally {
      setCreating(false);
    }
  };

  const pollPodcastStatus = async (podcastId) => {
    const interval = setInterval(async () => {
      const response = await fetch(`/api/v1/${spaceId}/podcasts/${podcastId}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const result = await response.json();
      if (result.success) {
        setPodcast(result.data);
        if (
          result.data.status === "completed" ||
          result.data.status === "failed"
        ) {
          clearInterval(interval);
        }
      }
    }, 5000); // 每5秒检查一次

    // 5分钟后停止轮询
    setTimeout(() => clearInterval(interval), 300000);
  };

  return (
    <div className="knowledge-detail">
      <h1>{knowledge?.title}</h1>

      <div className="knowledge-actions">
        {podcast ? (
          <div className="podcast-section">
            <h3>播客</h3>
            {podcast.status === "completed" ? (
              <audio
                controls
                src={podcast.audio_url}
              />
            ) : (
              <div className="status-indicator">
                状态: {podcast.status}
                {podcast.status === "processing" && (
                  <span className="spinner">⏳</span>
                )}
              </div>
            )}
          </div>
        ) : (
          <button
            onClick={handleCreatePodcast}
            disabled={creating}
          >
            {creating ? "创建中..." : "转换为播客"}
          </button>
        )}
      </div>
    </div>
  );
}
```

## 最佳实践

### 1. 状态管理

- 使用 WebSocket 实时更新播客状态（如果可用）
- 轮询间隔建议 5-10 秒
- 实现超时机制，避免无限轮询

### 2. 音频播放

- 预签名 URL 有有效期，及时处理过期问题
- 提供加载状态和错误处理
- 支持播放速度调节

### 3. 批量操作

- 显示批量创建进度
- 处理部分失败的情况
- 提供取消操作的功能

### 4. 用户体验

- 提供清晰的加载状态
- 错误信息本地化
- 支持操作撤销（删除）

### 5. 性能优化

- 列表分页加载
- 音频预加载
- 缓存播客信息

## 错误处理

### 常见错误码

| 错误码                 | HTTP 状态码 | 说明         | 解决方案               |
| ---------------------- | ----------- | ------------ | ---------------------- |
| VALIDATION_ERROR       | 400         | 请求参数无效 | 检查请求参数           |
| PODCAST_NOT_FOUND      | 404         | 播客不存在   | 检查播客 ID            |
| UNAUTHORIZED           | 401         | 未授权       | 检查 JWT token         |
| FORBIDDEN              | 403         | 权限不足     | 检查 Space 权限        |
| PODCAST_ALREADY_EXISTS | 409         | 播客已存在   | 提示用户或获取现有播客 |

### 错误处理示例

```javascript
async function handleApiError(response) {
  const result = await response.json();

  if (!result.success) {
    const { code, message } = result.error;

    switch (code) {
      case "PODCAST_NOT_FOUND":
        alert("播客不存在或已被删除");
        break;
      case "PODCAST_ALREADY_EXISTS":
        alert("该内容已经创建过播客");
        // 可以跳转到现有播客
        break;
      case "UNAUTHORIZED":
        // 重新登录
        redirectToLogin();
        break;
      default:
        alert(`操作失败: ${message}`);
    }
  }
}
```

---

## 总结

Podcast API 提供了完整的播客生命周期管理功能。前端集成时需要注意：

1. **状态追踪**: 实时更新播客生成状态
2. **权限控制**: 确保用户有足够的权限
3. **错误处理**: 提供友好的错误提示
4. **用户体验**: 优化加载状态和交互反馈

如有问题，请参考本文档或联系后端团队。
