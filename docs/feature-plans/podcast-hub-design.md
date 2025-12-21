# Podcast Hub 功能设计文档

## 1. 概述

### 1.1 功能简介

Podcast Hub 是 QukaAI 的一个创新功能，旨在将用户的知识库内容（Knowledge、Journal、RSS Daily Digest）转换为音频播客形式，让用户可以通过收听的方式来回顾和消化自己的记忆内容。

### 1.2 核心价值

- **多模态内容消费**: 让用户可以在通勤、运动等场景下收听自己的知识库内容
- **记忆强化**: 通过听觉方式增强记忆留存
- **内容盘活**: 将静态文字内容转化为动态音频内容
- **个性化播客**: 每个用户都有自己独特的知识播客库

### 1.3 支持的内容类型

- **Knowledge**: 用户创建的各类知识卡片（文本、URL 等）
- **Journal**: 用户的日记内容
- **RSS Daily Digest**: RSS 订阅的每日摘要

## 2. 功能设计

### 2.1 核心功能模块

#### 2.1.1 Podcast 创建与管理

- 支持一键将 Knowledge/Journal/RSS Daily Digest 转换为 Podcast
- 支持批量创建 Podcast（如将一周的 Journal 批量转换）
- 支持 Podcast 的删除和查询
- Podcast 自动继承源内容的标签

#### 2.1.2 TTS（文字转语音）集成

- core 下已经集成 core.srv.Postcast 文字转播客服务

#### 2.1.3 音频文件管理

- 生成的音频文件存储到 S3 兼容存储
- 音频文件格式：MP3
- 支持音频文件的下载和在线播放

### 2.2 数据模型设计

#### 2.2.1 Podcast 表（quka_podcasts）

```sql
CREATE TABLE quka_podcasts (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    space_id VARCHAR(36) NOT NULL,

    -- 来源信息
    source_type VARCHAR(20) NOT NULL, -- 'knowledge', 'journal', 'rss_digest'
    source_id VARCHAR(36) NOT NULL,   -- 对应源的ID

    -- 基本信息
    title VARCHAR(500) NOT NULL,
    description TEXT,
    tags TEXT[], -- 标签数组

    -- 音频信息
    audio_url VARCHAR(1000),          -- S3存储的音频文件URL
    audio_duration INTEGER,           -- 音频时长（秒）
    audio_size BIGINT,                -- 音频文件大小（字节）
    audio_format VARCHAR(10),         -- 音频格式 mp3/m4a

    -- TTS 配置
    tts_provider VARCHAR(50),         -- TTS服务商
    tts_model VARCHAR(100),           -- TTS模型

    -- 状态信息
    status VARCHAR(20) NOT NULL,      -- 'pending', 'processing', 'completed', 'failed'
    error_message TEXT,               -- 错误信息
    retry_times INTEGER DEFAULT 0,    -- 重试次数

    -- 时间戳
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    generated_at BIGINT,              -- 音频生成完成时间

    -- 索引
    INDEX idx_user_space (user_id, space_id),
    INDEX idx_source (source_type, source_id),
    INDEX idx_status (status),
    INDEX idx_created_at (created_at DESC)
);
```

#### 2.2.2 TTS 配置

**说明**:TTS 配置已在 `core.srv` 中实现,使用 core.srv.Podcast()

### 2.3 Go 数据结构设计

#### 2.3.1 pkg/types/podcast.go

```go
package types

import "github.com/lib/pq"

// PodcastSourceType 播客来源类型
type PodcastSourceType string

const (
    PODCAST_SOURCE_KNOWLEDGE   PodcastSourceType = "knowledge"
    PODCAST_SOURCE_JOURNAL     PodcastSourceType = "journal"
    PODCAST_SOURCE_RSS_DIGEST  PodcastSourceType = "rss_digest"
)

// PodcastStatus 播客状态
type PodcastStatus string

const (
    PODCAST_STATUS_PENDING    PodcastStatus = "pending"
    PODCAST_STATUS_PROCESSING PodcastStatus = "processing"
    PODCAST_STATUS_COMPLETED  PodcastStatus = "completed"
    PODCAST_STATUS_FAILED     PodcastStatus = "failed"
)

// Podcast 播客
type Podcast struct {
    ID          string            `json:"id" db:"id"`
    UserID      string            `json:"user_id" db:"user_id"`
    SpaceID     string            `json:"space_id" db:"space_id"`

    // 来源信息
    SourceType  PodcastSourceType `json:"source_type" db:"source_type"`
    SourceID    string            `json:"source_id" db:"source_id"`

    // 基本信息
    Title       string            `json:"title" db:"title"`
    Description string            `json:"description" db:"description"`
    Tags        pq.StringArray    `json:"tags" db:"tags"`

    // 音频信息
    AudioURL      string  `json:"audio_url" db:"audio_url"`
    AudioDuration int     `json:"audio_duration" db:"audio_duration"`
    AudioSize     int64   `json:"audio_size" db:"audio_size"`
    AudioFormat   string  `json:"audio_format" db:"audio_format"`

    // TTS 配置
    TTSProvider string  `json:"tts_provider" db:"tts_provider"`
    TTSModel    string  `json:"tts_model" db:"tts_model"`
    TTSVoice    string  `json:"tts_voice" db:"tts_voice"`
    TTSSpeed    float64 `json:"tts_speed" db:"tts_speed"`

    // 状态信息
    Status       PodcastStatus `json:"status" db:"status"`
    ErrorMessage string        `json:"error_message" db:"error_message"`
    RetryTimes   int           `json:"retry_times" db:"retry_times"`

    // 时间戳
    CreatedAt   int64 `json:"created_at" db:"created_at"`
    UpdatedAt   int64 `json:"updated_at" db:"updated_at"`
    GeneratedAt int64 `json:"generated_at" db:"generated_at"`
}
    TTSLanguage string  `json:"tts_language" db:"tts_language"`

    CreatedAt int64 `json:"created_at" db:"created_at"`
    UpdatedAt int64 `json:"updated_at" db:"updated_at"`
}

// CreatePodcastRequest 创建播客请求
type CreatePodcastRequest struct {
    SourceType PodcastSourceType `json:"source_type" binding:"required"`
    SourceID   string            `json:"source_id" binding:"required"`
}

// BatchCreatePodcastRequest 批量创建播客请求
type BatchCreatePodcastRequest struct {
    SourceType  PodcastSourceType `json:"source_type" binding:"required"`
    SourceIDs   []string          `json:"source_ids" binding:"required"`
}
```

### 2.4 API 设计

#### 2.4.1 Podcast 管理 API

**创建 Podcast**

```
POST /api/v1/podcasts
Content-Type: application/json

{
    "source_type": "knowledge",
    "source_id": "xxx-xxx-xxx"
}

Response:
{
    "id": "podcast-id",
    "status": "pending",
    "message": "Podcast creation task submitted"
}
```

**批量创建 Podcast**

```
POST /api/v1/podcasts/batch
Content-Type: application/json

{
    "source_type": "journal",
    "source_ids": ["id1", "id2", "id3"]
}

Response:
{
    "created_count": 3,
    "podcast_ids": ["id1", "id2", "id3"]
}
```

**获取 Podcast 列表**

```
GET /api/v1/podcasts?source_type=knowledge&status=completed&page=1&page_size=20

Response:
{
    "podcasts": [...],
    "total": 100,
    "page": 1,
    "page_size": 20
}
```

**获取单个 Podcast**

```
GET /api/v1/podcasts/:id

Response:
{
    "id": "xxx",
    "title": "...",
    "audio_url": "...",
    ...
}
```

**删除 Podcast**

```
DELETE /api/v1/podcasts/:id

Response:
{
    "message": "Podcast deleted successfully"
}
```

**重新生成 Podcast**

```
POST /api/v1/podcasts/:id/regenerate

Response:
{
    "message": "Podcast regeneration task submitted"
}
```

### 2.5 业务逻辑设计

#### 2.5.1 Podcast 生成流程

```
1. 用户发起创建 Podcast 请求
   ↓
2. 验证源内容是否存在（Knowledge/Journal/RSS Digest）
   ↓
3. 创建 Podcast 记录（状态：pending）
   ↓
4. 提取源内容的文本
   - Knowledge: content 字段
   - Journal: content 字段
   - RSS Daily Digest: content 字段
   ↓
5. 文本预处理
   - 调用LLM对文本进行预处理，如果是block格式，则先转换为markdown再进行预处理
   - HTML/Markdown 转纯文本
   - 移除特殊字符
   - 通过LLM 提取 标题及描述
   ↓
7. 调用 core.srv.Postcast 生成音频（异步任务）
   - 更新状态为 processing
   - 调用 Podcast 服务
   ↓
8. 音频文件处理
   - 上传到 S3 存储
   - 获取音频元信息（时长、大小）
   ↓
9. 更新 Podcast 记录
   - 状态: completed
   - audio_url: S3 URL
   - audio_duration: 时长
   - generated_at: 完成时间
```

#### 2.5.2 异步任务处理

使用项目现有的队列系统（pkg/queue）处理 TTS 生成任务：

```go
// pkg/queue/podcast_queue.go
type PodcastQueue struct {
    redis  *redis.Client
    prefix string
}

const (
    PODCAST_GENERATION_QUEUE = "podcast:generation"
)

// 入队
func (q *PodcastQueue) EnqueueGenerationTask(podcastID string) error

// 出队并处理
func (q *PodcastQueue) ProcessGenerationTask() error
```

#### 2.5.3 重试机制

- TTS 生成失败时，支持自动重试（最多 3 次）
- 每次重试间隔递增（1min, 5min, 15min）
- 重试次数用完后，状态设置为 failed
- 用户可以手动触发重新生成

## 3. 技术实现要点

### 3.1 文本预处理

#### 3.1.1 内容格式转换

- **Markdown 转纯文本**: 移除 Markdown 语法标记，保留文本内容
- **HTML 转纯文本**: 使用 goquery 或类似库提取文本
- **EditorJS Blocks 转纯文本**: 解析 blocks 结构，提取文本

#### 3.1.2 朗读优化

- 添加适当的停顿标记（SSML）
- 处理特殊符号和缩写
- 优化数字和日期的朗读
- 处理链接和引用

### 3.2 音频文件管理

#### 3.2.1 存储策略

- 使用项目现有的 S3 存储服务
- 文件路径: `podcasts/{user_id}/{podcast_id}.{format}`
- 支持音频文件过期策略（可选）

#### 3.2.2 音频格式

- 优先使用 MP3 格式（兼容性好）
- 支持 M4A 格式（Apple 设备优化）
- 比特率: 64-128 kbps（平衡质量和文件大小）

### 3.3 性能优化

#### 3.3.1 长文本处理

- TTS API 通常有单次请求的文本长度限制
- 需要将长文本分段处理：
  - 按自然段落分割
  - 每段控制在合理长度（如 4000 字符）
  - 保持语义完整性

#### 3.3.2 音频合并

- 使用 ffmpeg 合并多段音频
- 确保音频片段之间的平滑过渡

#### 3.3.3 并发控制

- TTS API 调用限流
- 使用队列控制并发任务数
- 避免对 TTS 服务造成过大压力

### 3.4 成本控制

#### 3.4.1 TTS 用量统计

- 记录每次 TTS 调用的字符数
- 统计用户的 TTS 使用量
- 可以设置用量配额

#### 3.4.2 缓存策略

- 相同内容的 Podcast 可以复用（可选）
- 缓存常用的 TTS 配置

## 4. 实施计划

### 4.1 Phase 1: 基础架构（第 1 周）

- [ ] 创建数据库表结构（podcasts）
- [ ] 定义 Go 数据结构和接口
- [ ] 实现 Store 层（数据库操作）

### 4.2 Phase 2: 核心功能（第 2-3 周）

- [ ] 实现 Podcast 创建和管理 API
- [ ] 实现文本提取和预处理逻辑
- [ ] 实现异步任务队列
- [ ] 实现音频文件上传到 S3

### 4.3 Phase 3: 优化和测试（第 6-7 周）

- [ ] 性能优化和并发控制
- [ ] 错误处理和重试机制完善
- [ ] 单元测试和集成测试
- [ ] API 文档完善

## 5. 关键考虑点

### 5.1 用户体验

- **快速反馈**: 创建 Podcast 后立即返回，异步处理
- **进度通知**: 通过 WebSocket 推送生成进度
- **错误提示**: 清晰的错误信息和重试指引

### 5.2 数据一致性

- **源内容更新**: 当 Knowledge/Journal 内容更新时，是否需要更新对应的 Podcast？
  - 建议：不自动更新，由用户手动触发重新生成
- **源内容删除**: 当源内容被删除时，Podcast 是否保留？
  - 建议：保留 Podcast（用户可能还想听），但标记源内容已删除

### 5.3 权限控制

- Podcast 继承源内容的 Space 权限
- 同一 Space 内的成员可以访问
- 分享功能：可以生成 Podcast 分享链接

### 5.4 配额管理

- 限制用户每日可创建的 Podcast 数量
- 限制音频文件总存储空间
- 不同用户计划有不同的配额

### 5.5 国际化支持

- TTS 支持多语言（中文、英文等）
- 自动检测源内容的语言
- 用户可以手动指定语言

## 6. 未来扩展

### 6.1 AI 增强功能

- **内容总结**: 为长文本生成播客摘要版本
- **多声音对话**: 为对话类内容使用多个声音
- **背景音乐**: 为播客添加背景音乐
- **智能分章节**: 自动为长播客添加章节标记

### 6.2 社交功能

- 分享播客到社交平台
- 播客评论和讨论
- 播客推荐系统

### 6.3 离线功能

- 支持播客下载到本地
- 离线播放支持

### 6.4 播客订阅

- 订阅特定标签的 Podcast
- 自动生成系列播客

## 7. 相关文件

### 7.1 需要创建的文件

- `pkg/types/podcast.go` - Podcast 数据类型定义
- `app/store/sqlstore/podcast_store.go` - Podcast 数据库操作
- `app/logic/v1/podcast.go` - Podcast 业务逻辑
- `app/logic/v1/process/podcast.go` - Podcast 异步处理
- `cmd/service/handler/podcast.go` - Podcast HTTP 处理器
- `pkg/ai/tts/tts.go` - TTS 服务接口
- `pkg/ai/tts/providers/openai.go` - OpenAI TTS 实现
- `pkg/queue/podcast_queue.go` - Podcast 任务队列
- `pkg/utils/audio.go` - 音频处理工具

### 7.2 需要修改的文件

- `pkg/types/tables.go` - 添加新表定义
- `app/store/store.go` - 注册新的 Store
- `cmd/service/router.go` - 添加路由
- `app/core/config.go` - 添加 TTS 相关配置

### 7.3 数据库表文件

- `app/store/sqlstore/podcast.sql` - Podcast 表定义

## 8. 技术栈总结

- **后端**: Go 1.23.1+
- **数据库**: PostgreSQL
- **存储**: S3 兼容存储
- **队列**: Redis
- **音频处理**: ffmpeg
- **TTS 服务**: OpenAI TTS / Azure TTS / ElevenLabs（可扩展）
- **API**: RESTful
- **实时通知**: WebSocket

## 9. 附录

### 9.1 TTS 服务商对比

| 服务商           | 优势                | 劣势                   | 定价        |
| ---------------- | ------------------- | ---------------------- | ----------- |
| OpenAI TTS       | API 简单，质量好    | 语音选择较少           | $15/1M 字符 |
| Azure TTS        | 语音丰富，支持 SSML | 配置复杂               | 按分钟计费  |
| ElevenLabs       | 声音自然，可定制    | 价格较高               | 按字符计费  |
| Google Cloud TTS | 语言支持广          | 需要 Google Cloud 账号 | 按字符计费  |

### 9.2 音频格式对比

| 格式 | 优势               | 劣势           | 推荐场景 |
| ---- | ------------------ | -------------- | -------- |
| MP3  | 兼容性好，压缩率高 | 有损压缩       | 通用场景 |
| M4A  | Apple 优化，质量好 | 部分设备不支持 | iOS 用户 |
| OGG  | 开源，质量好       | 兼容性一般     | Web 场景 |

## 10. 与现有架构的集成

### 10.1 与 Model Config 系统的关系

TTS 配置已集成到 `core.srv` 中,使用现有的 model provider 和 model config 系统进行管理。

### 10.3 数据库表更新

需要在 `pkg/types/tables.go` 中添加：

```go
const (
    // ... 现有表定义
    TABLE_PODCASTS    = TableName("podcasts")
)
```

## 11. 总结

Podcast Hub 功能将为 QukaAI 用户提供全新的内容消费方式，让知识库内容"活"起来。通过文字转语音技术，用户可以在各种场景下收听自己的知识库内容，提升记忆效果和学习体验。

### 11.1 设计亮点

1. **与现有架构完美集成**：复用 model_provider/model_config 系统，实现系统级和用户级的两层配置
2. **灵活的配置管理**：用户可以保存多套 TTS 配置，满足不同场景需求
3. **异步处理架构**：基于队列的任务处理，提供良好的用户体验
4. **可扩展的 TTS 抽象层**：支持多种 TTS 服务商，易于扩展

### 11.2 与现有系统的关系

- **复用** `model_provider` 和 `model_config` 表进行系统级 TTS 配置
- **新增** `podcasts` 表管理播客内容
- **复用** 现有的队列系统（`pkg/queue`）处理异步任务
- **复用** 现有的 S3 存储系统管理音频文件

本设计文档提供了完整的功能规划、数据模型、API 设计和实施计划。在实际开发过程中，可以根据用户反馈和技术调研结果进行调整优化。

---

**文档状态**: 设计完成
**创建时间**: 2025-12-13
**最后更新**: 2025-12-13
**负责人**: Claude Code
**审核状态**: 待审核

**变更记录**:

- 2025-12-13: 研究现有 model_config 架构，优化 TTS 配置设计，采用两层配置架构
- 2025-12-13: 移除播放列表和播放进度功能，简化为纯内容转播客功能
