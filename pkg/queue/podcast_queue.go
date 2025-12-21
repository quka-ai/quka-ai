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
	// Podcast 任务类型
	TaskTypePodcastGeneration = "podcast:generation"

	// Podcast 队列名称
	PodcastQueueName = "podcast"

	// Podcast 任务配置
	PodcastMaxRetries  = 3
	PodcastTaskTimeout = 30 * time.Minute // TTS 生成可能需要较长时间
)

// PodcastGenerationTask Podcast生成任务
type PodcastGenerationTask struct {
	PodcastID string `json:"podcast_id"`
}

// PodcastQueue Podcast队列管理器
type PodcastQueue struct {
	client    *asynq.Client
	keyPrefix string // TODO: Will support after asynq released
}

// NewPodcastQueueWithClientServer 使用已存在的 Client 和 Server 创建队列
// 适用于多个队列共享同一个 asynq 连接的场景
func NewPodcastQueueWithClientServer(keyPrefix string, client *asynq.Client) *PodcastQueue {
	if keyPrefix == "" {
		keyPrefix = "quka"
	}

	return &PodcastQueue{
		keyPrefix: keyPrefix,
		client:    client,
	}
}

// EnqueueGenerationTask 将 Podcast 生成任务加入队列
func (q *PodcastQueue) EnqueueGenerationTask(ctx context.Context, podcastID string) error {
	task := &PodcastGenerationTask{
		PodcastID: podcastID,
	}

	payload, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// 创建任务
	_, err = q.client.EnqueueContext(ctx, asynq.NewTask(TaskTypePodcastGeneration, payload,
		asynq.MaxRetry(PodcastMaxRetries),
		asynq.Timeout(PodcastTaskTimeout),
		asynq.Unique(5*time.Minute),
		asynq.Queue(PodcastQueueName),
	))

	if err != nil {
		return fmt.Errorf("failed to enqueue podcast generation task: %w", err)
	}

	slog.Info("Podcast generation task enqueued",
		slog.String("podcast_id", podcastID))

	return nil
}

// EnqueueDelayedGenerationTask 将 Podcast 生成任务加入延迟队列（用于重试）
func (q *PodcastQueue) EnqueueDelayedGenerationTask(ctx context.Context, podcastID string, delay time.Duration) error {
	task := &PodcastGenerationTask{
		PodcastID: podcastID,
	}

	payload, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	_, err = q.client.EnqueueContext(ctx, asynq.NewTask(TaskTypePodcastGeneration, payload,
		asynq.MaxRetry(PodcastMaxRetries),
		asynq.Timeout(PodcastTaskTimeout),
		asynq.ProcessIn(delay),
		asynq.Queue(PodcastQueueName), // 指定队列名称
	))

	if err != nil {
		return fmt.Errorf("failed to enqueue delayed podcast generation task: %w", err)
	}

	slog.Info("Podcast generation task scheduled for delayed execution",
		slog.String("podcast_id", podcastID),
		slog.Duration("delay", delay))

	return nil
}

// Shutdown 优雅关闭队列资源
func (q *PodcastQueue) Shutdown() {
	slog.Info("Shutting down Podcast queue")

	// 关闭 client
	if q.client != nil {
		if err := q.client.Close(); err != nil {
			slog.Error("Failed to close podcast queue client", slog.String("error", err.Error()))
		} else {
			slog.Info("Podcast queue client closed")
		}
	}
}
