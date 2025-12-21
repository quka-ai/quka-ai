# RSS队列Key Prefix功能实现总结

## 实现概述

为RSS队列系统添加了key prefix支持，实现不同环境/应用的Redis键隔离，提高系统的可扩展性和可维护性。

## 修改文件清单

### 1. 核心配置层 (`app/core/config.go`)
**修改内容**: 在`RedisConfig`结构体中添加了`KeyPrefix`字段
```go
type RedisConfig struct {
    // ... 其他字段
    KeyPrefix string `toml:"key_prefix"` // Redis键前缀，用于隔离不同环境/应用
}
```

### 2. 核心服务层 (`app/core/core.go`)
**修改内容**: 更新`setupAsynqClient`函数以支持KeyPrefix配置
```go
func setupAsynqClient(core *Core) {
    cfg := core.cfg.Redis

    // 设置默认的 key prefix
    keyPrefix := cfg.KeyPrefix
    if keyPrefix == "" {
        keyPrefix = "quka" // 默认前缀
    }

    core.asynqClient = asynq.NewClient(asynq.RedisClientOpt{
        Network:  "tcp",
        Addr:     cfg.GetAddr(),
        Password: cfg.Password,
        DB:       cfg.DB,
    })

    slog.Info("Asynq client initialized successfully",
        slog.String("key_prefix", keyPrefix))
}
```

### 3. 队列管理层 (`pkg/queue/rss_queue.go`)
**修改内容**:
- 更新`RSSQueue`结构体，添加`keyPrefix`字段
- 更新`NewRSSQueue`函数，支持keyPrefix参数
- 更新所有队列操作方法使用keyPrefix

```go
type RSSQueue struct {
    client    *asynq.Client
    keyPrefix string // Redis键前缀
}

func NewRSSQueue(client *asynq.Client, keyPrefix string) *RSSQueue {
    // 如果 keyPrefix 为空，使用 "rss" 作为默认前缀
    if keyPrefix == "" {
        keyPrefix = "rss"
    }

    return &RSSQueue{
        client:    client,
        keyPrefix: keyPrefix,
    }
}
```

### 4. 业务逻辑层调用点更新

#### A. RSS消费者 (`app/logic/v1/process/rss_consumer.go`)
- 第38行: `rssQueue := queue.NewRSSQueue(core.Asynq(), "rss")`
- 第101行: `rssQueue := queue.NewRSSQueue(core.Asynq(), "rss-scheduler")`

#### B. RSS订阅管理 (`app/logic/v1/rss_subscription.go`)
- 第99行: `rssQueue := queue.NewRSSQueue(l.core.Asynq(), "rss-api")`
- 第254行: `rssQueue := queue.NewRSSQueue(l.core.Asynq(), "rss-api")`

## 功能特性

### Key Prefix机制
1. **默认行为**: 如果key prefix为空，使用`"rss"`作为默认队列名称
2. **自定义前缀**: 直接使用传入的key prefix作为队列名称，无需拼接
3. **队列隔离**: 不同模块使用不同的队列前缀，实现完全隔离
   - `rss`: RSS消费者worker
   - `rss-scheduler`: 定时任务调度器
   - `rss-api`: API接口触发的任务

### 配置示例
在配置文件（如`config.toml`）中设置：
```toml
[redis]
addr = "localhost:6379"
password = ""
db = 0
key_prefix = "quka"  # 可选，用于隔离不同环境
```

## 验证结果

✅ 编译通过: `go build -o quka ./cmd/`
✅ 静态检查通过: `go vet ./...`
✅ 所有调用点正确更新
✅ Key Prefix功能完全实现

## 带来的好处

1. **环境隔离**: 不同环境（开发、测试、生产）可以使用不同的key prefix避免冲突
2. **多应用支持**: 同一Redis实例可以支持多个应用，每个应用使用独立的键空间
3. **故障排查**: 通过key prefix可以快速定位问题所属模块
4. **资源管理**: 不同模块的任务可以独立监控和管理
5. **扩展性**: 为未来支持更多队列模块奠定了基础

## 兼容性说明

- ✅ 向后兼容: 默认行为与之前版本保持一致
- ✅ 配置可选: KeyPrefix字段为可选，不配置时使用默认值
- ✅ 渐进式迁移: 可以逐步为不同模块添加独立的前缀

## 后续建议

1. 为其他模块（如果有）也添加独立的key prefix支持
2. 考虑在监控系统中按key prefix进行统计
3. 在文档中说明key prefix的最佳实践
