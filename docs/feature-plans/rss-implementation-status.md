# RSS 订阅功能实现状态

## 已完成

### 1. 数据库表定义 ✅

- [x] `app/store/sqlstore/rss_subscription.sql`
- [x] `app/store/sqlstore/rss_article.sql`
- [x] `app/store/sqlstore/rss_user_interest.sql`

### 2. 数据类型定义 ✅

- [x] `pkg/types/rss.go`
  - RSSSubscription
  - RSSArticle
  - RSSUserInterest
  - RSSFeedItem
  - RSSFeed

### 3. 数据访问层 ✅

- [x] `app/store/sqlstore/rss_subscription_store.go`
- [x] `app/store/sqlstore/rss_article_store.go`
- [x] `app/store/sqlstore/rss_user_interest_store.go`
- [x] 在 `app/store/store.go` 中注册新的 Store 接口
- [x] 在 `app/store/sqlstore/provider.go` 中注册新的 Store 实现

### 4. RSS 解析和抓取 ✅

- [x] `pkg/rss/parser.go` - RSS/Atom 格式解析（使用 gofeed 库）
- [x] `pkg/rss/fetcher.go` - HTTP 抓取器（带重试机制）

### 5. 内容处理 ✅

- [x] `pkg/rss/processor.go`
  - 文章去重
  - 创建 Knowledge 记录
  - 自动过期管理
  - 用户兴趣模型更新

### 6. 业务逻辑层 ✅

- [x] `app/logic/v1/rss_subscription.go`
  - 订阅 CRUD（创建、查询、更新、删除）
  - 手动触发抓取
  - 权限检查
- [x] `app/logic/v1/rss_fetcher.go`
  - 单个订阅抓取
  - 批量抓取所有启用的订阅
  - 旧文章清理

## 待实现

### 7. API 层 ✅

- [x] `cmd/service/handler/rss.go` - HTTP 处理器
  - CreateRSSSubscription - 创建 RSS 订阅
  - GetRSSSubscription - 获取订阅详情
  - ListRSSSubscriptions - 获取订阅列表
  - UpdateRSSSubscription - 更新订阅配置
  - DeleteRSSSubscription - 删除订阅
  - TriggerRSSFetch - 手动触发抓取
- [x] 在路由中注册 RSS API

### 8. AI 智能摘要 ✅

**设计文档**: [rss-ai-summary-design.md](./rss-ai-summary-design.md)

#### 核心设计

- **摘要存储**: 在 `RSSArticle` 表存储共享的 AI 摘要（所有订阅用户共享，节省成本）
- **Knowledge 关联**: 每个用户通过 `Knowledge.rel_doc_id` 关联到文章，拥有个人副本
- **双向查询**:
  - 文章 → 用户的 Knowledge（通过 `rel_doc_id` 查询）
  - 用户列表 → 包含 `knowledge_id`（LEFT JOIN）

#### 实现任务

数据库修改:

- [x] 修改 `quka_rss_articles` 表添加摘要字段
  - `summary TEXT` - AI 生成的摘要
  - `keywords TEXT[]` - 关键词
  - `summary_generated_at BIGINT` - 生成时间
  - `ai_model VARCHAR(128)` - AI 模型
  - 创建索引用于快速查询没有摘要的文章
  - 迁移脚本: `app/store/sqlstore/migrations/rss_add_summary_fields.sql`

代码实现:

- [x] 更新 `types.RSSArticle` 数据结构（添加摘要字段）
- [x] 创建 RSS 专用摘要生成器 `pkg/rss/summarizer.go`
  - RSS 专用 Prompt（中英文）
  - 单篇文章摘要生成
  - 批量摘要生成（并发控制）
- [x] 新增 Processor 方法
  - `GenerateArticleSummary()` - 为单篇文章生成摘要
  - `BatchGenerateArticleSummaries()` - 批量生成摘要
- [x] 新增 Store 方法
  - `RSSArticleStore.UpdateSummary()` - 更新文章摘要
  - `RSSArticleStore.ListWithoutSummary()` - 获取没有摘要的文章
- [x] Knowledge 创建流程（无需修改）
  - ✅ 现有流程已完善：Article → Knowledge (SUMMARIZE) → 分块 → 向量化
  - ✅ RSSArticle.summary 用于快速预览（共享）
  - ✅ Knowledge 分块用于语义搜索（用户隔离）
  - ✅ 两层摘要各司其职，互不冲突
- [x] 集成到抓取流程 ✅
  - ✅ 文章抓取后异步触发摘要生成（rss_fetcher.go:141）
  - ✅ 支持多用户订阅同一文章，摘要共享
  - ✅ 自动检测文章是否已存在，避免重复生成摘要
  - ✅ 检查用户是否已有 Knowledge，避免重复创建

优化:

- [x] 批量生成摘要（提高效率） ✅
  - ✅ 已实现 BatchGenerateSummaries
  - ✅ 支持并发控制（默认 3 个并发）
- [x] 摘要生成失败重试机制 ✅
  - ✅ 添加重试次数字段 summary_retry_times
  - ✅ 记录错误信息 last_summary_error
  - ✅ 最大重试次数限制（3 次）
  - ✅ ListWithoutSummary 自动过滤重试次数过多的文章
  - ✅ IncrementSummaryRetry 方法自动增加重试计数
- [x] 记录 AI Token 使用量 ✅
  - ✅ Article 表添加 user_id 字段，记录最初订阅用户
  - ✅ Token 使用量记录到 quka_ai_token_usage 表（而非 Article 表）
  - ✅ 单篇生成时自动记录 Token（GenerateArticleSummaryWithTokenTracking）
  - ✅ 批量生成时统计并记录每篇文章的 Token 消耗
  - ✅ Token 归属到最初订阅该文章的用户
  - ✅ 日志输出详细的 Token 使用情况

### 9. 每日摘要（Daily Digest）✅

**设计文档**: [rss-daily-digest-api.md](../api-documentation/rss-daily-digest-api.md)

#### 核心功能

- **智能整合**: AI 自动将用户订阅的所有 RSS 文章按主题分类整合
- **一份报告**: 每天生成一份统一的 Markdown 格式摘要
- **主题识别**: 根据文章内容自动识别主题并建立关联
- **快速跳转**: 支持从摘要跳转到完整 Knowledge 内容

#### 实现任务

数据库:

- [x] 创建 `quka_rss_daily_digests` 表
  - `id, user_id, space_id, date, content, article_ids, article_count, ai_model, generated_at, created_at`
  - 唯一索引：(user_id, space_id, date)
  - 表文件: `app/store/sqlstore/rss_daily_digest.sql`

代码实现:

- [x] 创建 `pkg/rss/daily_digest.go` - DailyDigestGenerator
  - `GenerateDailyDigest()` - 生成指定日期的每日摘要
  - `BatchGenerateDailyDigests()` - 批量为多个用户生成
  - 每日摘要专用 Prompt（中英文）
  - 主题自动分类和文章归组
- [x] 创建 RSSDailyDigestStore
  - `app/store/sqlstore/rss_daily_digest_store.go`
  - `Create()`, `Get()`, `GetByUserAndDate()`, `ListByUser()`, `Delete()`
  - `ListByDateRange()`, `Exists()`, `DeleteOld()`
- [x] 更新 Store 接口
  - `app/store/store.go` - 添加 RSSDailyDigestStore 接口
  - `app/store/sqlstore/provider.go` - 注册 Store 实现
- [x] 数据类型定义
  - `pkg/types/rss.go` - RSSDailyDigest, RSSDigestArticle
  - `pkg/types/tables.go` - TABLE_RSS_DAILY_DIGESTS
- [x] 辅助方法
  - `RSSArticleStore.ListByDateRange()` - 按日期范围查询文章
  - `KnowledgeStore.GetByRelDocID()` - 通过 rel_doc_id 查找 Knowledge

API 实现:

- [x] `POST /api/v1/rss/digest/generate` - 手动触发生成摘要
- [x] `GET /api/v1/rss/digest/daily` - 获取每日摘要（不存在则生成）
- [x] `GET /api/v1/rss/digest/history` - 获取历史摘要列表
- [x] `GET /api/v1/rss/digest/:id` - 获取摘要详情
- [x] `DELETE /api/v1/rss/digest/:id` - 删除摘要

集成:

- [x] 定时任务：每天凌晨 4 点自动为所有用户生成前一天的摘要（UTC时间）
- [x] 时区统一：所有时区用户基于 UTC 昨天生成摘要，避免时区混乱
- [ ] 前端展示：每日摘要阅读界面

### 10. 定时任务 ✅

- [x] `app/logic/v1/process/rss_daily_digest.go`
  - 集成到现有 process 系统
  - 每天凌晨 4 点执行
  - 自动为所有用户生成前一天的摘要（UTC时间）
  - 时区统一：所有用户基于相同的 UTC 日期，避免时区混乱
  - 避免重复生成摘要
  - 支持空摘要处理
- [x] `app/logic/v1/process/rss_sync.go`
  - 定时同步需要更新的订阅（每 30 分钟执行）
  - 改为任务生产者：将订阅推送到 Redis 队列
  - 通过队列解耦 Logic 和 Process 层

### 11. Redis 队列重构 ✅

**设计文档**: [rss-queue-refactoring.md](./rss-queue-refactoring.md)

#### 核心改造

- **问题**: Logic 层不应该直接通过 goroutine 调用 Fetcher 抓取内容
- **方案**: 使用 Redis 队列实现生产者-消费者模式
- **架构**: Logic 层生产任务 → Redis 队列 → Process 层消费任务

#### 实现任务

队列基础设施:

- [x] `pkg/queue/rss_queue.go` - Redis 队列管理器
  - `EnqueueTask()` - 任务入队
  - `DequeueTask()` - 阻塞式出队（BLPOP）
  - `MarkTaskSuccess()` / `MarkTaskFailed()` - 任务状态管理
  - `RecoverTimeoutTasks()` - 超时任务恢复
  - `GetQueueStats()` - 队列监控
  - Redis 数据结构:
    - `rss:queue` (List) - 主任务队列
    - `rss:task:{id}` (Hash) - 任务元数据
    - `rss:processing` (Sorted Set) - 超时检测

消费者实现:

- [x] `app/logic/v1/process/rss_consumer.go` - 队列消费者
  - 启动 3 个并发 Worker
  - 阻塞式消费任务（BLPOP with 5s timeout）
  - 复用 Fetcher 核心逻辑
  - 任务失败重试机制（最多 3 次，延迟 5 秒）
  - 定时超时恢复（每分钟检查一次）

生产者改造:

- [x] `app/logic/v1/rss_subscription.go`
  - `CreateSubscription()` - 新建订阅时入队任务
  - `TriggerFetch()` - 手动触发时入队任务
  - 移除直接调用 Fetcher 的 goroutine

- [x] `app/logic/v1/process/rss_sync.go`
  - 定时任务改为生产者角色
  - 将需要更新的订阅推送到队列

监控 API:

- [x] `cmd/service/handler/rss.go`
  - `GET /api/v1/rss/queue/stats` - 队列统计信息
  - `GET /api/v1/rss/queue/task` - 任务状态查询

废弃代码:

- [x] `app/logic/v1/rss_fetcher.go`
  - 标记旧方法为 Deprecated
  - `FetchSubscription()`, `FetchAllEnabledSubscriptions()`, `FetchSubscriptionsNeedingUpdate()`
  - 清理不可达代码

### 12. Redis 配置重构 ✅

**设计文档**: [redis-configuration-refactoring.md](./redis-configuration-refactoring.md)

- [x] 独立 Redis 配置（从 Centrifuge 配置中解耦）
- [x] 支持单机模式和集群模式
- [x] 支持账号密码配置
- [x] 使用 `redis.UniversalClient` 接口统一处理
- [x] 连接池配置（PoolSize, MinIdleConns, MaxRetries）
- [x] 超时配置（DialTimeout, ReadTimeout, WriteTimeout）
- [x] 验证集群模式支持队列操作

### 13. 测试

- [ ] 单元测试
- [ ] 集成测试

## 依赖库

需要添加到 go.mod:

```
github.com/mmcdole/gofeed v1.2.1  // RSS 解析
github.com/robfig/cron/v3 v3.0.1  // 定时任务
```

## 下一步

建议按以下顺序继续实现：

1. 实现推送通知：摘要生成完成后通知用户
2. 前端展示：每日摘要阅读界面
3. 编写测试
   - 单元测试
   - 集成测试
4. 性能优化
   - 批量摘要生成优化
   - 缓存机制
