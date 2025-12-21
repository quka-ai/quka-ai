# Podcast 分享功能实现文档

## 概述
本文档记录了为 QukaAI 项目添加 Podcast 分享功能的实现细节。

## 实施日期
2025-12-19

## 实现内容

### 1. 添加分享类型常量
**文件**: `pkg/types/share_token.go`

在 `ShareToken` 类型常量中新增：
```go
SHARE_TYPE_PODCAST = "podcast"
```

### 2. 业务逻辑层实现
**文件**: `app/logic/v1/share.go`

#### 2.1 创建 Podcast 分享 Token
添加了 `CreatePodcastShareToken` 方法：
- 权限检查：要求 `PermissionView` 权限
- Token 生成：使用 MD5 哈希生成唯一 token
- 有效期：7天
- 支持 token 复用：如果已存在未过期的分享链接，直接返回

#### 2.2 通过 Token 获取 Podcast 信息
添加了 `GetPodcastByShareToken` 方法：
- 根据 token 查询分享链接
- 获取对应的 podcast 详情
- 获取分享用户信息
- 返回完整的分享信息（包括用户信息和 podcast 数据）

### 3. HTTP Handler 层
**文件**: `cmd/service/handler/share.go`

#### 3.1 创建分享链接 API
- **Handler**: `CreatePodcastShareToken`
- **请求参数**:
  - `embedding_url`: 嵌入页面 URL（必填）
  - `podcast_id`: Podcast ID（必填）
- **响应**:
  - `token`: 分享 token
  - `url`: 完整的分享 URL

#### 3.2 获取分享内容 API
- **Handler**: `GetPodcastByShareToken`
- **功能**: 通过 token 获取 podcast 分享信息

#### 3.3 分享页面渲染
- **Handler**: `BuildPodcastSharePage`
- **功能**: 渲染 HTML 分享页面
- **模板**: 使用 `share.html` 模板

#### 3.4 URL 生成函数
添加了 `genPodcastShareURL` 函数：
- 格式: `{domain}/s/p/{token}`
- 与其他分享类型保持一致的 URL 结构

### 4. 路由配置
**文件**: `cmd/service/router.go`

#### 4.1 公开访问路由（无需认证）
```go
s.Engine.GET("/s/p/:token", s.BuildPodcastSharePage)
```
用于直接访问分享页面

#### 4.2 API 路由
```go
// 获取分享内容（无需认证）
share.GET("/podcast/:token", s.GetPodcastByShareToken)

// 创建分享链接（需要认证和权限）
space.POST("/:spaceid/podcast/share", middleware.PaymentRequired, s.CreatePodcastShareToken)
```

## API 接口说明

### 创建 Podcast 分享链接
**接口**: `POST /api/v1/space/:spaceid/podcast/share`

**请求头**:
- `Authorization`: Bearer token（必需）

**请求体**:
```json
{
  "embedding_url": "https://example.com/embed/{token}",
  "podcast_id": "podcast-id-here"
}
```

**响应**:
```json
{
  "token": "generated-token-hash",
  "url": "https://share.example.com/s/p/generated-token-hash"
}
```

### 获取 Podcast 分享信息
**接口**: `GET /api/v1/share/podcast/:token`

**响应**:
```json
{
  "user_id": "user-id",
  "user_name": "User Name",
  "user_avatar": "https://avatar.url",
  "podcast": {
    "id": "podcast-id",
    "title": "Podcast Title",
    "description": "Podcast Description",
    "audio_url": "https://audio.url",
    "audio_duration": 3600,
    "status": "completed",
    ...
  },
  "embedding_url": "https://example.com/embed/token"
}
```

### 访问分享页面
**接口**: `GET /s/p/:token`

直接在浏览器访问，返回渲染好的 HTML 页面。

## 技术特性

### 1. 安全性
- Token 采用 MD5 哈希，包含 spaceID、podcastID 和时间戳
- 分享链接有 7 天有效期
- 支持权限控制（需要 View 权限才能创建分享链接）

### 2. 用户体验
- 自动复用已存在的分享链接
- 过期链接自动更新有效期
- 支持自定义分享域名

### 3. 一致性
- 遵循现有的分享功能设计模式
- URL 结构与 knowledge、session、space 分享保持一致
- 错误处理遵循项目 i18n 规范

## 数据库
使用现有的 `share_token` 表，无需新增表或字段。

## 依赖
- 依赖 `PodcastStore.Get()` 方法获取 podcast 数据
- 依赖 `ShareTokenStore` 进行分享 token 管理
- 依赖 `UserStore.GetUser()` 获取用户信息

## 测试建议

### 单元测试
1. 测试 token 生成的唯一性
2. 测试权限检查逻辑
3. 测试过期时间更新逻辑

### 集成测试
1. 测试完整的分享流程（创建 -> 访问 -> 获取数据）
2. 测试无效 token 的处理
3. 测试过期 token 的更新

### 端到端测试
1. 前端创建分享链接
2. 通过分享链接访问页面
3. 验证 podcast 播放功能

## 后续优化建议
1. 添加分享统计（访问次数、播放次数等）
2. 支持分享链接的自定义有效期
3. 支持分享链接的撤销功能
4. 添加分享链接的访问密码保护选项

## 相关文件
- `pkg/types/share_token.go` - 分享类型定义
- `app/logic/v1/share.go` - 分享业务逻辑
- `cmd/service/handler/share.go` - HTTP 处理器
- `cmd/service/router.go` - 路由配置
- `pkg/types/podcast.go` - Podcast 类型定义
