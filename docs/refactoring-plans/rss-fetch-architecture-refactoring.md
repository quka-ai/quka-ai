# RSS 订阅抓取架构重构计划

## 1. 问题背景

当前 RSS 订阅系统存在架构不合理的问题：

### 1.1 当前架构问题

#### 问题描述

在 `RSSSubscriptionLogic.CreateSubscription()` 中（[rss_subscription.go:98-104](app/logic/v1/rss_subscription.go#L98-L104)），当用户创建新订阅时，代码会立即启动 goroutine 调用 `RSSFetcherLogic.FetchSubscription()` 来同步抓取内容：

```go
// 立即触发一次抓取
go func() {
    fetchLogic := NewRSSFetcherLogic(context.Background(), l.core)
    if err := fetchLogic.FetchSubscription(subscription.ID); err != nil {
        // 记录错误但不阻塞创建流程
        fmt.Printf("Failed to fetch new subscription: %v\n", err)
    }
}()
```

同样，在 `TriggerFetch()` 方法中（[rss_subscription.go:249-255](app/logic/v1/rss_subscription.go#L249-L255)）也存在相同问题。

#### 核心问题

1. **职责混乱**：

   - `RSSFetcherLogic` 应该只包含 RSS 抓取的**核心逻辑**（如何抓取、如何解析）
   - 但实际上它承担了**调度逻辑**（什么时候抓取、抓取哪些订阅）
   - `RSSSubscriptionLogic` 不应该直接调用抓取逻辑，这违反了分层架构原则

2. **重复代码**：

   - `RSSFetcherLogic.FetchSubscription()` 中的核心抓取逻辑与 `process/rss_sync.go` 中的 `processSubscription()` 几乎完全重复
   - 两处都在处理：抓取 Feed、遍历文章、创建 Knowledge、更新时间戳

3. **缺乏统一触发机制**：

   - 新订阅创建时通过 goroutine 直接调用 `RSSFetcherLogic.FetchSubscription()`
   - 定时任务通过 `process/rss_sync.go` 处理
   - 手动触发也通过 goroutine 调用 `RSSFetcherLogic.FetchSubscription()`
   - 三种触发方式没有统一入口，难以管理和监控

4. **缺少任务队列**：
   - 直接启动 goroutine 可能导致大量并发抓取请求
   - 没有优先级、限流、重试等机制
   - 无法追踪任务状态和结果

### 1.2 理想架构

正确的架构应该是：

```
用户操作（创建订阅/手动触发）
    ↓
RSSSubscriptionLogic（业务逻辑层）
    ↓
【Redis任务队列】← 引入统一的任务队列！
    ↓
Process/Consumer（后台消费者）
    ↓
核心抓取逻辑
```

**核心原则**：

- Logic 层只负责业务逻辑和数据验证
- 任务生产者（Logic 层）通过 Redis 队列发布任务
- 任务消费者（Process 层）从队列消费任务并执行
- 完全解耦，支持分布式扩展

---

## 2. 重构目标

### 2.1 架构目标

1. **分离关注点**：Logic 层不直接调用抓取，而是发布任务到 Redis 队列
2. **消除重复代码**：只在 Process 层保留一套完整的抓取逻辑
3. **统一触发机制**：所有抓取需求都通过 Redis 队列
4. **可追踪和监控**：能够查询队列状态、任务进度、失败原因

### 2.2 功能目标

1. 新订阅创建后**秒级**触发首次抓取
2. 定时任务定期检查需要更新的订阅并推送到队列
3. 用户可以手动触发单个订阅的抓取（秒级响应）
4. 支持失败重试和错误追踪
5. 支持分布式部署（多个消费者并发处理）
6. 公平调度：所有订阅按 FIFO 顺序处理，一视同仁

---

## 3. 技术方案：Redis 任务队列

### 3.1 方案选择

选择 **Redis List（单队列）** 实现：

**实现方式**：

- 使用单个 Redis List 作为任务队列：
  - `rss:queue` - 统一的任务队列（FIFO，先进先出）
- 使用 Redis Hash 存储任务状态和元数据：
  - `rss:task:{subscription_id}` - 任务详情
- 使用 Redis Sorted Set 记录正在处理的任务（用于超时检测）：
  - `rss:processing` - 正在处理的任务及其开始时间

**优点**：

- ✅ **实时性好**：任务推送到队列后立即被消费，无延迟
- ✅ **简单直观**：单队列 FIFO，逻辑清晰，易于理解和维护
- ✅ **性能好**：Redis 操作高效，支持高并发
- ✅ **易于扩展**：支持多个消费者并发处理任务
- ✅ **任务持久化**：Redis 可以配置 RDB/AOF 持久化，重启不丢任务
- ✅ **公平调度**：所有订阅一视同仁，按创建顺序处理

**缺点**：

- ❌ 需要实现队列消费逻辑（但复杂度可控）
- ❌ 需要实现超时检测和重试机制（但可以复用现有逻辑）

### 3.2 为什么不使用 Asynq 等第三方库

虽然 Asynq 等任务队列库功能完善，但考虑到：

1. QukaAI 项目追求简单轻量，不希望引入过多依赖
2. RSS 抓取任务的需求相对简单，不需要复杂的任务调度
3. 自己实现可以更好地控制逻辑和性能
4. 实现难度不高（约 150 行代码）

---

## 4. 详细实施步骤

### 4.1 Redis 队列设计

#### 4.1.1 数据结构设计

**1. 任务队列（Redis List）**

```
# 统一任务队列
Key: rss:queue
Type: List
Value: JSON serialized task
Example:
{
  "subscription_id": 123,
  "created_at": 1702887600,
  "retry_count": 0
}
```

**2. 任务元数据（Redis Hash）**

```
# 任务详情（用于追踪状态）
Key: rss:task:{subscription_id}
Type: Hash
Fields:
  - status: pending/processing/success/failed
  - created_at: 任务创建时间
  - started_at: 开始处理时间
  - finished_at: 完成时间
  - error: 错误信息（失败时）
  - retry_count: 重试次数
  - worker_id: 处理该任务的worker ID

# TTL: 任务完成后1小时自动删除
```

**3. 正在处理的任务（Redis Sorted Set）**

```
# 用于超时检测
Key: rss:processing
Type: Sorted Set
Member: subscription_id
Score: 开始处理的时间戳

# 消费者定期扫描这个集合，如果某个任务处理时间超过15分钟，
# 认为worker已挂掉，将任务重新放回队列
```

#### 4.1.2 队列操作接口

在 `pkg/queue/rss_queue.go` 中实现队列操作：

```go
package queue

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
    "log/slog"
)

const (
    QueueKey         = "rss:queue"
    ProcessingSetKey = "rss:processing"
    TaskKeyPrefix    = "rss:task:"

    TaskStatusPending    = "pending"
    TaskStatusProcessing = "processing"
    TaskStatusSuccess    = "success"
    TaskStatusFailed     = "failed"

    TaskTimeout = 15 * time.Minute  // 任务超时时间
    TaskTTL     = 1 * time.Hour     // 任务元数据保留时间
)

// RSSFetchTask RSS抓取任务
type RSSFetchTask struct {
    SubscriptionID int64 `json:"subscription_id"`
    CreatedAt      int64 `json:"created_at"`
    RetryCount     int   `json:"retry_count"`
}

// RSSQueue RSS任务队列管理器
type RSSQueue struct {
    redis    *redis.Client
    workerID string // 当前worker的唯一ID
}

// NewRSSQueue 创建队列管理器
func NewRSSQueue(redisClient *redis.Client, workerID string) *RSSQueue {
    return &RSSQueue{
        redis:    redisClient,
        workerID: workerID,
    }
}

// EnqueueTask 将任务加入队列
func (q *RSSQueue) EnqueueTask(ctx context.Context, subscriptionID int64) error {
    task := RSSFetchTask{
        SubscriptionID: subscriptionID,
        CreatedAt:      time.Now().Unix(),
        RetryCount:     0,
    }

    taskJSON, err := json.Marshal(task)
    if err != nil {
        return fmt.Errorf("failed to marshal task: %w", err)
    }

    // 推送到队列（右推左弹，保证FIFO）
    if err := q.redis.RPush(ctx, QueueKey, taskJSON).Err(); err != nil {
        return fmt.Errorf("failed to enqueue task: %w", err)
    }

    // 更新任务元数据
    taskKey := fmt.Sprintf("%s%d", TaskKeyPrefix, subscriptionID)
    if err := q.redis.HSet(ctx, taskKey,
        "status", TaskStatusPending,
        "created_at", task.CreatedAt,
        "retry_count", 0,
    ).Err(); err != nil {
        return fmt.Errorf("failed to set task metadata: %w", err)
    }

    // 设置TTL
    q.redis.Expire(ctx, taskKey, TaskTTL)

    slog.Info("Task enqueued",
        slog.Int64("subscription_id", subscriptionID))

    return nil
}

// DequeueTask 从队列取出任务（阻塞式）
func (q *RSSQueue) DequeueTask(ctx context.Context, timeout time.Duration) (*RSSFetchTask, error) {
    // 从队列取任务（阻塞式，FIFO）
    result, err := q.redis.BLPop(ctx, timeout, QueueKey).Result()
    if err != nil {
        if err == redis.Nil {
            return nil, nil // 队列为空
        }
        return nil, fmt.Errorf("failed to dequeue task: %w", err)
    }

    // result[0] 是队列名，result[1] 是任务数据
    var task RSSFetchTask
    if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
        return nil, fmt.Errorf("failed to unmarshal task: %w", err)
    }

    // 标记任务为处理中
    taskKey := fmt.Sprintf("%s%d", TaskKeyPrefix, task.SubscriptionID)
    now := time.Now().Unix()

    if err := q.redis.HSet(ctx, taskKey,
        "status", TaskStatusProcessing,
        "started_at", now,
        "worker_id", q.workerID,
    ).Err(); err != nil {
        slog.Error("Failed to mark task as processing", slog.String("error", err.Error()))
    }

    // 添加到processing set（用于超时检测）
    if err := q.redis.ZAdd(ctx, ProcessingSetKey, redis.Z{
        Score:  float64(now),
        Member: task.SubscriptionID,
    }).Err(); err != nil {
        slog.Error("Failed to add task to processing set", slog.String("error", err.Error()))
    }

    slog.Info("Task dequeued",
        slog.Int64("subscription_id", task.SubscriptionID))

    return &task, nil
}

// MarkTaskSuccess 标记任务成功
func (q *RSSQueue) MarkTaskSuccess(ctx context.Context, subscriptionID int64) error {
    taskKey := fmt.Sprintf("%s%d", TaskKeyPrefix, subscriptionID)
    now := time.Now().Unix()

    if err := q.redis.HSet(ctx, taskKey,
        "status", TaskStatusSuccess,
        "finished_at", now,
    ).Err(); err != nil {
        return fmt.Errorf("failed to mark task success: %w", err)
    }

    // 从processing set中移除
    q.redis.ZRem(ctx, ProcessingSetKey, subscriptionID)

    // 设置较短的TTL（成功的任务不需要长期保留）
    q.redis.Expire(ctx, taskKey, 10*time.Minute)

    slog.Info("Task marked as success", slog.Int64("subscription_id", subscriptionID))
    return nil
}

// MarkTaskFailed 标记任务失败
func (q *RSSQueue) MarkTaskFailed(ctx context.Context, subscriptionID int64, errMsg string, shouldRetry bool) error {
    taskKey := fmt.Sprintf("%s%d", TaskKeyPrefix, subscriptionID)
    now := time.Now().Unix()

    // 获取当前重试次数
    retryCount, _ := q.redis.HGet(ctx, taskKey, "retry_count").Int()

    if err := q.redis.HSet(ctx, taskKey,
        "status", TaskStatusFailed,
        "finished_at", now,
        "error", errMsg,
        "retry_count", retryCount+1,
    ).Err(); err != nil {
        return fmt.Errorf("failed to mark task failed: %w", err)
    }

    // 从processing set中移除
    q.redis.ZRem(ctx, ProcessingSetKey, subscriptionID)

    // 如果需要重试且重试次数未超限（最多3次）
    if shouldRetry && retryCount < 3 {
        slog.Info("Task failed, will retry",
            slog.Int64("subscription_id", subscriptionID),
            slog.Int("retry_count", retryCount+1))

        // 重新入队
        time.Sleep(5 * time.Second) // 延迟5秒后重试
        return q.EnqueueTask(ctx, subscriptionID)
    }

    slog.Error("Task failed permanently",
        slog.Int64("subscription_id", subscriptionID),
        slog.Int("retry_count", retryCount+1),
        slog.String("error", errMsg))

    return nil
}

// RecoverTimeoutTasks 恢复超时任务（由定时任务定期调用）
func (q *RSSQueue) RecoverTimeoutTasks(ctx context.Context) error {
    now := time.Now().Unix()
    timeoutThreshold := now - int64(TaskTimeout.Seconds())

    // 查询超时的任务
    timeoutTasks, err := q.redis.ZRangeByScore(ctx, ProcessingSetKey, &redis.ZRangeBy{
        Min: "0",
        Max: fmt.Sprintf("%d", timeoutThreshold),
    }).Result()

    if err != nil {
        return fmt.Errorf("failed to query timeout tasks: %w", err)
    }

    if len(timeoutTasks) == 0 {
        return nil
    }

    slog.Warn("Found timeout tasks, recovering",
        slog.Int("count", len(timeoutTasks)))

    for _, taskIDStr := range timeoutTasks {
        var subscriptionID int64
        fmt.Sscanf(taskIDStr, "%d", &subscriptionID)

        // 重新入队
        if err := q.EnqueueTask(ctx, subscriptionID); err != nil {
            slog.Error("Failed to recover timeout task",
                slog.Int64("subscription_id", subscriptionID),
                slog.String("error", err.Error()))
            continue
        }

        // 从processing set中移除
        q.redis.ZRem(ctx, ProcessingSetKey, subscriptionID)

        slog.Info("Timeout task recovered",
            slog.Int64("subscription_id", subscriptionID))
    }

    return nil
}

// GetQueueStats 获取队列统计信息
func (q *RSSQueue) GetQueueStats(ctx context.Context) (map[string]int64, error) {
    queueLen, err := q.redis.LLen(ctx, QueueKey).Result()
    if err != nil {
        return nil, err
    }

    processingLen, err := q.redis.ZCard(ctx, ProcessingSetKey).Result()
    if err != nil {
        return nil, err
    }

    return map[string]int64{
        "queue_length": queueLen,
        "processing":   processingLen,
    }, nil
}
```

### 4.2 Process 层改造

#### 4.2.1 新增队列消费者

在 `app/logic/v1/process/rss_consumer.go` 中实现消费者：

```go
package process

import (
    "context"
    "fmt"
    "log/slog"
    "time"

    "github.com/quka-ai/quka-ai/app/core"
    "github.com/quka-ai/quka-ai/pkg/queue"
    "github.com/quka-ai/quka-ai/pkg/register"
    "github.com/quka-ai/quka-ai/pkg/rss"
    "github.com/quka-ai/quka-ai/pkg/utils"
)

func init() {
    register.RegisterFunc[*Process](ProcessKey{}, func(p *Process) {
        // 启动RSS任务消费者（3个并发worker）
        for i := 0; i < 3; i++ {
            workerID := fmt.Sprintf("worker-%d", i)
            go startRSSConsumer(p.Core(), workerID)
        }

        // 每1分钟检查一次超时任务
        p.Cron().AddFunc("*/1 * * * *", func() {
            recoverTimeoutTasks(p.Core())
        })

        slog.Info("RSS task consumers started", slog.Int("worker_count", 3))
    })
}

// startRSSConsumer 启动消费者（阻塞式运行）
func startRSSConsumer(core *core.Core, workerID string) {
    rssQueue := queue.NewRSSQueue(core.Redis(), workerID)
    fetcher := rss.NewFetcher()

    slog.Info("RSS consumer started", slog.String("worker_id", workerID))

    for {
        // 从队列取任务（30秒超时）
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        task, err := rssQueue.DequeueTask(ctx, 30*time.Second)
        cancel()

        if err != nil {
            slog.Error("Failed to dequeue task",
                slog.String("worker_id", workerID),
                slog.String("error", err.Error()))
            time.Sleep(5 * time.Second)
            continue
        }

        if task == nil {
            // 队列为空，等待下一次轮询
            continue
        }

        // 处理任务
        processCtx, processCancel := context.WithTimeout(context.Background(), 15*time.Minute)
        err = processRSSTask(processCtx, core, fetcher, rssQueue, task)
        processCancel()

        if err != nil {
            slog.Error("Failed to process RSS task",
                slog.String("worker_id", workerID),
                slog.Int64("subscription_id", task.SubscriptionID),
                slog.String("error", err.Error()))
        }

        // 短暂休息，避免过度请求
        time.Sleep(500 * time.Millisecond)
    }
}

// processRSSTask 处理单个RSS任务
func processRSSTask(ctx context.Context, core *core.Core, fetcher *rss.Fetcher, rssQueue *queue.RSSQueue, task *queue.RSSFetchTask) error {
    // 获取订阅信息
    subscription, err := core.Store().RSSSubscriptionStore().Get(ctx, task.SubscriptionID)
    if err != nil {
        // 订阅不存在或已删除，标记任务失败（不重试）
        rssQueue.MarkTaskFailed(ctx, task.SubscriptionID, fmt.Sprintf("subscription not found: %v", err), false)
        return err
    }

    // 检查是否启用
    if !subscription.Enabled {
        rssQueue.MarkTaskFailed(ctx, task.SubscriptionID, "subscription is disabled", false)
        return fmt.Errorf("subscription %d is disabled", task.SubscriptionID)
    }

    // 复用现有的processSubscription逻辑
    if err := processSubscription(ctx, core, fetcher, subscription); err != nil {
        // 标记失败，允许重试
        rssQueue.MarkTaskFailed(ctx, task.SubscriptionID, err.Error(), true)
        return err
    }

    // 标记成功
    rssQueue.MarkTaskSuccess(ctx, task.SubscriptionID)
    return nil
}

// recoverTimeoutTasks 恢复超时任务
func recoverTimeoutTasks(core *core.Core) {
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
    defer cancel()

    rssQueue := queue.NewRSSQueue(core.Redis(), "timeout-checker")
    if err := rssQueue.RecoverTimeoutTasks(ctx); err != nil {
        slog.Error("Failed to recover timeout tasks", slog.String("error", err.Error()))
    }
}
```

#### 4.2.2 保留定时任务（作为任务生产者）

修改 `app/logic/v1/process/rss_sync.go`，改为定时推送任务到队列：

```go
package process

import (
    "context"
    "log/slog"
    "time"

    "github.com/quka-ai/quka-ai/app/core"
    "github.com/quka-ai/quka-ai/pkg/queue"
    "github.com/quka-ai/quka-ai/pkg/register"
)

func init() {
    register.RegisterFunc[*Process](ProcessKey{}, func(p *Process) {
        // 每5分钟检查一次需要更新的订阅，并推送到队列
        p.Cron().AddFunc("*/5 * * * *", func() {
            enqueueSubscriptionsNeedingUpdate(p.Core())
        })

        slog.Info("RSS sync scheduler registered: runs every 5 minutes")
    })
}

// enqueueSubscriptionsNeedingUpdate 将需要更新的订阅推送到队列
func enqueueSubscriptionsNeedingUpdate(core *core.Core) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    slog.Debug("Checking subscriptions needing update")

    // 获取需要更新的订阅（最多100个）
    subscriptions, err := core.Store().RSSSubscriptionStore().GetSubscriptionsNeedingUpdate(ctx, 100)
    if err != nil {
        slog.Error("Failed to get subscriptions needing update", slog.String("error", err.Error()))
        return
    }

    if len(subscriptions) == 0 {
        slog.Debug("No subscriptions need update")
        return
    }

    slog.Info("Found subscriptions needing update, enqueuing tasks",
        slog.Int("count", len(subscriptions)))

    rssQueue := queue.NewRSSQueue(core.Redis(), "scheduler")

    successCount := 0
    failedCount := 0

    for _, subscription := range subscriptions {
        // 推送到队列
        if err := rssQueue.EnqueueTask(ctx, subscription.ID); err != nil {
            failedCount++
            slog.Error("Failed to enqueue task",
                slog.Int64("subscription_id", subscription.ID),
                slog.String("error", err.Error()))
        } else {
            successCount++
        }
    }

    slog.Info("Finished enqueuing tasks",
        slog.Int("total", len(subscriptions)),
        slog.Int("success", successCount),
        slog.Int("failed", failedCount))
}
```

### 4.3 Logic 层改造

#### 4.3.1 修改 `CreateSubscription()`

修改 `app/logic/v1/rss_subscription.go`：

```go
// CreateSubscription 创建RSS订阅
func (l *RSSSubscriptionLogic) CreateSubscription(spaceID, resourceID, url, title, description, category string, updateFrequency int) (*types.RSSSubscription, error) {
    // ... 现有验证逻辑保持不变 ...

    subscription := &types.RSSSubscription{
        ID:              utils.GenUniqID(),
        UserID:          user.User,
        SpaceID:         spaceID,
        ResourceID:      resourceID,
        URL:             url,
        Title:           title,
        Description:     description,
        Category:        category,
        UpdateFrequency: updateFrequency,
        Enabled:         true,
        CreatedAt:       time.Now().Unix(),
        UpdatedAt:       time.Now().Unix(),
    }

    if err := l.core.Store().RSSSubscriptionStore().Create(l.ctx, subscription); err != nil {
        return nil, errors.New("RSSSubscriptionLogic.CreateSubscription.RSSSubscriptionStore.Create", i18n.ERROR_INTERNAL, err)
    }

    // 🔑 推送任务到队列
    rssQueue := queue.NewRSSQueue(l.core.Redis(), "api")
    if err := rssQueue.EnqueueTask(l.ctx, subscription.ID); err != nil {
        // 入队失败不阻塞订阅创建，记录日志即可
        slog.Error("Failed to enqueue fetch task for new subscription",
            slog.Int64("subscription_id", subscription.ID),
            slog.String("error", err.Error()))
    } else {
        slog.Info("New subscription fetch task enqueued",
            slog.Int64("subscription_id", subscription.ID))
    }

    return subscription, nil
}
```

#### 4.3.2 修改 `TriggerFetch()`

```go
// TriggerFetch 手动触发订阅抓取
func (l *RSSSubscriptionLogic) TriggerFetch(id int64) error {
    // ... 现有权限检查代码保持不变 ...

    // 🔑 推送任务到队列
    rssQueue := queue.NewRSSQueue(l.core.Redis(), "api")
    if err := rssQueue.EnqueueTask(l.ctx, id); err != nil {
        return errors.New("RSSSubscriptionLogic.TriggerFetch.EnqueueTask", i18n.ERROR_INTERNAL, err)
    }

    slog.Info("Manual fetch task enqueued",
        slog.Int64("subscription_id", id))

    return nil
}
```

### 4.4 清理 `RSSFetcherLogic`

可以删除或标记废弃以下方法：

```go
// @Deprecated: 使用队列机制替代，该方法将在下个版本移除
func (l *RSSFetcherLogic) FetchSubscription(subscriptionID int64) error {
    return fmt.Errorf("deprecated: use queue mechanism instead")
}

// @Deprecated
func (l *RSSFetcherLogic) FetchAllEnabledSubscriptions() error {
    return fmt.Errorf("deprecated: use queue mechanism instead")
}

// @Deprecated
func (l *RSSFetcherLogic) FetchSubscriptionsNeedingUpdate(limit int) error {
    return fmt.Errorf("deprecated: use queue mechanism instead")
}
```

保留以下有用的方法：

- `GetArticlesBySubscription()` - 查询方法
- `CleanupOldArticles()` - 清理逻辑
- `UpdateUserInterests()` - 用户兴趣模型

---

## 5. 实施顺序

### 阶段一：队列基础设施（独立可测试）

1. ✅ 实现 `pkg/queue/rss_queue.go`
2. ✅ 编写单元测试
3. ✅ 验证 Redis 操作正确性

### 阶段二：消费者实现（不影响现有功能）

1. ✅ 实现 `app/logic/v1/process/rss_consumer.go`
2. ✅ 复用现有的 `processSubscription()` 逻辑
3. ✅ 测试消费者能否正确处理任务

### 阶段三：定时任务改造（关键变更）

1. ✅ 修改 `rss_sync.go`，改为推送任务而非直接抓取
2. ✅ 测试定时任务能否正确入队

### 阶段四：Logic 层改造（用户可见变更）

1. ✅ 修改 `CreateSubscription()` 和 `TriggerFetch()`
2. ✅ 测试新订阅创建和手动触发流程
3. ✅ 验证响应速度（应在秒级）

### 阶段五：清理和优化

1. ✅ 废弃 `RSSFetcherLogic` 中的重复代码
2. ✅ 更新 API 文档
3. ✅ 进行全面的集成测试

### 阶段六：监控和调优

1. ✅ 增加队列监控 API（查询队列长度、处理中任务数）
2. ✅ 观察生产环境表现
3. ✅ 根据负载调整 worker 数量

---

## 6. 监控和管理 API

### 6.1 队列状态查询 API

在 `cmd/service/handler/rss.go` 中增加管理接口：

```go
// GetRSSQueueStatsResponse 队列统计响应
type GetRSSQueueStatsResponse struct {
    QueueLength     int64 `json:"queue_length"`
    ProcessingCount int64 `json:"processing_count"`
    WorkerCount     int   `json:"worker_count"`
}

func (s *HttpSrv) GetRSSQueueStats(c *gin.Context) {
    rssQueue := queue.NewRSSQueue(s.Core.Redis(), "api")
    stats, err := rssQueue.GetQueueStats(c)
    if err != nil {
        response.APIError(c, err)
        return
    }

    response.APISuccess(c, GetRSSQueueStatsResponse{
        QueueLength:     stats["queue_length"],
        ProcessingCount: stats["processing"],
        WorkerCount:     3, // 可以从配置读取
    })
}
```

### 6.2 任务状态查询 API

```go
// GetRSSTaskStatusRequest 任务状态查询请求
type GetRSSTaskStatusRequest struct {
    SubscriptionID int64 `json:"subscription_id" form:"subscription_id" binding:"required"`
}

// GetRSSTaskStatusResponse 任务状态响应
type GetRSSTaskStatusResponse struct {
    Status     string `json:"status"`    // pending/processing/success/failed
    CreatedAt  int64  `json:"created_at"`
    StartedAt  int64  `json:"started_at"`
    FinishedAt int64  `json:"finished_at"`
    Error      string `json:"error,omitempty"`
    RetryCount int    `json:"retry_count"`
    WorkerID   string `json:"worker_id,omitempty"`
}

func (s *HttpSrv) GetRSSTaskStatus(c *gin.Context) {
    var req GetRSSTaskStatusRequest
    if err := utils.BindArgsWithGin(c, &req); err != nil {
        response.APIError(c, err)
        return
    }

    taskKey := fmt.Sprintf("rss:task:%d", req.SubscriptionID)
    taskData, err := s.Core.Redis().HGetAll(c, taskKey).Result()
    if err != nil || len(taskData) == 0 {
        response.APIError(c, fmt.Errorf("task not found"))
        return
    }

    // 解析任务数据
    resp := GetRSSTaskStatusResponse{
        Status:   taskData["status"],
        Error:    taskData["error"],
        WorkerID: taskData["worker_id"],
    }

    // 解析时间戳
    if createdAt, err := strconv.ParseInt(taskData["created_at"], 10, 64); err == nil {
        resp.CreatedAt = createdAt
    }
    if startedAt, err := strconv.ParseInt(taskData["started_at"], 10, 64); err == nil {
        resp.StartedAt = startedAt
    }
    if finishedAt, err := strconv.ParseInt(taskData["finished_at"], 10, 64); err == nil {
        resp.FinishedAt = finishedAt
    }
    if retryCount, err := strconv.Atoi(taskData["retry_count"]); err == nil {
        resp.RetryCount = retryCount
    }

    response.APISuccess(c, resp)
}
```

---

## 7. 关键考虑点

### 7.1 性能和并发

**并发控制**：

- 默认启动 3 个 worker 并发消费
- 可以通过配置调整 worker 数量
- Redis 队列天然支持并发安全

**限流**：

- 每个 worker 处理完任务后休息 500ms
- 避免对 RSS 源服务器造成过大压力

### 7.2 容错和重试

**任务超时**：

- 单个任务最多执行 15 分钟
- 超时后由定时任务检测并重新入队

**失败重试**：

- 最多重试 3 次
- 重试时降级为普通优先级
- 失败任务保留 1 小时用于调试

**Worker 崩溃**：

- 任务在 processing set 中有记录
- 超时检测机制会自动恢复任务

### 7.3 数据一致性

**幂等性**：

- `processSubscription()` 中已有去重逻辑（通过 GUID 检查）
- 重复执行不会创建重复的 Knowledge 记录

**订阅删除**：

- 如果订阅被删除，消费者会检测到并标记任务失败（不重试）

### 7.4 扩展性

**水平扩展**：

- 可以在多个服务器上部署消费者
- 所有消费者共享同一个 Redis 队列
- Redis 天然支持分布式消费

**队列容量**：

- Redis List 理论上无限容量
- 实际受限于 Redis 内存
- 可以通过监控队列长度预警

---

## 8. 测试计划

### 8.1 单元测试

**队列操作测试**：

```go
func TestRSSQueue_EnqueueDequeue(t *testing.T) {
    // 测试入队出队
    // 测试优先级
}

func TestRSSQueue_MarkSuccess(t *testing.T) {
    // 测试成功标记
}

func TestRSSQueue_MarkFailed(t *testing.T) {
    // 测试失败标记和重试
}

func TestRSSQueue_RecoverTimeout(t *testing.T) {
    // 测试超时恢复
}
```

### 8.2 集成测试

**端到端流程测试**：

1. 创建新订阅 → 验证任务入队 → 验证消费者处理 → 验证 Knowledge 创建
2. 手动触发抓取 → 验证高优先级任务 → 验证立即处理
3. 定时任务触发 → 验证批量入队 → 验证顺序处理

**优先级测试**：

1. 同时创建高优先级和普通优先级任务
2. 验证高优先级任务先被处理

**失败重试测试**：

1. 模拟抓取失败
2. 验证重试机制
3. 验证最大重试次数限制

### 8.3 性能测试

**大批量订阅测试**：

- 创建 1000+订阅
- 同时推送大量任务到队列
- 观察消费速度和 Redis 性能

**并发测试**：

- 多个 worker 同时消费
- 验证无重复消费

---

## 9. 回滚方案

如果新架构出现问题，可以快速回滚：

### 9.1 临时回滚

1. 停止所有消费者 worker
2. 恢复 `CreateSubscription()` 和 `TriggerFetch()` 中的 goroutine 调用
3. Redis 队列中的任务会自然过期（或手动清空）

### 9.2 完整回滚

1. 回滚代码到重构前版本
2. 清空 Redis 中的队列数据（`rss:queue:*`、`rss:task:*`、`rss:processing`）

---

## 10. 后续优化方向

### 10.1 短期优化

**动态调整 worker 数量**：

- 根据队列长度动态增减 worker
- 负载低时减少 worker，负载高时增加

**更智能的重试策略**：

- 指数退避
- 根据错误类型决定是否重试

### 10.2 长期优化

**任务优先级细化**：

- 除了 high/normal，增加更多优先级级别
- 根据用户 VIP 等级调整优先级

**分布式锁**：

- 如果需要严格防止重复抓取，可以增加 Redis 分布式锁

**任务调度可视化**：

- 增加管理后台展示队列状态
- 展示任务处理历史和统计图表

---

## 11. 需要确认的问题

### 11.1 业务需求确认

1. **Worker 数量**：3 个并发 worker 是否合适？需要根据什么指标调整？
2. **任务超时时间**：15 分钟是否合理？
3. **失败重试次数**：最多 3 次是否合适？
4. **队列监控**：是否需要在管理后台展示队列状态？

### 11.2 技术实现确认

1. **Redis 持久化配置**：确认 Redis 是否已配置 AOF/RDB 持久化
2. **Worker 启动方式**：是否需要支持配置文件指定 worker 数量？
3. **日志级别**：队列操作日志是否需要可配置的级别（Debug/Info）？

---

## 12. 时间估算

| 阶段     | 工作内容                | 预计时间   |
| -------- | ----------------------- | ---------- |
| 阶段一   | 实现队列基础设施 + 测试 | 1 天       |
| 阶段二   | 实现消费者 + 测试       | 1 天       |
| 阶段三   | 改造定时任务 + 测试     | 0.5 天     |
| 阶段四   | 改造 Logic 层 + 测试    | 0.5 天     |
| 阶段五   | 清理代码和文档更新      | 0.5 天     |
| 阶段六   | 集成测试和监控 API      | 1 天       |
| **总计** |                         | **4.5 天** |

---

## 13. 结论

通过 Redis 单队列方案，我们实现了：

1. ✅ **完全解耦**：Logic 层和 Process 层通过队列通信，职责清晰
2. ✅ **秒级响应**：新订阅和手动触发立即入队，无延迟
3. ✅ **消除重复代码**：只保留一套抓取逻辑
4. ✅ **高可用**：支持多 worker 并发，支持失败重试和超时恢复
5. ✅ **易于监控**：可查询队列状态和任务状态
6. ✅ **水平扩展**：天然支持分布式部署
7. ✅ **公平调度**：FIFO 队列，所有订阅一视同仁，简单直观

相比方案 A（数据库轮询），Redis 队列方案的优势：

- **实时性**：秒级响应 vs 分钟级响应
- **性能**：Redis 操作高效，数据库压力小
- **并发能力**：支持多 worker 并发消费
- **代码简洁**：不需要在数据库中增加额外字段，单队列逻辑更清晰

---

**文档版本**：v1.1（Redis Single Queue - 简化版）
**创建时间**：2025-12-11
**最后更新**：2025-12-11
**状态**：待 Review ⏳
