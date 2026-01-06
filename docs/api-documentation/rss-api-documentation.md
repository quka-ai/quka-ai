# RSS 功能 API 对接文档

## 概述

QukaAI 的 RSS 功能支持用户订阅 RSS 源、自动抓取文章、AI 生成摘要，并提供每日摘要报告。本文档面向前端开发者，提供所有 RSS 相关 API 的详细说明。

## 基础信息

- **Base URL**: `/api/v1/:spaceid/rss`
- **认证方式**: JWT Token (通过 `Authorization: Bearer <token>` 头部传递)
- **权限要求**:
  - 查看类 API 需要 `PermissionView` 权限
  - 创建/修改/删除 API 需要 `PermissionEdit` 权限

## 通用响应格式

### 成功响应

```json
{
  "meta": {
    "code": 0,
    "message": "success",
    "request_id": "xxx"
  },
  "data": { ... }
}
```

### 错误响应

```json
{
  "meta": {
    "code": <错误码>,
    "message": "<错误信息>",
    "request_id": "xxx"
  },
  "data": null
}
```

## API 接口列表

### 一、RSS 订阅管理

#### 1.1 创建 RSS 订阅

**接口**: `POST /api/v1/:spaceid/rss/subscriptions`

**权限**: `PermissionEdit`

**请求参数**:

```json
{
  "resource_id": "string (必填)", // Resource ID
  "url": "string (必填)", // RSS Feed URL
  "title": "string (可选)", // 订阅标题，不填则使用 Feed 自带标题
  "description": "string (可选)", // 订阅描述，不填则使用 Feed 自带描述
  "category": "string (可选)", // 分类标签
  "update_frequency": "number (可选)" // 更新频率（秒），默认 3600（1小时）
}
```

**响应示例**:

```json
{
  "meta": {
    "code": 0,
    "message": "success",
    "request_id": "xxx"
  },
  "data": {
    "id": 1234567890,
    "user_id": "user_123",
    "space_id": "space_456",
    "resource_id": "resource_789",
    "url": "https://example.com/feed.xml",
    "title": "Example RSS Feed",
    "description": "This is an example feed",
    "category": "tech",
    "update_frequency": 3600,
    "last_fetched_at": 1702345678,
    "enabled": true,
    "created_at": 1702345678,
    "updated_at": 1702345678
  }
}
```

**错误情况**:

- `400`: URL 无效或订阅已存在
- `403`: 没有编辑权限
- `404`: Resource 不存在

---

#### 1.2 获取订阅列表

**接口**: `GET /api/v1/:spaceid/rss/subscriptions`

**权限**: `PermissionView`

**请求参数**: 无

**响应示例**:

```json
{
  "meta": {
    "code": 0,
    "message": "success",
    "request_id": "xxx"
  },
  "data": [
    {
      "id": 1234567890,
      "user_id": "user_123",
      "space_id": "space_456",
      "resource_id": "resource_789",
      "url": "https://example.com/feed.xml",
      "title": "Example RSS Feed",
      "description": "This is an example feed",
      "category": "tech",
      "update_frequency": 3600,
      "last_fetched_at": 1702345678,
      "enabled": true,
      "created_at": 1702345678,
      "updated_at": 1702345678
    }
  ]
}
```

**说明**:

- 返回当前用户在指定 Space 下的所有订阅
- 如果没有订阅，返回空数组

---

#### 1.3 获取订阅详情

**接口**: `GET /api/v1/:spaceid/rss/subscriptions/:id`

**权限**: `PermissionView`

**路径参数**:

- `id`: 订阅 ID

**响应示例**:

```json
{
  "meta": {
    "code": 0,
    "message": "success",
    "request_id": "xxx"
  },
  "data": {
    "id": 1234567890,
    "user_id": "user_123",
    "space_id": "space_456",
    "resource_id": "resource_789",
    "url": "https://example.com/feed.xml",
    "title": "Example RSS Feed",
    "description": "This is an example feed",
    "category": "tech",
    "update_frequency": 3600,
    "last_fetched_at": 1702345678,
    "enabled": true,
    "created_at": 1702345678,
    "updated_at": 1702345678
  }
}
```

**错误情况**:

- `403`: 没有查看权限
- `404`: 订阅不存在

---

#### 1.4 更新 RSS 订阅

**接口**: `PUT /api/v1/:spaceid/rss/subscriptions/:id`

**权限**: `PermissionEdit`

**路径参数**:

- `id`: 订阅 ID

**请求参数** (所有字段可选，只更新提供的字段):

```json
{
  "title": "string (可选)",
  "description": "string (可选)",
  "category": "string (可选)",
  "update_frequency": "number (可选)",
  "enabled": "boolean (可选)" // 启用/禁用订阅
}
```

**响应示例**:

```json
{
  "meta": {
    "code": 0,
    "message": "success",
    "request_id": "xxx"
  },
  "data": null
}
```

**错误情况**:

- `403`: 没有编辑权限或不是订阅所有者
- `404`: 订阅不存在

---

#### 1.5 删除 RSS 订阅

**接口**: `DELETE /api/v1/:spaceid/rss/subscriptions/:id`

**权限**: `PermissionEdit`

**路径参数**:

- `id`: 订阅 ID

**响应示例**:

```json
{
  "meta": {
    "code": 0,
    "message": "success",
    "request_id": "xxx"
  },
  "data": null
}
```

**说明**:

- 删除订阅不会删除已生成的 Knowledge 记录
- 已抓取的文章会保留在数据库中

**错误情况**:

- `403`: 没有编辑权限或不是订阅所有者
- `404`: 订阅不存在

---

#### 1.6 手动触发抓取

**接口**: `POST /api/v1/:spaceid/rss/subscriptions/:id/fetch`

**权限**: `PermissionEdit`

**路径参数**:

- `id`: 订阅 ID

**请求参数**: 无

**响应示例**:

```json
{
  "meta": {
    "code": 0,
    "message": "success",
    "request_id": "xxx"
  },
  "data": {
    "message": "抓取任务已触发"
  }
}
```

**说明**:

- 立即将抓取任务加入队列，无需等待定时任务
- 适用于用户想要立即查看新文章的场景

**错误情况**:

- `403`: 没有编辑权限或不是订阅所有者
- `404`: 订阅不存在

---

### 二、每日摘要 (Daily Digest)

#### 2.1 获取每日摘要（不存在则自动生成）

**接口**: `GET /api/v1/:spaceid/rss/digest/daily`

**权限**: `PermissionView`

**请求参数** (Query String):

```
date: string (可选)  // 日期格式：2006-01-02，不填则默认为前一天
```

**响应示例**:

```json
{
  "meta": {
    "code": 0,
    "message": "success",
    "request_id": "xxx"
  },
  "data": {
    "id": 987654321,
    "date": "2024-12-11",
    "content": "# 📅 2024年12月11日 RSS 每日摘要\n\n> 今日共收到 15 篇更新，涵盖 3 个主题\n\n## 🏷️ AI 技术动态\n\n...",
    "article_count": 15,
    "article_ids": [123, 456, 789],
    "model": "gpt-4",
    "generated_at": 1702345678
  }
}
```

**说明**:

- 如果指定日期的摘要已存在，直接返回
- 如果不存在，自动调用 AI 生成并保存
- 如果当天没有文章，返回空摘要提示
- content 为 Markdown 格式，包含文章链接（格式：`#knowledge-文章ID`）

**错误情况**:

- `400`: 日期格式错误
- `403`: 没有查看权限

---

#### 2.2 手动生成每日摘要

**接口**: `POST /api/v1/:spaceid/rss/digest/generate`

**权限**: `PermissionEdit`

**请求参数**:

```json
{
  "date": "string (可选)" // 日期格式：2006-01-02，不填则默认为前一天
}
```

**响应示例**:

```json
{
  "meta": {
    "code": 0,
    "message": "success",
    "request_id": "xxx"
  },
  "data": {
    "id": 987654321,
    "date": "2024-12-11",
    "content": "# 📅 2024年12月11日 RSS 每日摘要\n\n> 今日共收到 15 篇更新，涵盖 3 个主题\n\n## 🏷️ AI 技术动态\n\n...",
    "article_count": 15,
    "article_ids": [123, 456, 789],
    "model": "gpt-4",
    "generated_at": 1702345678
  }
}
```

**说明**:

- 返回完整的摘要内容（包含 Markdown 格式的 content 字段）
- 如果指定日期的摘要已存在，直接返回已有摘要（不会重新生成）
- 主动生成场景，适合用户主动请求生成历史摘要
- 如果文章数量为 0，不会创建数据库记录

**错误情况**:

- `400`: 日期格式错误
- `403`: 没有编辑权限

---

#### 2.3 获取历史摘要列表

**接口**: `GET /api/v1/:spaceid/rss/digest/history`

**权限**: `PermissionView`

**请求参数** (Query String):

```
start_date: string (可选)  // 开始日期，格式：2006-01-02
end_date: string (可选)    // 结束日期，格式：2006-01-02
limit: number (可选)       // 返回条数，默认 30
```

**响应示例**:

```json
{
  "meta": {
    "code": 0,
    "message": "success",
    "request_id": "xxx"
  },
  "data": [
    {
      "id": 987654321,
      "date": "2024-12-11",
      "article_count": 15,
      "model": "gpt-4",
      "generated_at": 1702345678
    },
    {
      "id": 987654320,
      "date": "2024-12-10",
      "article_count": 8,
      "model": "gpt-4",
      "generated_at": 1702259278
    }
  ]
}
```

**说明**:

- 按日期倒序返回（最新的在前）
- 如果指定日期范围，返回该范围内的摘要
- 如果不指定日期范围，返回最近的 N 条摘要（N 为 limit 参数）
- 不包含摘要内容，仅元数据

**错误情况**:

- `403`: 没有查看权限

---

#### 2.4 通过 ID 获取摘要详情

**接口**: `GET /api/v1/:spaceid/rss/digest/:id`

**权限**: `PermissionView`

**路径参数**:

- `id`: 摘要 ID

**响应示例**:

```json
{
  "meta": {
    "code": 0,
    "message": "success",
    "request_id": "xxx"
  },
  "data": {
    "id": 987654321,
    "date": "2024-12-11",
    "content": "# 📅 2024年12月11日 RSS 每日摘要\n\n...",
    "article_count": 15,
    "article_ids": [123, 456, 789],
    "model": "gpt-4",
    "generated_at": 1702345678
  }
}
```

**说明**:

- 包含完整的摘要内容（Markdown 格式）
- 适用于查看历史摘要详情

**错误情况**:

- `403`: 没有查看权限
- `404`: 摘要不存在

---

#### 2.5 删除摘要

**接口**: `DELETE /api/v1/:spaceid/rss/digest/:id`

**权限**: `PermissionEdit`

**路径参数**:

- `id`: 摘要 ID

**响应示例**:

```json
{
  "meta": {
    "code": 0,
    "message": "success",
    "request_id": "xxx"
  },
  "data": null
}
```

**说明**:

- 只删除摘要记录，不影响原始文章
- 删除后可重新生成

**错误情况**:

- `403`: 没有编辑权限
- `404`: 摘要不存在

---

## 数据模型说明

### RSSSubscription (订阅)

```typescript
interface RSSSubscription {
  id: number; // 订阅 ID
  user_id: string; // 用户 ID
  space_id: string; // Space ID
  resource_id: string; // Resource ID
  url: string; // RSS Feed URL
  title: string; // 订阅标题
  description: string; // 订阅描述
  category: string; // 分类标签
  update_frequency: number; // 更新频率（秒）
  last_fetched_at: number; // 上次抓取时间戳（Unix 时间戳）
  enabled: boolean; // 是否启用
  created_at: number; // 创建时间戳
  updated_at: number; // 更新时间戳
}
```

### DailyDigest (每日摘要)

```typescript
interface DailyDigest {
  id: number; // 摘要 ID
  date: string; // 日期（YYYY-MM-DD）
  content: string; // 摘要内容（Markdown 格式）
  article_count: number; // 文章总数
  article_ids: number[]; // 文章 ID 列表
  model: string; // AI 模型名称
  generated_at: number; // 生成时间戳
}
```

### DailyDigestItem (摘要列表项)

```typescript
interface DailyDigestItem {
  id: number; // 摘要 ID
  date: string; // 日期（YYYY-MM-DD）
  article_count: number; // 文章总数
  model: string; // AI 模型名称
  generated_at: number; // 生成时间戳
}
```

---

## 业务流程说明

### 1. RSS 订阅流程

```
1. 用户创建订阅 (POST /subscriptions)
   ↓
2. 后端验证 URL 有效性
   ↓
3. 创建订阅记录
   ↓
4. 自动加入抓取队列
   ↓
5. 后台定时抓取文章
   ↓
6. AI 生成文章摘要
   ↓
7. 保存为 Knowledge 记录
```

### 2. 每日摘要生成流程

```
1. 用户请求摘要 (GET /digest/daily?date=2024-12-11)
   ↓
2. 检查是否已存在
   ├─ 存在：直接返回
   └─ 不存在：
       ↓
       获取该日期所有文章（已生成摘要的）
       ↓
       按主题分类文章
       ↓
       调用 AI 生成整合摘要
       ↓
       保存摘要记录
       ↓
       返回结果
```

### 3. 文章链接处理

每日摘要中的文章链接格式为 `#knowledge-{文章ID}`，前端需要实现锚点跳转到对应的 Knowledge 详情页面。

**示例**:

- Markdown: `[如何优化 React 性能](#knowledge-123456)`
- 前端路由: `/knowledge/123456` 或 `/space/{spaceid}/knowledge/123456`

建议前端实现:

```typescript
// 监听链接点击事件
document.addEventListener("click", (e) => {
  const target = e.target as HTMLAnchorElement;
  if (target.hash && target.hash.startsWith("#knowledge-")) {
    e.preventDefault();
    const articleId = target.hash.replace("#knowledge-", "");
    // 跳转到 Knowledge 详情页
    router.push(`/space/${spaceId}/knowledge/${articleId}`);
  }
});
```

---

## 错误码说明

| 错误码 | 说明                       |
| ------ | -------------------------- |
| 0      | 成功                       |
| 400    | 请求参数错误               |
| 401    | 未授权（Token 无效或过期） |
| 403    | 权限不足                   |
| 404    | 资源不存在                 |
| 500    | 服务器内部错误             |

---

## 开发建议

### 1. 订阅管理页面

- 显示订阅列表，支持分类筛选
- 提供创建、编辑、删除订阅功能
- 显示上次抓取时间和启用状态
- 支持手动触发抓取

### 2. 每日摘要页面

- 日历视图，显示有摘要的日期
- 点击日期查看摘要详情
- 摘要内容用 Markdown 渲染器展示
- 支持文章链接跳转到 Knowledge 详情

### 3. 性能优化

- 订阅列表考虑分页或虚拟滚动
- 摘要内容较长，考虑懒加载
- 缓存摘要内容，避免重复请求

### 4. 用户体验

- 创建订阅时实时验证 URL
- 显示抓取进度和状态
- 摘要生成时显示 Loading 状态
- 支持搜索和过滤订阅

---

## 常见问题 (FAQ)

### Q1: 为什么创建订阅后立即请求列表看不到文章？

A: 订阅创建后会加入抓取队列，由后台异步处理。第一次抓取可能需要几秒到几分钟时间。可以通过 `last_fetched_at` 字段判断是否已完成首次抓取。

### Q2: 摘要生成需要多长时间？

A: 取决于文章数量和 AI 模型响应速度，通常在 5-30 秒之间。建议前端使用 Loading 状态提示用户等待。

### Q3: 如果某天没有文章会返回什么？

A: 会返回一个空摘要提示，`article_count` 为 0，`content` 包含友好提示信息。

### Q4: 可以修改已生成的摘要吗？

A: 当前不支持修改已生成的摘要。可以删除后重新生成，但由于 AI 生成结果可能不同，内容可能会有差异。

### Q5: 订阅的 `update_frequency` 如何影响抓取？

A: 后台定时任务会根据 `update_frequency` 和 `last_fetched_at` 判断是否需要抓取。建议设置为 3600（1 小时）或更长，避免频繁抓取。

---

## 联系与支持

如有疑问或需要技术支持，请联系后端团队或在项目 Issue 中提出。

**文档版本**: v1.0
**最后更新**: 2024-12-12
