# Redis 配置重构

## 背景
原有的 Redis 配置耦合在 Centrifuge 配置中，不够灵活。本次重构将 Redis 配置独立出来，支持独立的用户名/密码配置，并同时支持单机模式和集群模式。

## 改造目标
1. 将 Redis 配置从 Centrifuge 配置中解耦
2. 支持独立的用户名和密码配置
3. 同时支持 Redis 单机模式和集群模式
4. 使用 `redis.UniversalClient` 接口统一处理两种模式
5. 验证 Redis 集群模式支持队列操作（BLPOP/RPUSH）

## 实施方案

### 1. 配置结构定义

在 `app/core/config.go` 中新增独立的 `RedisConfig` 结构：

```go
type RedisConfig struct {
    // 单机模式配置
    Addr     string `toml:"addr"`     // Redis地址，格式: host:port
    Password string `toml:"password"` // Redis密码
    DB       int    `toml:"db"`       // Redis数据库索引 (0-15)

    // 集群模式配置
    Cluster       bool     `toml:"cluster"`        // 是否启用集群模式
    ClusterAddrs  []string `toml:"cluster_addrs"`  // 集群节点地址列表
    ClusterPasswd string   `toml:"cluster_passwd"` // 集群密码

    // 连接池配置
    PoolSize     int `toml:"pool_size"`     // 连接池大小，默认10
    MinIdleConns int `toml:"min_idle_conns"` // 最小空闲连接数，默认0
    MaxRetries   int `toml:"max_retries"`   // 最大重试次数，默认3
    DialTimeout  int `toml:"dial_timeout"`  // 连接超时(秒)，默认5
    ReadTimeout  int `toml:"read_timeout"`  // 读超时(秒)，默认3
    WriteTimeout int `toml:"write_timeout"` // 写超时(秒)，默认3
}
```

辅助方法：
- `FromENV()`: 从环境变量加载配置
- `GetAddr()`: 获取 Redis 地址（兼容旧配置）
- `IsCluster()`: 判断是否为集群模式

### 2. Core 层改造

在 `app/core/core.go` 中：

**字段类型变更**：
```go
type Core struct {
    // ...
    redisClient redis.UniversalClient // 从 *redis.Client 改为 UniversalClient
    // ...
}
```

**Redis() 方法签名变更**：
```go
func (s *Core) Redis() redis.UniversalClient {
    return s.redisClient
}
```

**setupRedis() 函数实现**：
```go
func setupRedis(core *Core) {
    cfg := core.cfg.Redis

    // 设置默认值
    if cfg.PoolSize <= 0 {
        cfg.PoolSize = 10
    }
    if cfg.MaxRetries <= 0 {
        cfg.MaxRetries = 3
    }
    // ... 其他默认值设置

    // 判断是否为集群模式
    if cfg.IsCluster() {
        // Redis 集群模式
        core.redisClient = redis.NewClusterClient(&redis.ClusterOptions{
            Addrs:        cfg.ClusterAddrs,
            Password:     cfg.ClusterPasswd,
            PoolSize:     cfg.PoolSize,
            MinIdleConns: cfg.MinIdleConns,
            MaxRetries:   cfg.MaxRetries,
            DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
            ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
            WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
        })
    } else {
        // 单机模式
        core.redisClient = redis.NewClient(&redis.Options{
            Addr:         cfg.GetAddr(),
            Password:     cfg.Password,
            DB:           cfg.DB,
            PoolSize:     cfg.PoolSize,
            MinIdleConns: cfg.MinIdleConns,
            MaxRetries:   cfg.MaxRetries,
            DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
            ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
            WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
        })
    }

    // 测试连接
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := core.redisClient.Ping(ctx).Err(); err != nil {
        panic(fmt.Sprintf("Failed to connect to Redis: %v", err))
    }

    slog.Info("Redis connected successfully",
        slog.Bool("cluster_mode", cfg.IsCluster()))
}
```

### 3. 队列层改造

在 `pkg/queue/rss_queue.go` 中：

**RSSQueue 结构体字段类型变更**：
```go
type RSSQueue struct {
    redis    redis.UniversalClient // 从 *redis.Client 改为 UniversalClient
    workerID string
}
```

**构造函数签名变更**：
```go
func NewRSSQueue(redisClient redis.UniversalClient, workerID string) *RSSQueue {
    return &RSSQueue{
        redis:    redisClient,
        workerID: workerID,
    }
}
```

### 4. 配置文件示例

**单机模式配置** (`config.toml`):
```toml
[redis]
addr = "localhost:6379"
password = "your_password"
db = 0
pool_size = 10
min_idle_conns = 2
max_retries = 3
dial_timeout = 5
read_timeout = 3
write_timeout = 3
```

**集群模式配置** (`config.toml`):
```toml
[redis]
cluster = true
cluster_addrs = [
    "redis-node1:6379",
    "redis-node2:6379",
    "redis-node3:6379"
]
cluster_passwd = "your_cluster_password"
pool_size = 20
min_idle_conns = 5
max_retries = 3
dial_timeout = 5
read_timeout = 3
write_timeout = 3
```

**环境变量配置**:
```bash
QUKA_REDIS_ADDR=localhost:6379
QUKA_REDIS_PASSWORD=your_password
QUKA_REDIS_DB=0
```

## 技术要点

### 1. UniversalClient 接口的使用
`redis.UniversalClient` 是 go-redis 库提供的统一接口，可以同时兼容：
- `redis.Client` (单机模式)
- `redis.ClusterClient` (集群模式)

这样做的好处：
- 业务代码无需关心底层是单机还是集群
- 可以在运行时动态切换模式（通过配置）
- 所有队列操作（BLPOP, RPUSH, ZAdd 等）在两种模式下都可以正常工作

### 2. Redis 集群对队列的支持
Redis 集群模式**完全支持**队列操作，包括：
- `BLPOP` (阻塞式左弹出)
- `RPUSH` (右推入)
- `LPUSH`, `RPOP` 等其他列表操作
- `ZADD`, `ZRANGEBYSCORE` 等有序集合操作

集群模式下的注意事项：
- 所有操作同一个 key 的命令会被路由到同一个节点
- 队列的 key（如 `rss:queue`）会根据哈希槽分配到特定节点
- 不影响队列的 FIFO 特性和原子性

### 3. 代码兼容性
所有使用 `core.Redis()` 的代码无需修改：
- 返回类型从 `*redis.Client` 改为 `redis.UniversalClient`
- `redis.UniversalClient` 接口包含所有常用 Redis 命令
- 现有的队列操作代码保持不变

## 修改文件清单

1. **app/core/config.go**
   - 新增 `RedisConfig` 结构体
   - 新增 `FromENV()`, `GetAddr()`, `IsCluster()` 方法

2. **app/core/core.go**
   - 修改 `Core.redisClient` 字段类型：`*redis.Client` → `redis.UniversalClient`
   - 修改 `Redis()` 方法返回类型
   - 实现 `setupRedis()` 函数，支持单机和集群模式

3. **pkg/queue/rss_queue.go**
   - 修改 `RSSQueue.redis` 字段类型
   - 修改 `NewRSSQueue()` 参数类型

4. **app/logic/v1/rss_fetcher.go**
   - 清理已废弃方法中的不可达代码
   - 保留废弃标记和警告日志

## 测试验证

### 1. 编译测试
```bash
go build -o /tmp/quka-test ./cmd/
go vet ./...
```

### 2. 单机模式测试
- 配置单机 Redis
- 启动服务，验证连接成功
- 触发 RSS 订阅抓取，验证队列工作正常

### 3. 集群模式测试
- 配置 Redis 集群
- 启动服务，验证集群连接成功
- 触发 RSS 订阅抓取，验证队列在集群模式下工作正常
- 监控队列统计 API，验证任务正常入队和出队

## 状态
✅ 已完成

## 相关文档
- [RSS 队列重构计划](./rss-queue-refactoring.md)
- [RSS 功能实现状态](./rss-implementation-status.md)
