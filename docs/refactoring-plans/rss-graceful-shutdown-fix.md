# RSS 进程优雅退出修复方案

## 问题描述

在增加 RSS 相关的 process 后，进程在收到退出命令（SIGTERM/SIGINT）后无法正常退出，出现 hang 住的情况。

## 根本原因分析

### 1. Asynq Server 阻塞未关闭
- 在 `app/logic/v1/process/rss_consumer.go:78`，`rssQueue.StartWorker(mux)` 调用了 `asynq.Server.Run()`
- `asynq.Server.Run()` 是一个**阻塞调用**，会一直运行直到显式调用 `Shutdown()`
- 虽然在 goroutine 中启动，但主进程退出时没有调用 `Shutdown()`，导致该 goroutine 永远不会退出

### 2. HTTP Server 阻塞未关闭（关键问题）
- 在 `cmd/service/router.go:28`，`core.HttpEngine().Run(address)` 调用了 `gin.Engine.Run()`
- `gin.Engine.Run()` 内部启动 `http.Server` 并**阻塞运行**
- 没有优雅关闭机制，收到退出信号时无法正常退出
- **这是主要的阻塞点**，即使修复了 RSS Queue，HTTP Server 仍会阻塞

### 3. 资源清理机制缺失
- `Process` 结构体没有 `Stop()` 方法来清理资源
- Asynq client 和 server 没有被正确关闭
- 没有维护对这些资源的引用，无法在退出时清理

### 4. Goroutine 泄露
- `go startRSSConsumer(p.Core())` 启动的 goroutine 永远不会退出
- 主程序即使收到退出信号，也会被这个 goroutine 阻塞

## 修复方案

### 1. 修改 HTTP Server 支持优雅关闭（最关键）

**文件**: `cmd/service/router.go`

这是最重要的修复，解决了主要的阻塞问题。

```go
func serve(core *core.Core) {
	httpSrv := &handler.HttpSrv{
		Core:   core,
		Engine: core.HttpEngine(),
	}
	setupHttpRouter(httpSrv)

	address := core.Cfg().Addr
	if address == "" {
		address = ":33033"
	}

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:    address,
		Handler: core.HttpEngine(),
	}

	// 在 goroutine 中启动服务器
	go func() {
		slog.Info("HTTP server starting", slog.String("address", address))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", slog.String("error", err.Error()))
		}
	}()

	// 等待中断信号以优雅关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down HTTP server...")

	// 设置 5 秒的超时时间来关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("HTTP server forced to shutdown", slog.String("error", err.Error()))
	}

	slog.Info("HTTP server exited")
}
```

**关键点**:
- 使用 `http.Server` 而不是直接调用 `gin.Engine.Run()`
- 在 goroutine 中启动服务器，主线程监听信号
- 收到 SIGTERM/SIGINT 后调用 `srv.Shutdown(ctx)` 优雅关闭
- 设置 5 秒超时，防止关闭时间过长
- `srv.Shutdown()` 会停止接受新连接，并等待现有连接完成

### 2. 为 RSSQueue 添加 Shutdown 方法

**文件**: `pkg/queue/rss_queue.go`

```go
// Shutdown 优雅关闭队列资源
func (q *RSSQueue) Shutdown() {
	slog.Info("Shutting down RSS queue")

	// 关闭 client
	if q.client != nil {
		if err := q.client.Close(); err != nil {
			slog.Error("Failed to close asynq client", slog.String("error", err.Error()))
		} else {
			slog.Info("Asynq client closed")
		}
	}

	// 关闭 server（这会让 StartWorker 返回）
	if q.server != nil {
		q.server.Shutdown()
		slog.Info("Asynq server shutdown initiated")
	}
}
```

**关键点**:
- `q.server.Shutdown()` 会优雅地关闭 asynq server，停止接收新任务并等待当前任务完成
- `q.client.Close()` 关闭客户端连接

### 2. 在 Process 中维护 RSSQueue 实例

**文件**: `app/logic/v1/process/process.go`

```go
type Process struct {
	cron     *cron.Cron
	core     *core.Core
	rssQueue *queue.RSSQueue  // 新增字段
}

func (p *Process) Stop() {
	// 停止 cron 调度器
	if p.cron != nil {
		ctx := p.cron.Stop()
		<-ctx.Done()
	}

	// 关闭 RSS Queue
	if p.rssQueue != nil {
		p.rssQueue.Shutdown()
	}
}

func (p *Process) SetRSSQueue(rssQueue *queue.RSSQueue) {
	p.rssQueue = rssQueue
}
```

**关键点**:
- 添加 `rssQueue` 字段保存队列实例
- `Stop()` 方法按顺序关闭 cron 和 RSS queue
- `p.cron.Stop()` 返回的 context 会在所有任务完成后 Done

### 3. 修改 startRSSConsumer 保存队列实例

**文件**: `app/logic/v1/process/rss_consumer.go`

```go
func init() {
	register.RegisterFunc[*Process](ProcessKey{}, func(p *Process) {
		// 启动 RSS 消费者（传入 Process 而不是 Core）
		go startRSSConsumer(p)

		// 每5分钟检查一次需要更新的订阅
		p.Cron().AddFunc("*/5 * * * *", func() {
			enqueueSubscriptionsNeedingUpdate(p.Core())
		})

		slog.Info("RSS task consumers started")
	})
}

func startRSSConsumer(p *Process) {
	core := p.Core()

	// 创建 RSSQueue，并发数为 3
	cfg := core.Cfg().Redis
	rssQueue := queue.NewRSSQueue(&cfg, 3)

	// 保存 RSSQueue 实例到 Process，以便在 Stop 时关闭
	p.SetRSSQueue(rssQueue)

	// ... 设置任务处理器 ...

	// 启动 worker（这是阻塞调用，但在 goroutine 中运行）
	if err := rssQueue.StartWorker(mux); err != nil {
		slog.Error("RSS worker failed to start", slog.String("error", err.Error()))
	}
}
```

**关键点**:
- 修改函数签名接收 `*Process` 而不是 `*core.Core`
- 使用 `p.SetRSSQueue(rssQueue)` 保存队列实例
- 当 `p.Stop()` 被调用时，会触发 `rssQueue.Shutdown()`，从而让 `StartWorker` 返回

### 4. 在主程序中调用 Stop

**文件**: `cmd/service/command.go`

#### 修改 Run 函数（service 模式）

```go
func Run(opts *Options) error {
	app := core.MustSetupCore(core.MustLoadBaseConfig(opts.ConfigPath))
	plugins.Setup(app.InstallPlugins, opts.Init)
	p := process.NewProcess(app)
	p.Start()

	// 在服务退出时确保清理资源
	defer p.Stop()

	serve(app)

	return nil
}
```

#### 修改 RunProcess 函数（process 模式）

```go
func RunProcess(opts *Options) error {
	app := core.MustSetupCore(core.MustLoadBaseConfig(opts.ConfigPath))
	plugins.Setup(app.InstallPlugins, opts.Init)
	p := process.NewProcess(app)
	p.Start()

	fmt.Println("Process starting...")
	sigs := make(chan os.Signal, 1)
	// 监听 os.Interrupt (Ctrl+C) 和 syscall.SIGTERM (kill)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	// 阻塞等待信号
	<-sigs

	fmt.Println("Shutting down process...")
	p.Stop()
	fmt.Println("Process stopped successfully")

	return nil
}
```

**关键点**:
- 保存 `process.NewProcess(app)` 的返回值
- 在 `service` 模式中使用 `defer p.Stop()` 确保退出时清理
- 在 `process` 模式中显式调用 `p.Stop()`

## 优雅关闭流程

### 当收到退出信号时：

1. **Process.Stop()** 被调用
2. **停止 Cron 调度器**
   - `p.cron.Stop()` 停止调度新任务
   - 等待所有正在执行的 cron 任务完成
3. **关闭 RSS Queue**
   - `rssQueue.Shutdown()` 被调用
   - Asynq server 停止接收新任务
   - 等待当前正在处理的任务完成
   - 关闭 Asynq client 和 server
4. **StartWorker 返回**
   - `startRSSConsumer` goroutine 退出
5. **主程序退出**

## 验证

编译成功：
```bash
go build -o quka ./cmd/
```

## 测试建议

1. 启动进程：`./quka process -c config.toml`
2. 等待 RSS 消费者启动并处理一些任务
3. 发送 SIGTERM 信号：`kill <pid>` 或按 Ctrl+C
4. 观察日志输出：
   - 应该看到 "Shutting down process..." 消息
   - 应该看到 "Shutting down RSS queue" 消息
   - 应该看到 "Asynq server shutdown initiated" 消息
   - 应该看到 "Process stopped successfully" 消息
5. 进程应该在几秒内正常退出（不再 hang 住）

## 相关文件

- **`cmd/service/router.go`**: 修改 serve 函数支持 HTTP Server 优雅关闭（最关键）
- `pkg/queue/rss_queue.go`: 添加了 Shutdown 方法
- `app/logic/v1/process/process.go`: 添加了 Stop 方法和 rssQueue 字段
- `app/logic/v1/process/rss_consumer.go`: 修改了 startRSSConsumer 以保存队列实例
- `cmd/service/command.go`: 在主程序中调用 Stop 方法

## 注意事项

1. **Asynq Server 的 Shutdown 是优雅的**：它会等待当前正在处理的任务完成，而不是强制中断
2. **如果任务执行时间很长**：进程可能需要等待较长时间才能完全退出
3. **Cron 任务也会等待完成**：确保 cron 任务不会执行过长时间
4. **多次调用 Stop 是安全的**：由于有 nil 检查，重复调用不会 panic

## 后续优化建议

1. 考虑添加超时机制：如果任务执行时间过长，可以强制退出
2. 添加更详细的关闭日志，方便调试
3. 考虑使用 context 来传播取消信号到所有 goroutine
