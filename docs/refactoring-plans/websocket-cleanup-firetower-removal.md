# WebSocket 改造方案：移除 FireTower 遗留代码

## 问题背景

在之前的 WebSocket 改造中，系统从 FireTower 迁移到了 Centrifuge，但存在以下问题：

1. **遗留的 FireTower 初始化**：在 `app/core/core.go:100` 中仍然调用 `srv.ApplyTower()`
2. **重复的流控制机制**：系统同时维护了两套流信号注册机制
3. **未完全移除的依赖**：FireTower 相关代码仍然存在于代码库中

## 当前实现分析

### 流信号控制机制

**用途**：当用户在聊天过程中点击"停止"按钮时，需要终止正在进行的 AI 推理。

**实现原理**：
1. 在 AI 推理开始前，注册一个 `closeFunc` 到信号管理器中（key 为 sessionID）
2. 当用户点击停止按钮时，通过 API 调用 `StopChatStream`
3. `StopChatStream` 触发 `NewCloseChatStreamSignal`，执行对应的 `closeFunc`
4. `closeFunc` 调用 `cancel()` 取消 context，终止 AI 推理

### 代码调用链路

#### 注册流信号（3个位置）

**[app/logic/v1/chat.go](../../app/logic/v1/chat.go)**:
- 316 行：`RequestAssistantNormal` 中注册
- 345 行：`RequestAssistantNormalWithThinking` 中注册
- 383 行：`RequestAssistantWithRAG` 中注册

```go
removeSignalFunc := core.Srv().Centrifuge().RegisterStreamSignal(userMessage.SessionID, func() {
    slog.Debug("close chat stream", slog.String("session_id", userMessage.SessionID))
    reqCancel()
    receiver.GetDoneFunc(nil)(context.Canceled)
})
defer removeSignalFunc()
```

#### 触发停止信号

**[app/logic/v1/chat.go:412](../../app/logic/v1/chat.go#L412)**:
```go
func (l *ChatLogic) StopStream(sessionID string) error {
    err := l.core.Srv().Centrifuge().NewCloseChatStreamSignal(sessionID)
    if err != nil {
        return errors.New("ChatLogic.StopStream.Srv.Tower.NewCloseChatStreamSignal", i18n.ERROR_INTERNAL, err)
    }
    return nil
}
```

#### HTTP 路由

**[cmd/service/router.go:201](../../cmd/service/router.go#L201)**:
```go
chat.POST("/:session/stop", s.StopChatStream)
```

### FireTower vs Centrifuge 对比

| 组件 | FireTower | Centrifuge |
|------|-----------|------------|
| **消息发布** | Tower.Publish() | Manager.PublishJSON() |
| **流信号注册** | Tower.RegisterStreamSignal() | Manager.RegisterStreamSignal() |
| **触发停止** | Tower.NewCloseChatStreamSignal() | Manager.NewCloseChatStreamSignal() |
| **实现方式** | 使用内部消息队列 + Goroutine 监听 | 直接内存 Map 存储回调函数 |
| **依赖** | github.com/holdno/firetower | github.com/centrifugal/centrifuge |

**关键发现**：
- FireTower 的流控制功能**不是**通过 WebSocket 实现的
- 它使用内部的服务端订阅主题 `TOWER_EVENT_CLOSE_CHAT_STREAM`
- 通过 Goroutine 监听该主题，收到消息后调用注册的 `closeFunc`
- Centrifuge 版本简化了这个机制，直接使用内存 Map 存储回调函数

## 改造目标

1. 完全移除 FireTower 相关代码
2. 保留并优化 Centrifuge 的流控制功能
3. 清理不再使用的接口和类型定义
4. 确保停止推理功能正常工作

## 改造步骤

### 第一步：移除 ApplyTower 初始化

**文件**: `app/core/core.go`

**修改**:
```go
// 移除第 100 行
// srv.ApplyTower(),

// 修改后的 SetupSrvs 调用
core.srv = srv.SetupSrvs(
    aiApplyFunc,
    srv.ApplyCentrifuge(centrifugeSetupFunc),
)
```

### 第二步：清理 Tower 相关代码

#### 2.1 移除 Tower 结构和方法

**文件**: `app/core/srv/publish.go`

**操作**: 删除整个文件（因为它只包含 Tower 相关代码）

**影响的代码**:
- `Tower` 结构体
- `PublishData` 类型
- `SetupSocketSrv()` 函数
- `ApplyTower()` 函数
- `Tower.RegisterServerSideTopic()` 方法
- `EventRegistry` 结构体

#### 2.2 更新 Srv 结构

**文件**: `app/core/srv/srv.go`

**修改**:
```go
type Srv struct {
    rbac       *RBACSrv
    ai         *AI
    // tower      *Tower  // 删除这一行
    centrifuge CentrifugeManager
}

// 删除 Tower() 方法
// func (s *Srv) Tower() *Tower {
//     return s.tower
// }

// 删除 Pusher() 方法（如果没有其他地方使用）
// func (t *Tower) Pusher() *firetower.SelfPusher[PublishData] {
//     return t.pusher
// }
```

#### 2.3 移除 FireTower 包装器

**文件**: `pkg/socket/firetower/firetower.go`

**操作**: 评估是否可以删除整个文件
- 检查是否有其他地方引用 `SelfPusher`
- 如果没有引用，删除整个文件和目录

### 第三步：清理常量定义

**文件**: `pkg/types/common.go`

**修改**:
```go
// 删除或注释掉 FireTower 事件常量
// const TOWER_EVENT_CLOSE_CHAT_STREAM = "tower_event_close_chat_stream"
```

### 第四步：优化错误消息

**文件**: `app/logic/v1/chat.go:414`

**修改**:
```go
func (l *ChatLogic) StopStream(sessionID string) error {
    err := l.core.Srv().Centrifuge().NewCloseChatStreamSignal(sessionID)
    if err != nil {
        // 修改错误前缀，移除 "Tower" 字样
        return errors.New("ChatLogic.StopStream.Centrifuge.NewCloseChatStreamSignal", i18n.ERROR_INTERNAL, err)
    }
    return nil
}
```

### 第五步：清理依赖

**文件**: `go.mod`

**操作**: 运行清理命令
```bash
go mod tidy
```

**预期移除的依赖**:
- `github.com/holdno/firetower`

### 第六步：更新导入

**检查并删除所有文件中的 FireTower 导入**:
```bash
# 搜索所有导入 firetower 的文件
grep -r "github.com/holdno/firetower" .
grep -r "pkg/socket/firetower" .
```

## 验证测试

### 功能测试

1. **正常聊天流程**
   - 创建聊天会话
   - 发送消息
   - 验证 AI 正常响应

2. **停止推理功能**
   - 发送消息触发 AI 推理
   - 在推理过程中调用停止接口
   - 验证推理被正确终止
   - 检查日志中是否有 "close chat stream" 消息

3. **多会话并发**
   - 同时创建多个会话
   - 同时停止多个会话的推理
   - 验证不会相互干扰

### API 测试

**停止推理接口**:
```bash
POST /api/v1/:spaceid/chat/:session/stop
Authorization: Bearer <token>
```

**预期响应**:
```json
{
  "code": 0,
  "message": "success"
}
```

### 集成测试脚本

```bash
#!/bin/bash

# 1. 启动服务
./quka service -c config.toml &
SERVER_PID=$!

# 2. 等待服务启动
sleep 3

# 3. 创建会话并发送消息
SESSION_ID=$(curl -X POST http://localhost:33033/api/v1/space123/chat \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data.id')

# 4. 发送消息（触发 AI 推理）
MESSAGE_ID=$(curl -X POST http://localhost:33033/api/v1/space123/chat/$SESSION_ID/message/id \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data.message_id')

curl -X POST http://localhost:33033/api/v1/space123/chat/$SESSION_ID/message \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"id\":\"$MESSAGE_ID\",\"message\":\"Tell me a long story\"}" &

# 5. 等待 1 秒后停止推理
sleep 1
curl -X POST http://localhost:33033/api/v1/space123/chat/$SESSION_ID/stop \
  -H "Authorization: Bearer $TOKEN"

# 6. 检查日志
grep "close chat stream" logs/quka.log

# 7. 清理
kill $SERVER_PID
```

## 风险评估

### 低风险项
- ✅ 移除 `ApplyTower()` 调用
- ✅ 删除 `publish.go` 文件
- ✅ 更新错误消息

### 中风险项
- ⚠️ 删除 `firetower.go` 包装器（需要确认没有其他引用）
- ⚠️ 清理 `go.mod` 依赖（可能影响构建）

### 需要特别注意
- 🔴 停止推理功能必须完整测试
- 🔴 确保 `streamSignals` Map 的并发安全性（已使用 `cmap.ConcurrentMap`）
- 🔴 验证 Goroutine 泄漏（`defer removeSignalFunc()` 必须被调用）

## 回滚方案

如果出现问题，可以通过以下步骤回滚：

1. **恢复 git 提交**
   ```bash
   git revert <commit-hash>
   ```

2. **临时兼容方案**
   - 保留 Centrifuge 实现
   - 重新添加 `ApplyTower()` 调用
   - 两套系统并行运行（不推荐）

## 实施建议

1. **分阶段实施**
   - 第一阶段：移除 ApplyTower 调用，验证系统正常
   - 第二阶段：删除 Tower 相关代码文件
   - 第三阶段：清理依赖和常量

2. **充分测试**
   - 在开发环境完整测试停止功能
   - 在预发布环境进行压力测试
   - 准备快速回滚方案

3. **监控指标**
   - 监控 "close chat stream" 日志
   - 监控 Goroutine 数量
   - 监控内存使用情况

## 相关文件清单

### 需要修改的文件
- [x] `app/core/core.go` - 移除 ApplyTower 调用
- [x] `app/core/srv/srv.go` - 删除 tower 字段和方法
- [x] `app/logic/v1/chat.go` - 更新错误消息
- [x] `pkg/types/common.go` - 移除 TOWER_EVENT 常量

### 需要删除的文件
- [x] `app/core/srv/publish.go` - Tower 实现
- [x] `pkg/socket/firetower/firetower.go` - FireTower 包装器（待确认）

### 需要保留的文件
- ✅ `pkg/socket/centrifuge/manager.go` - Centrifuge 管理器
- ✅ `app/core/srv/centrifuge.go` - Centrifuge 接口定义

## 技术债务清理

此次改造将清理以下技术债务：

1. ✅ 移除不再使用的 WebSocket 框架
2. ✅ 统一流控制机制
3. ✅ 减少依赖复杂度
4. ✅ 简化代码维护

## 总结

这次改造是对之前 WebSocket 迁移的完善，主要目的是：

1. **完全移除 FireTower 遗留代码**
2. **保留并优化流控制功能**（停止 AI 推理）
3. **简化系统架构**

关键点是理解**流控制机制不是通过 WebSocket 实现的**，而是通过内存中的回调函数 Map 实现的。Centrifuge 版本的实现更加简洁高效。

## 待确认问题

1. ❓ `pkg/socket/firetower/` 目录中是否还有其他文件依赖？
2. ❓ 是否有其他服务或测试代码仍在使用 FireTower？
3. ❓ 是否需要保留 `TOWER_EVENT_CLOSE_CHAT_STREAM` 常量用于向后兼容？

## 参考资料

- Centrifuge 官方文档: https://centrifugal.dev/
- FireTower GitHub: https://github.com/holdno/firetower
- 项目 WebSocket 迁移记录: （如有文档请补充）
