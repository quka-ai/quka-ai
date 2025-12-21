package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

const (
	// 任务类型
	TaskTypeRSSFetch = "rss:fetch"

	// RSS 队列名称
	RSSQueueName = "rss"

	// 重试和超时配置
	MaxRetries  = 3
	TaskTimeout = 15 * time.Minute
)

// RSSFetchTask RSS抓取任务
type RSSFetchTask struct {
	SubscriptionID string `json:"subscription_id"`
}

// RSSQueue 基于 Asynq 的队列管理器
type RSSQueue struct {
	client    *asynq.Client
	server    *asynq.Server
	keyPrefix string
}

// NewRSSQueueWithClientServer 使用已存在的 Client 和 Server 创建队列
// 适用于多个队列共享同一个 asynq 连接的场景
func NewRSSQueueWithClientServer(keyPrefix string, client *asynq.Client) *RSSQueue {
	if keyPrefix == "" {
		keyPrefix = "quka"
	}

	return &RSSQueue{
		keyPrefix: keyPrefix,
		client:    client,
	}
}

// EnqueueTask 将任务加入队列
func (q *RSSQueue) EnqueueTask(ctx context.Context, subscriptionID string) error {
	task := &RSSFetchTask{
		SubscriptionID: subscriptionID,
	}

	payload, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// 创建任务
	_, err = q.client.EnqueueContext(ctx, asynq.NewTask(TaskTypeRSSFetch, payload,
		asynq.MaxRetry(MaxRetries),
		asynq.Timeout(TaskTimeout),
		asynq.Unique(time.Hour), // 1小时内不重复
		asynq.Queue(RSSQueueName),
	))

	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	slog.Info("RSS fetch task enqueued",
		slog.String("subscription_id", subscriptionID))

	return nil
}

// EnqueueDelayedTask 将任务加入延迟队列
func (q *RSSQueue) EnqueueDelayedTask(ctx context.Context, subscriptionID string, delay time.Duration) error {
	task := &RSSFetchTask{
		SubscriptionID: subscriptionID,
	}

	payload, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	_, err = q.client.EnqueueContext(ctx, asynq.NewTask(TaskTypeRSSFetch, payload,
		asynq.MaxRetry(MaxRetries),
		asynq.Timeout(TaskTimeout),
		asynq.ProcessIn(delay),
		asynq.Unique(time.Hour),
		asynq.Queue(RSSQueueName), // 指定队列名称
	))

	if err != nil {
		return fmt.Errorf("failed to enqueue delayed task: %w", err)
	}

	slog.Info("RSS fetch task scheduled for delayed execution",
		slog.String("subscription_id", subscriptionID),
		slog.Duration("delay", delay))

	return nil
}

// HandlerFunc Asynq 任务处理器函数类型
type HandlerFunc func(ctx context.Context, task *asynq.Task) error

// SetupHandler 设置任务处理器
func (q *RSSQueue) SetupHandler(handler HandlerFunc) *asynq.ServeMux {
	mux := asynq.NewServeMux()

	mux.HandleFunc(TaskTypeRSSFetch, func(ctx context.Context, task *asynq.Task) error {
		return handler(ctx, task)
	})

	return mux
}

// asynqLogger 适配器，将 asynq 日志输出到项目的 slog
type asynqLogger struct{}

func NewAsynqLogger() *asynqLogger {
	return &asynqLogger{}
}

func (l *asynqLogger) Debug(args ...any) {
	slog.Debug(fmt.Sprint(args...))
}

func (l *asynqLogger) Info(args ...any) {
	slog.Info(fmt.Sprint(args...))
}

func (l *asynqLogger) Warn(args ...any) {
	slog.Warn(fmt.Sprint(args...))
}

func (l *asynqLogger) Error(args ...any) {
	slog.Error(fmt.Sprint(args...))
}

func (l *asynqLogger) Fatal(args ...any) {
	slog.Error(fmt.Sprint(args...))
	panic(fmt.Sprint(args...))
}

// StartWorker 启动 worker（运行 Server）
// 如果在创建时未指定并发数，此方法会 panic
func (q *RSSQueue) StartWorker(mux *asynq.ServeMux) error {
	if q.server == nil {
		panic("server not initialized: concurrency must be > 0 when creating RSSQueue")
	}
	return q.server.Run(mux)
}

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

	// 关闭 server
	if q.server != nil {
		q.server.Shutdown()
		slog.Info("Asynq server shutdown initiated")
	}
}
