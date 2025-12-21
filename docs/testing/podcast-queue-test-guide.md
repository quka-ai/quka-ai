# Podcast Queue 单元测试指南

## 概述

本指南详细说明了如何使用 `pkg/queue/podcast_queue_test.go` 中的单元测试来验证 podcast queue 的功能。测试覆盖了队列创建、任务入队、延迟队列、处理器设置、优雅关闭等核心功能。

## 架构设计

### 队列名称规范

Podcast queue 使用专门的队列名称 `"podcast"`，通过常量 `PodcastQueueName` 定义：

```go
const (
    TaskTypePodcastGeneration = "podcast:generation"
    PodcastQueueName          = "podcast"
    PodcastMaxRetries         = 3
    PodcastTaskTimeout        = 30 * time.Minute
)
```

**设计优势：**
- **任务隔离**：Podcast 任务不会与其他队列（RSS 等）的任务混杂
- **独立监控**：可以单独监控 podcast 队列的状态和性能
- **灵活配置**：可以为不同队列设置不同的优先级和并发度
- **易于维护**：队列名称集中管理，避免硬编码

## 环境配置

### Redis 环境变量

测试使用以下环境变量（使用 `QUKA_TEST_` 前缀）：

```bash
# Redis 地址（默认: localhost:6379）
export QUKA_TEST_REDIS_ADDR="localhost:6379"

# Redis 密码（默认: 空）
export QUKA_TEST_REDIS_PASSWORD="your_redis_password"

# Redis 数据库编号（默认: 1，避免与生产数据冲突）
export QUKA_TEST_REDIS_DB="1"
```

### .env 文件配置

在项目根目录创建 `.env` 文件：

```bash
# .env 文件内容
QUKA_TEST_REDIS_ADDR=localhost:6379
QUKA_TEST_REDIS_PASSWORD=
QUKA_TEST_REDIS_DB=1
```

## 运行测试

### 1. 运行所有测试

```bash
# 运行所有 podcast queue 测试
go test ./pkg/queue/ -v -run TestPodcast

# 运行测试并显示详细输出
go test ./pkg/queue/ -v -run TestPodcast 2>&1 | tee test_output.log
```

### 2. 运行特定测试

```bash
# 测试队列创建
go test ./pkg/queue/ -v -run TestPodcastQueue_NewPodcastQueueWithClientServer

# 测试任务入队
go test ./pkg/queue/ -v -run TestPodcastQueue_EnqueueGenerationTask

# 测试延迟任务入队
go test ./pkg/queue/ -v -run TestPodcastQueue_EnqueueDelayedGenerationTask

# 测试处理器设置
go test ./pkg/queue/ -v -run TestPodcastQueue_SetupHandler

# 测试优雅关闭
go test ./pkg/queue/ -v -run TestPodcastQueue_Shutdown

# 测试 JSON 序列化
go test ./pkg/queue/ -v -run TestPodcastGenerationTask_JSONMarshaling

# 测试任务出队（消费）
go test ./pkg/queue/ -v -run TestPodcastQueue_DequeueTask

# 测试延迟任务出队
go test ./pkg/queue/ -v -run TestPodcastQueue_DequeueDelayedTask

# 运行集成测试
go test ./pkg/queue/ -v -run TestPodcastQueue_Integration

# 运行性能基准测试
go test ./pkg/queue/ -bench Benchmark_EnqueueGenerationTask
```

### 3. 运行短测试（跳过集成测试）

```bash
# 跳过集成测试（使用 -short 标志）
go test ./pkg/queue/ -v -short

# 或使用测试选择器
go test ./pkg/queue/ -v -run "TestPodcastQueue.*/(?!Integration)"
```

## 测试覆盖范围

### 单元测试

1. **TestPodcastQueue_NewPodcastQueueWithClientServer**
   - 测试队列创建功能
   - 验证 keyPrefix、client、server 的正确设置
   - 测试空 keyPrefix 的默认值处理

2. **TestPodcastQueue_NewPodcastQueueWithClientServer_EmptyKeyPrefix**
   - 专门测试空 keyPrefix 的默认值处理

3. **TestPodcastQueue_EnqueueGenerationTask**
   - 测试正常任务入队
   - 验证队列状态
   - 检查任务是否正确提交

4. **TestPodcastQueue_EnqueueGenerationTask_MarshalError**
   - 测试 JSON 序列化错误处理
   - 验证大数据量场景下的稳定性

5. **TestPodcastQueue_EnqueueDelayedGenerationTask**
   - 测试延迟任务入队
   - 验证延迟参数的正确应用

6. **TestPodcastQueue_SetupHandler**
   - 测试任务处理器注册
   - 验证任务类型和载荷解析

7. **TestPodcastQueue_Shutdown**
   - 测试优雅关闭功能
   - 验证资源清理（client、server）

8. **TestPodcastGenerationTask_JSONMarshaling**
   - 测试任务结构的 JSON 序列化/反序列化
   - 验证数据一致性

9. **TestPodcastQueue_DequeueTask**
   - 测试任务出队（消费）流程
   - 验证 worker 正确处理任务
   - 验证队列状态变化
   - 验证任务幂等性

10. **TestPodcastQueue_DequeueDelayedTask**
    - 测试延迟任务出队
    - 验证延迟时间正确
    - 验证任务按时执行

### 集成测试

11. **TestPodcastQueue_Integration**
   - 多个任务并发入队测试
   - 队列状态监控
   - 端到端功能验证
   - 使用 `-short` 标志跳过

### 性能测试

12. **Benchmark_EnqueueGenerationTask**
    - 性能基准测试
    - 评估入队操作的吞吐量
    - 用于性能回归检测

## 测试场景说明

### 1. 单例模式 Redis 客户端

测试使用单例模式的 Redis 客户端：
- 所有测试共享同一个 Redis 连接
- 避免连接泄漏
- 提高测试执行效率

### 2. 测试数据库隔离

- 使用 Redis DB 1（通过 `QUKA_TEST_REDIS_DB` 配置）
- 避免与生产数据冲突
- 测试完成后自动清理（`FlushDB`）

### 3. 队列隔离

- Podcast 任务使用独立的 "podcast" 队列
- 与其他队列（如 RSS）的任务完全隔离
- 可以独立监控和管理各队列的状态
- 避免任务冲突和资源竞争

### 4. 出队测试

- 启动真实的 asynq worker 进行任务消费
- 验证任务从 pending 状态到 completed 状态的转换
- 验证延迟任务按预期时间执行
- 验证队列状态在任务处理前后的变化
- 使用通道进行测试同步，确保测试稳定性

### 5. 错误处理测试

- JSON 序列化错误
- 网络连接错误
- 资源清理验证

### 6. 并发安全

- 测试多任务并发入队
- 验证队列状态一致性

## 故障排除

### 常见问题

1. **Redis 连接失败**
   ```
   Error: dial tcp: connection refused
   ```
   解决方案：
   - 确保 Redis 服务正在运行
   - 检查 `QUKA_TEST_REDIS_ADDR` 环境变量
   - 验证防火墙设置

2. **环境变量未设置**
   ```
   Error: Getenv returned empty string
   ```
   解决方案：
   - 确保已设置必要的环境变量
   - 检查 `.env` 文件是否存在
   - 验证 `testutils.LoadEnvOrPanic()` 加载成功

3. **测试超时**
   ```
   Error: test timed out
   ```
   解决方案：
   - 增加测试超时时间
   - 使用 `-short` 跳过集成测试
   - 检查 Redis 性能

4. **队列名称不匹配**
   ```
   Error: Failed to get queue info: queue not found
   ```
   解决方案：
   - 确保 `PodcastQueueName` 常量正确
   - 检查测试中的队列名称配置
   - 验证 asynq 服务器的队列配置

### 调试技巧

1. **启用详细日志**
   ```bash
   go test ./pkg/queue/ -v -run TestPodcast 2>&1 | tee debug.log
   ```

2. **查看 Redis 中的测试数据**
   ```bash
   redis-cli -h localhost -p 6379 -n 1 KEYS "*"
   ```

3. **监控 Redis 连接**
   ```bash
   redis-cli -h localhost -p 6379 INFO clients
   ```

## 性能优化

### 基准测试结果

运行性能基准测试：
```bash
go test ./pkg/queue/ -bench Benchmark_EnqueueGenerationTask -benchmem
```

预期结果：
- 每秒可处理 1000+ 次入队操作
- 内存分配合理
- 无内存泄漏

### 性能调优建议

1. **Redis 连接池配置**
   - 调整 `MaxActive` 和 `MaxIdle`
   - 设置合理的连接超时

2. **队列并发度配置**
   - 根据 CPU 核数调整 `Concurrency`
   - 监控任务处理延迟

3. **任务重试策略**
   - 合理设置 `PodcastMaxRetries`
   - 优化重试间隔

## 最佳实践

### 测试环境准备

1. **独立测试环境**
   - 使用独立的 Redis 实例
   - 定期清理测试数据
   - 监控测试资源使用

2. **环境变量管理**
   - 使用 `.env` 文件管理配置
   - 不同环境使用不同前缀
   - 定期更新默认配置

3. **测试数据管理**
   - 使用固定的测试数据
   - 避免硬编码的值
   - 确保测试幂等性

### 测试编写规范

1. **命名规范**
   - 测试函数名清晰描述测试内容
   - 使用中文注释便于团队理解

2. **测试结构**
   - Arrange（准备）→ Act（执行）→ Assert（验证）
   - 每个测试独立运行
   - 正确使用 `defer` 清理资源

3. **错误处理**
   - 使用 `t.Fatalf` 和 `t.Errorf` 报告错误
   - 提供详细的错误信息
   - 避免使用 `panic`

## 相关文档

- [Redis 配置重构文档](../feature-plans/redis-configuration-refactoring.md)
- [Go Testing 官方文档](https://pkg.go.dev/testing)
- [Asynq 库文档](https://github.com/hibiken/asynq)

## 更新记录

| 日期 | 版本 | 更新内容 |
|------|------|----------|
| 2025-12-14 | v1.0 | 初始版本，支持基本测试场景 |
| 2025-12-14 | v1.1 | 添加环境变量前缀 QUKA_TEST_ |
| 2025-12-14 | v1.2 | 优化测试性能和稳定性 |
| 2025-12-14 | v1.3 | 改进队列名称设计，使用独立的 "podcast" 队列，添加 PodcastQueueName 常量 |
| 2025-12-14 | v1.4 | 添加完整的出队测试，包括普通任务和延迟任务的消费流程，验证 worker 正常工作 |