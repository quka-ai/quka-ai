# WebSocket 分布式停止信号改造方案

## 问题背景

当前系统从 FireTower 迁移到 Centrifuge 时，**停止推理功能在分布式环境下存在严重问题**：

### 问题场景

```
[客户端] --WebSocket--> [服务器 A] (AI 推理正在进行，closeFunc 在 A 的内存)
    |
    +--- HTTP POST /stop --> [服务器 B] (负载均衡到 B)
                                |
                                +---> 在 B 的本地 Map 查找 closeFunc
                                |
                                +---> ❌ 找不到！停止失败
```

### 根本原因

**当前实现**（[manager.go:161-168](../../pkg/socket/centrifuge/manager.go#L161-L168)）：
```go
func (m *Manager) NewCloseChatStreamSignal(sessionID string) error {
    closeFunc, exists := m.streamSignals.Get(sessionID)  // ❌ 只查本地内存
    if exists && closeFunc != nil {
        go closeFunc()
    }
    return nil
}
```

这个实现**假设了单机环境**，在分布式部署时：
- 不同服务器实例的内存是隔离的
- HTTP 请求可能路由到任意实例
- WebSocket 连接固定在某个实例上

### FireTower 的原始设计

FireTower 即使配置为 `SingleMode`，也使用了**发布/订阅模式**：

```go
// publish.go:163-174
func (t *Tower) NewCloseChatStreamSignal(sessionID string) error {
    fire := t.NewFire(protocol.SourceSystem, t.pusher)
    fire.Message.Topic = types.TOWER_EVENT_CLOSE_CHAT_STREAM  // 发布到主题
    fire.Message.Data = PublishData{
        Subject: sessionID,
    }
    return t.Publish(fire)  // 发布消息
}

// publish.go:122-144
func (t *Tower) RegisterServerSideTopic() {
    serverSideTower.Subscribe(fire.Context, []string{
        types.TOWER_EVENT_CLOSE_CHAT_STREAM,  // 订阅主题
    })
    serverSideTower.SetReceivedHandler(func(fi fireprotocol.ReadOnlyFire[PublishData]) {
        closeFunc, exist := t.systemEventRegistry.ChatStreamSignal.Get(fi.GetMessage().Data.Subject)
        if exist {
            closeFunc()  // 收到消息后执行
        }
    })
}
```

**关键特性**：
- 即使是单机，也通过内部消息队列传递
- 天然支持扩展到分布式（换用 Redis 作为 Broker）
- 发送方和接收方解耦

## 解决方案设计

### 方案：使用 Centrifuge Server-Side Channel

Centrifuge 支持服务端订阅频道（不需要客户端连接）：

#### 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      Redis (Broker)                         │
│                                                             │
│  Channel: "system:stop_chat_stream"                        │
│    Message: {"session_id": "xxx"}                          │
└─────────────────────────────────────────────────────────────┘
           ↑                                    ↓
    Publish (服务器 B)                  Subscribe (服务器 A)
           │                                    │
┌──────────┴─────────┐              ┌──────────┴─────────┐
│   服务器 B          │              │   服务器 A          │
│                    │              │                    │
│  HTTP /stop 请求   │              │  AI 推理进行中     │
│                    │              │  closeFunc 在内存  │
│  发布停止消息 →    │              │  收到消息 → 执行   │
└────────────────────┘              └────────────────────┘
```

### 实现步骤

#### 1. 定义系统频道常量

**文件**: `pkg/types/common.go`

```go
// 系统内部频道（服务端订阅）
const (
    // 停止聊天流信号频道
    SYSTEM_CHANNEL_STOP_CHAT_STREAM = "system:stop_chat_stream"
)

// 停止信号消息结构
type StopChatStreamMessage struct {
    SessionID string `json:"session_id"`
    Timestamp int64  `json:"timestamp"`
}
```

#### 2. 改造 Manager 初始化

**文件**: `pkg/socket/centrifuge/manager.go`

```go
func NewManager(cfg *Config, authorStore Author) (*Manager, error) {
    // ... 现有代码 ...

    manager := &Manager{
        node:          node,
        config:        cfg,
        streamSignals: cmap.New[func()](),
    }

    // 配置 Broker（分布式模式）
    if cfg.DeploymentMode == "distributed" {
        if err := manager.setupRedisBroker(cfg.RedisURL, cfg.RedisCluster); err != nil {
            return nil, fmt.Errorf("failed to setup Redis broker: %w", err)
        }
    }

    // 设置认证和连接处理器
    // ... 现有代码 ...

    // ✅ 新增：注册服务端订阅
    if err := manager.setupServerSideSubscription(); err != nil {
        return nil, fmt.Errorf("failed to setup server side subscription: %w", err)
    }

    // 启动节点
    if err := node.Run(); err != nil {
        return nil, fmt.Errorf("failed to run Centrifuge node: %w", err)
    }

    return manager, nil
}
```

#### 3. 实现服务端订阅

**文件**: `pkg/socket/centrifuge/manager.go`

```go
// setupServerSideSubscription 设置服务端订阅
func (m *Manager) setupServerSideSubscription() error {
    // 订阅停止聊天流频道
    err := m.node.Subscribe(types.SYSTEM_CHANNEL_STOP_CHAT_STREAM, func(e centrifuge.StreamPosition) error {
        slog.Debug("server side subscription established",
            "channel", types.SYSTEM_CHANNEL_STOP_CHAT_STREAM)
        return nil
    })

    if err != nil {
        return fmt.Errorf("failed to subscribe to stop channel: %w", err)
    }

    // 注册消息处理器
    m.node.On().Message(func(e centrifuge.MessageEvent) {
        // 只处理停止聊天流频道的消息
        if e.Channel != types.SYSTEM_CHANNEL_STOP_CHAT_STREAM {
            return
        }

        // 解析消息
        var msg types.StopChatStreamMessage
        if err := json.Unmarshal(e.Data, &msg); err != nil {
            slog.Error("failed to unmarshal stop message", "error", err)
            return
        }

        slog.Debug("received stop signal",
            "session_id", msg.SessionID,
            "timestamp", msg.Timestamp)

        // 查找并执行 closeFunc
        closeFunc, exists := m.streamSignals.Get(msg.SessionID)
        if exists && closeFunc != nil {
            slog.Info("executing stop signal", "session_id", msg.SessionID)
            go closeFunc()
        } else {
            slog.Debug("session not found on this instance", "session_id", msg.SessionID)
        }
    })

    return nil
}
```

#### 4. 改造停止信号发送方法

**文件**: `pkg/socket/centrifuge/manager.go`

```go
// NewCloseChatStreamSignal 创建关闭聊天流信号（支持分布式）
func (m *Manager) NewCloseChatStreamSignal(sessionID string) error {
    // 构造停止消息
    msg := types.StopChatStreamMessage{
        SessionID: sessionID,
        Timestamp: time.Now().Unix(),
    }

    // 序列化消息
    data, err := json.Marshal(msg)
    if err != nil {
        return fmt.Errorf("failed to marshal stop message: %w", err)
    }

    // 发布到系统频道（通过 Redis Broker 广播到所有实例）
    _, err = m.node.Publish(
        types.SYSTEM_CHANNEL_STOP_CHAT_STREAM,
        data,
        centrifuge.WithHistory(0, time.Time{}), // 不保存历史
    )

    if err != nil {
        return fmt.Errorf("failed to publish stop signal: %w", err)
    }

    slog.Info("stop signal published", "session_id", sessionID)
    return nil
}
```

#### 5. 保持注册方法不变

**文件**: `pkg/socket/centrifuge/manager.go`

```go
// RegisterStreamSignal 注册流信号（保持不变）
func (m *Manager) RegisterStreamSignal(sessionID string, closeFunc func()) func() {
    m.streamSignals.Set(sessionID, closeFunc)
    slog.Debug("stream signal registered", "session_id", sessionID)

    return func() {
        m.streamSignals.Remove(sessionID)
        slog.Debug("stream signal removed", "session_id", sessionID)
    }
}
```

### 工作流程

#### 单机模式
```
1. HTTP /stop 请求到达
2. 调用 NewCloseChatStreamSignal(sessionID)
3. 发布消息到 "system:stop_chat_stream"
4. Centrifuge 内存 Broker 将消息路由到订阅者
5. 本机的 Message 处理器收到消息
6. 查找 streamSignals Map，执行 closeFunc
7. AI 推理被取消
```

#### 分布式模式
```
1. HTTP /stop 请求到达服务器 B
2. 服务器 B 调用 NewCloseChatStreamSignal(sessionID)
3. 发布消息到 "system:stop_chat_stream"
4. Redis Broker 广播到所有服务器
5. 服务器 A 的 Message 处理器收到消息
6. 服务器 A 查找 streamSignals Map，找到 closeFunc
7. 执行 closeFunc，AI 推理被取消
8. 服务器 B、C、D 也收到消息，但找不到 closeFunc（正常）
```

## 配置更新

### config.toml 示例

```toml
[centrifuge]
# 单机模式（开发环境）
deployment_mode = "single"
max_connections = 10000
heartbeat_interval = 25
enable_presence = true
enable_recovery = true
allowed_origins = ["*"]

# 分布式模式（生产环境）
# deployment_mode = "distributed"
# redis_url = "${REDIS_URL}"  # 从环境变量读取
# redis_cluster = false
```

### 环境变量

```bash
# 生产环境
export REDIS_URL="redis://redis-master:6379"
```

## Centrifuge API 说明

### 服务端订阅

```go
// 方法 1: 使用 node.Subscribe（推荐用于系统频道）
err := node.Subscribe(channel, func(e centrifuge.StreamPosition) error {
    // 订阅成功回调
    return nil
})

// 方法 2: 使用 node.On().Message（全局消息处理器）
node.On().Message(func(e centrifuge.MessageEvent) {
    // 处理任何频道的消息
})
```

### 发布消息

```go
// 发布到频道（会通过 Broker 广播）
result, err := node.Publish(
    channel,
    data,
    centrifuge.WithHistory(ttl, size), // 可选：历史记录
)
```

### Broker 配置

```go
// 内存 Broker（单机，默认）
// 自动使用，无需配置

// Redis Broker（分布式）
broker, err := centrifuge.NewRedisBroker(node, centrifuge.RedisBrokerConfig{
    Shards: redisShards,
})
node.SetBroker(broker)
```

## 兼容性考虑

### 向后兼容

- ✅ API 签名不变（`RegisterStreamSignal`、`NewCloseChatStreamSignal`）
- ✅ 业务逻辑层无需修改
- ✅ 单机模式下行为一致
- ✅ 平滑升级到分布式模式

### 降级方案

如果 Redis 不可用，Centrifuge 会：
- 日志报错但不会 panic
- 回退到内存 Broker（仅本机可用）
- 需要监控 Redis 连接状态

## 测试方案

### 单元测试

**文件**: `pkg/socket/centrifuge/manager_test.go`

```go
func TestStopSignalSingleInstance(t *testing.T) {
    // 创建单机模式 Manager
    cfg := DefaultConfig()
    cfg.DeploymentMode = "single"

    manager, err := NewManager(cfg, mockStore)
    require.NoError(t, err)
    defer manager.Shutdown(context.Background())

    // 注册信号
    stopped := make(chan bool, 1)
    sessionID := "test-session-123"

    removeFunc := manager.RegisterStreamSignal(sessionID, func() {
        stopped <- true
    })
    defer removeFunc()

    // 发送停止信号
    err = manager.NewCloseChatStreamSignal(sessionID)
    require.NoError(t, err)

    // 等待信号触发
    select {
    case <-stopped:
        // 成功
    case <-time.After(time.Second * 2):
        t.Fatal("timeout waiting for stop signal")
    }
}
```

### 集成测试（分布式）

```go
func TestStopSignalDistributed(t *testing.T) {
    // 需要 Redis
    if testing.Short() {
        t.Skip("skipping distributed test")
    }

    // 创建两个实例
    cfg1 := DefaultConfig()
    cfg1.DeploymentMode = "distributed"
    cfg1.RedisURL = "redis://localhost:6379"

    manager1, err := NewManager(cfg1, mockStore)
    require.NoError(t, err)
    defer manager1.Shutdown(context.Background())

    cfg2 := DefaultConfig()
    cfg2.DeploymentMode = "distributed"
    cfg2.RedisURL = "redis://localhost:6379"

    manager2, err := NewManager(cfg2, mockStore)
    require.NoError(t, err)
    defer manager2.Shutdown(context.Background())

    // 在实例 1 注册信号
    stopped := make(chan bool, 1)
    sessionID := "test-session-456"

    removeFunc := manager1.RegisterStreamSignal(sessionID, func() {
        stopped <- true
    })
    defer removeFunc()

    // 等待订阅建立
    time.Sleep(time.Millisecond * 100)

    // 从实例 2 发送停止信号
    err = manager2.NewCloseChatStreamSignal(sessionID)
    require.NoError(t, err)

    // 等待实例 1 收到信号
    select {
    case <-stopped:
        // 成功！跨实例通信工作正常
    case <-time.After(time.Second * 3):
        t.Fatal("timeout waiting for distributed stop signal")
    }
}
```

### 手动测试

**场景 1：单机测试**
```bash
# 启动单个实例
./quka service -c config.toml

# 发送消息触发 AI 推理
curl -X POST http://localhost:33033/api/v1/space1/chat/session1/message \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"id":"msg1","message":"Tell me a long story"}'

# 立即停止
curl -X POST http://localhost:33033/api/v1/space1/chat/session1/stop \
  -H "Authorization: Bearer $TOKEN"

# 检查日志
# 应该看到: "stop signal published" 和 "executing stop signal"
```

**场景 2：分布式测试**
```bash
# 启动 Redis
docker run -d -p 6379:6379 redis:alpine

# 修改配置为分布式模式
# deployment_mode = "distributed"
# redis_url = "redis://localhost:6379"

# 启动实例 1（端口 33033）
./quka service -c config.toml

# 启动实例 2（端口 33034）
./quka service -c config2.toml

# WebSocket 连接到实例 1
wscat -c ws://localhost:33033/api/v1/connect?token=$TOKEN

# 发送消息到实例 1（通过 WebSocket 或 HTTP）
curl -X POST http://localhost:33033/api/v1/space1/chat/session1/message \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"id":"msg1","message":"Tell me a long story"}'

# 发送停止请求到实例 2
curl -X POST http://localhost:33034/api/v1/space1/chat/session1/stop \
  -H "Authorization: Bearer $TOKEN"

# 检查实例 1 的日志
# 应该看到: "received stop signal" 和 "executing stop signal"

# 检查实例 2 的日志
# 应该看到: "stop signal published"
```

## 性能考虑

### 延迟

- **单机模式**: < 1ms（内存队列）
- **分布式模式**: 5-20ms（Redis RTT + 网络）

### Redis 负载

每次停止操作：
- 1 次 PUBLISH 命令
- N 次订阅者接收（N = 实例数）

**示例**：100 实例集群，每秒 1000 次停止请求
- Redis 吞吐：1000 PUBLISH/s
- Redis 负载：可忽略不计（Redis 可轻松处理 10w+ ops/s）

### 内存占用

- `streamSignals` Map：每个活跃会话 ~200 bytes
- 10,000 并发会话：~2 MB
- 可忽略不计

## 监控指标

### 日志监控

```go
// 发布成功
slog.Info("stop signal published", "session_id", sessionID)

// 接收成功
slog.Info("executing stop signal", "session_id", sessionID)

// 找不到会话（正常，说明在其他实例）
slog.Debug("session not found on this instance", "session_id", sessionID)

// 错误情况
slog.Error("failed to publish stop signal", "error", err)
slog.Error("failed to unmarshal stop message", "error", err)
```

### Prometheus 指标（可选）

```go
var (
    stopSignalPublished = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "centrifuge_stop_signal_published_total",
        Help: "Total number of stop signals published",
    })

    stopSignalExecuted = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "centrifuge_stop_signal_executed_total",
        Help: "Total number of stop signals executed",
    })

    stopSignalNotFound = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "centrifuge_stop_signal_not_found_total",
        Help: "Total number of stop signals for sessions not found",
    })
)
```

## 与 FireTower 对比

| 特性 | FireTower | Centrifuge（改造后） |
|------|-----------|---------------------|
| **单机模式** | ✅ 内部消息队列 | ✅ 内存 Broker |
| **分布式模式** | ⚠️ 需要配置 Redis | ✅ Redis Broker |
| **消息传递** | 自定义协议 | Centrifuge 协议 |
| **订阅机制** | `BuildServerSideTower` + `Subscribe` | `node.Subscribe` + `On().Message` |
| **发布机制** | `t.Publish(fire)` | `node.Publish(channel, data)` |
| **性能** | 未知 | 经过验证的生产级性能 |
| **维护性** | 小众库 | CNCF 项目，活跃维护 |

## 迁移检查清单

- [ ] 更新 `pkg/types/common.go` 添加常量
- [ ] 修改 `pkg/socket/centrifuge/manager.go`
  - [ ] 添加 `setupServerSideSubscription()` 方法
  - [ ] 改造 `NewCloseChatStreamSignal()` 方法
  - [ ] 在 `NewManager()` 中调用订阅设置
- [ ] 编写单元测试
- [ ] 编写集成测试（分布式）
- [ ] 更新配置文档
- [ ] 生产环境测试
- [ ] 监控指标验证
- [ ] 性能压测

## 总结

这个改造方案：

1. ✅ **解决分布式问题**：通过 Centrifuge Broker 实现跨实例通信
2. ✅ **向后兼容**：API 不变，现有代码无需修改
3. ✅ **性能优良**：延迟 < 20ms，Redis 负载可忽略
4. ✅ **生产就绪**：Centrifuge 是经过验证的 CNCF 项目
5. ✅ **易于测试**：单机和分布式都有完整测试方案

**关键洞察**：
- FireTower 即使在单机模式下也用了 Pub/Sub，这是为了架构一致性
- Centrifuge 原生支持服务端订阅，无需客户端连接
- 通过 Redis Broker，消息自动广播到所有实例
- 即使消息到达所有实例，只有持有 closeFunc 的实例会执行

这个设计完全对齐了 FireTower 的原始架构意图！
