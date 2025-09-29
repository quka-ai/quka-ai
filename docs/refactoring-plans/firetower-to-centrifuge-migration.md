# Firetower 到 Centrifuge 迁移改造计划

## 项目背景

当前项目使用firetower作为WebSocket连接维护器，存在以下问题：
- 性能瓶颈：ReadChanLens设置过小(5)，WriteChanLens(1000)在高并发下不足
- 缺乏分布式支持：无法水平扩展
- 功能限制：缺乏完善的在线状态管理、消息历史等功能
- 维护问题：社区活跃度较低，文档不完善

## 改造目标

1. **性能提升**：支持更高并发连接和消息吞吐量
2. **分布式支持**：支持水平扩展和高可用
3. **功能增强**：完善的在线状态、消息历史、频道管理
4. **降低维护成本**：使用成熟的开源方案

## 第一阶段：技术调研和准备 (1-2天)

### 1.1 依赖分析
- [ ] 分析当前firetower在项目中的使用范围
- [ ] 识别所有相关的代码文件和配置
- [ ] 评估数据迁移需求

### 1.2 技术选型确认
- [ ] Centrifuge版本选择：`github.com/centrifugal/centrifuge v0.29.x`
- [ ] Redis版本要求：Redis 6.0+
- [ ] 前端SDK选择：`centrifuge-js v4.x`

### 1.3 环境准备
- [ ] 准备测试环境Redis实例
- [ ] 搭建Centrifuge测试服务
- [ ] 验证基本功能可用性

## 第二阶段：后端核心改造 (3-5天)

### 2.1 依赖更新
```bash
# 添加centrifuge依赖
go get github.com/centrifugal/centrifuge@v0.29.x

# 移除firetower依赖
# 从go.mod中移除 github.com/holdno/firetower 相关依赖
```

### 2.2 核心模块改造

#### 2.2.1 替换 `pkg/socket/firetower/firetower.go`
**新文件**: `pkg/socket/centrifuge/centrifuge.go`

```go
package centrifuge

import (
    "context"
    "log/slog"
    "github.com/centrifugal/centrifuge"
)

type CentrifugeManager struct {
    node   *centrifuge.Node
    config *Config
}

type Config struct {
    // 基础配置
    TokenSecret      string
    AllowedOrigins   []string
    MaxConnections   int
    HeartbeatInterval int
    
    // 部署模式
    DeploymentMode   string // "single" | "distributed"
    
    // 分布式配置 (可选)
    RedisURL         string
    RedisCluster     bool
    
    // 功能开关
    EnablePresence   bool
    EnableHistory    bool  // 一般设为false，由业务层管理
    EnableRecovery   bool
}

func SetupCentrifuge(cfg *Config) (*CentrifugeManager, error) {
    // 根据deployment_mode选择引擎
    var engine centrifuge.Engine
    if cfg.DeploymentMode == "distributed" {
        // 使用Redis引擎
        redisEngine, err := centrifuge.NewRedisEngine(centrifuge.RedisEngineConfig{
            RedisAddress: cfg.RedisURL,
        })
        if err != nil {
            return nil, err
        }
        engine = redisEngine
    } else {
        // 使用内存引擎（单实例）
        engine = centrifuge.NewMemoryEngine()
    }

    node, err := centrifuge.New(centrifuge.Config{
        Engine: engine,
        // 其他配置...
    })
    
    return &CentrifugeManager{
        node:   node,
        config: cfg,
    }, err
}
```

#### 2.2.2 修改 `app/core/srv/publish.go`
**新接口设计**:
```go
type CentrifugeTower struct {
    manager *centrifuge.CentrifugeManager
}

func (c *CentrifugeTower) PublishMessage(channel string, data interface{}) error
func (c *CentrifugeTower) GetChannelStats(channel string) (*ChannelStats, error)
func (c *CentrifugeTower) RegisterConnectionHandler(handler ConnectionHandler)
```

#### 2.2.3 更新 `cmd/service/handler/websocket.go`
- [ ] 移除firetower相关代码
- [ ] 实现Centrifuge连接处理
- [ ] 添加认证和授权逻辑
- [ ] 实现频道订阅权限验证

### 2.3 消息结构标准化

#### 2.3.1 统一消息格式
```go
type StandardMessage struct {
    Type      string      `json:"type"`
    Channel   string      `json:"channel"`
    Data      interface{} `json:"data"`
    Timestamp int64       `json:"timestamp"`
    MessageID string      `json:"message_id,omitempty"`
}
```

#### 2.3.2 频道命名规范
```go
const (
    ChannelUser    = "user:%s"           // 用户私有频道
    ChannelSession = "session:%s"        // 会话频道
    ChannelSpace   = "space:%s"          // 空间频道
    ChannelSystem  = "system"            // 系统广播频道
)
```

### 2.4 配置文件更新

#### 2.4.1 更新 `cmd/service/etc/service-default.toml`
```toml
[websocket]
enable = true
allow_origins = ["*"]

[centrifuge]
# 部署模式: "single" | "distributed"
deployment_mode = "single"

# 分布式配置 (仅当 deployment_mode = "distributed" 时生效)
# redis_url = "redis://localhost:6379"
# redis_cluster = false

# 功能开关
enable_presence = true      # 在线状态统计
enable_history = false      # 消息历史 (由业务层MySQL管理)
enable_recovery = true      # 断线重连消息恢复

# 性能配置
max_connections = 10000     # 最大连接数
heartbeat_interval = 25     # 心跳间隔(秒)
max_channel_length = 255    # 最大频道名长度
max_message_size = 65536    # 最大消息大小
```

#### 2.4.2 认证方案

**复用现有JWT认证** (推荐):
- ✅ **无需额外配置**: 移除token_secret配置
- ✅ **复用现有逻辑**: 直接使用项目现有的JWT验证
- ✅ **简化前端**: 无需额外API获取Centrifuge Token
- ✅ **统一权限**: 使用现有的用户和空间权限逻辑

**认证流程**:
```
用户登录 → 现有JWT Token → 直接WebSocket连接
(无需额外的Centrifuge Token获取步骤)
```

#### 2.4.3 配置说明

**单实例模式** (推荐用于中小型部署):
```toml
[centrifuge]
deployment_mode = "single"
enable_history = false      # 历史消息由MySQL管理
enable_presence = true      # 保留在线统计功能
```

**分布式模式** (用于大规模部署):
```toml
[centrifuge]
deployment_mode = "distributed"
redis_url = "redis://localhost:6379"
enable_history = false      # 仍由MySQL管理
enable_presence = true      # 跨节点在线统计
```

**配置优势**:
- ✅ **零依赖启动**: 单实例模式无需Redis
- ✅ **渐进式扩展**: 可随时切换到分布式
- ✅ **简化架构**: 历史消息复用现有MySQL
- ✅ **复用认证**: 直接使用现有JWT验证逻辑
- ✅ **降低复杂度**: 最小化配置项

> 💡 **详细配置示例**: 参见 `docs/refactoring-plans/centrifuge-config-examples.md`

## 第三阶段：业务逻辑适配 (2-3天)

### 3.1 聊天功能适配

#### 3.1.1 修改 `app/logic/v1/chat.go`
```go
// 替换消息发送逻辑
func (l *ChatLogic) publishChatMessage(msg *types.ChatMessage) error {
    channel := fmt.Sprintf("session:%s", msg.SessionID)
    return l.core.Centrifuge().PublishMessage(channel, chatMsgToStandardMsg(msg))
}

// 添加在线用户统计
func (l *ChatLogic) GetSessionOnlineUsers(sessionID string) (int, error) {
    stats, err := l.core.Centrifuge().GetChannelStats(fmt.Sprintf("session:%s", sessionID))
    if err != nil {
        return 0, err
    }
    return stats.NumUsers, nil
}
```

#### 3.1.2 修改 `app/logic/v1/ai.go`
```go
// 更新stream消息发送
func getStreamReceiveFunc(ctx context.Context, core *core.Core, sendedCounter SendedCounter, msg *types.ChatMessage) types.ReceiveFunc {
    return func(message types.MessageContent, progressStatus types.MessageProgress) error {
        channel := fmt.Sprintf("session:%s", msg.SessionID)
        
        streamMsg := &StandardMessage{
            Type:      "ai_stream",
            Channel:   channel,
            Data: &types.StreamMessage{
                MessageID: msg.ID,
                SessionID: msg.SessionID,
                Message:   string(message.Bytes()),
                StartAt:   sendedCounter.Get(),
                MsgType:   msg.MsgType,
                Complete:  int32(progressStatus),
            },
            Timestamp: time.Now().Unix(),
        }
        
        return core.Centrifuge().PublishMessage(channel, streamMsg)
    }
}
```

### 3.2 权限和安全

#### 3.2.1 复用现有JWT认证
```go
// pkg/socket/centrifuge/auth.go
type AuthHandler struct {
    core *core.Core
}

// 连接认证 - 复用现有JWT验证
func (a *AuthHandler) OnConnecting(ctx context.Context, event centrifuge.ConnectEvent) (centrifuge.ConnectReply, error) {
    // 从URL参数或Header中获取现有JWT token
    token := a.extractToken(event.Transport)
    if token == "" {
        return centrifuge.ConnectReply{}, centrifuge.ErrorUnauthorized
    }
    
    // 使用现有的JWT验证逻辑
    claims, err := v1.ValidateJWTToken(token)
    if err != nil {
        return centrifuge.ConnectReply{}, centrifuge.ErrorUnauthorized
    }
    
    return centrifuge.ConnectReply{
        Credentials: &centrifuge.Credentials{
            UserID: claims.User,
        },
    }, nil
}
```

#### 3.2.2 频道订阅权限验证
```go
// 复用现有权限验证逻辑
func (a *AuthHandler) OnSubscribe(ctx context.Context, client *centrifuge.Client, event centrifuge.SubscribeEvent) (centrifuge.SubscribeReply, error) {
    userID := client.UserID()
    channel := event.Channel
    
    switch {
    case strings.HasPrefix(channel, "user:"):
        // 用户只能访问自己的频道
        return a.validateUserChannel(userID, channel)
        
    case strings.HasPrefix(channel, "session:"):
        // 使用现有的会话权限验证
        sessionID := strings.TrimPrefix(channel, "session:")
        return a.validateSessionAccess(userID, sessionID)
        
    case strings.HasPrefix(channel, "space:"):
        // 使用现有的空间权限验证
        spaceID := strings.TrimPrefix(channel, "space:")
        return a.validateSpaceAccess(userID, spaceID)
    }
    
    return centrifuge.SubscribeReply{}, centrifuge.ErrorPermissionDenied
}
```

## 第四阶段：前端对接文档编写 (1天)

### 4.1 前端SDK迁移指南
**文档路径**: `docs/frontend/centrifuge-migration-guide.md`

### 4.2 API变更说明
**文档路径**: `docs/frontend/websocket-api-changes.md`

### 4.3 示例代码
**文档路径**: `docs/frontend/centrifuge-examples.md`

## 第五阶段：测试和部署 (2-3天)

### 5.1 单元测试
- [ ] 添加Centrifuge集成测试
- [ ] 消息发送接收测试
- [ ] 权限验证测试
- [ ] 性能基准测试

### 5.2 集成测试
- [ ] 前后端联调测试
- [ ] 多用户并发测试
- [ ] 断线重连测试
- [ ] 消息历史测试

### 5.3 性能测试
- [ ] 连接数测试：目标10万连接
- [ ] 消息吞吐量测试：目标1万消息/秒
- [ ] 内存和CPU使用率测试
- [ ] Redis性能影响测试

### 5.4 回滚准备
- [ ] 保留原firetower代码分支
- [ ] 准备快速回滚脚本
- [ ] 数据备份策略

## 第六阶段：生产环境部署 (1天)

### 6.1 预发布环境验证
- [ ] 完整功能验证
- [ ] 性能压力测试
- [ ] 监控指标验证

### 6.2 生产环境部署
- [ ] 灰度发布策略
- [ ] 监控和告警配置
- [ ] 用户通知和说明

## 风险评估和缓解措施

### 高风险项
1. **数据一致性**：消息可能重复或丢失
   - 缓解：添加消息去重机制，幂等性处理
   
2. **性能回归**：新系统可能性能不如预期
   - 缓解：充分的性能测试，准备回滚方案

3. **前端兼容性**：客户端可能出现连接问题
   - 缓解：详细的前端对接文档，充分测试

### 中风险项
1. **Redis依赖**：增加了Redis的依赖
   - 缓解：Redis集群部署，监控告警

2. **学习成本**：团队需要学习新的API
   - 缓解：详细文档，内部培训

## 时间规划

| 阶段 | 工作内容 | 预计时间 | 负责人 |
|------|----------|----------|--------|
| 第一阶段 | 技术调研和准备 | 1-2天 | 后端开发 |
| 第二阶段 | 后端核心改造 | 3-5天 | 后端开发 |
| 第三阶段 | 业务逻辑适配 | 2-3天 | 后端开发 |
| 第四阶段 | 前端对接文档 | 1天 | 后端+前端 |
| 第五阶段 | 测试和部署 | 2-3天 | 全团队 |
| 第六阶段 | 生产环境部署 | 1天 | 运维+开发 |

**总计：10-15个工作日**

## 成功标准

1. **功能完整性**：所有现有WebSocket功能正常工作
2. **性能提升**：连接数和消息吞吐量有明显提升  
3. **稳定性**：连续运行72小时无异常
4. **用户体验**：前端用户无感知切换
5. **可维护性**：代码结构清晰，文档完善

## 后续优化计划

1. **监控完善**：添加详细的业务监控指标
2. **功能增强**：利用Centrifuge的高级特性
3. **性能优化**：根据生产环境数据进一步优化
4. **文档完善**：补充操作手册和故障排查指南

---

**注意**：此改造计划需要根据实际项目情况进行调整，建议在开始前与团队充分讨论和评估。