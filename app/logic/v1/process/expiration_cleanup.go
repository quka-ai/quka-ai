package process

import (
	"context"
	"log/slog"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/types"
)

// ExpirationCleanupTask 过期内容清理任务
type ExpirationCleanupTask struct {
	core     *core.Core
	strategy CleanupStrategy
}

// CleanupStrategy 清理策略
type CleanupStrategy string

const (
	// StrategyHardDelete 硬删除：从数据库中完全删除
	StrategyHardDelete CleanupStrategy = "hard_delete"
	// StrategySoftDelete 软删除：标记为删除但保留数据
	StrategySoftDelete CleanupStrategy = "soft_delete"
	// StrategyArchive 归档：移动到归档表
	StrategyArchive CleanupStrategy = "archive"
)

// NewExpirationCleanupTask 创建过期清理任务
func NewExpirationCleanupTask(core *core.Core, strategy CleanupStrategy) *ExpirationCleanupTask {
	if strategy == "" {
		strategy = StrategyHardDelete
	}
	return &ExpirationCleanupTask{
		core:     core,
		strategy: strategy,
	}
}

// Run 执行过期内容清理任务
func (t *ExpirationCleanupTask) Run(ctx context.Context) error {
	slog.Info("Starting expiration cleanup task", slog.String("strategy", string(t.strategy)))
	
	batchSize := uint64(1000)
	totalCleaned := 0
	
	for {
		// 分批获取过期的knowledge
		expiredKnowledges, err := t.core.Store().KnowledgeStore().ListKnowledges(ctx, types.GetKnowledgeOptions{
			ExpiredOnly: true,
		}, 1, batchSize)
		
		if err != nil {
			slog.Error("Failed to list expired knowledges", slog.Any("error", err))
			return err
		}
		
		// 如果没有过期内容，结束清理
		if len(expiredKnowledges) == 0 {
			break
		}
		
		// 根据策略处理过期内容
		cleaned, err := t.cleanupBatch(ctx, expiredKnowledges)
		if err != nil {
			slog.Error("Failed to cleanup batch", 
				slog.Int("batch_size", len(expiredKnowledges)),
				slog.Any("error", err))
			return err
		}
		
		totalCleaned += cleaned
		
		// 如果批次大小小于预期，说明已经处理完所有过期内容
		if len(expiredKnowledges) < int(batchSize) {
			break
		}
		
		// 防止过于频繁的数据库操作
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	
	slog.Info("Expiration cleanup task completed", 
		slog.Int("total_cleaned", totalCleaned),
		slog.String("strategy", string(t.strategy)))
	
	return nil
}

// cleanupBatch 批量清理过期内容
func (t *ExpirationCleanupTask) cleanupBatch(ctx context.Context, knowledges []*types.Knowledge) (int, error) {
	cleaned := 0
	
	for _, knowledge := range knowledges {
		err := t.cleanupSingle(ctx, knowledge)
		if err != nil {
			slog.Warn("Failed to cleanup single knowledge",
				slog.String("knowledge_id", knowledge.ID),
				slog.String("title", knowledge.Title),
				slog.Any("error", err))
			continue // 继续处理其他记录，不因单个失败而中断
		}
		cleaned++
		
		slog.Debug("Cleaned up expired knowledge",
			slog.String("knowledge_id", knowledge.ID),
			slog.String("title", knowledge.Title),
			slog.String("resource", knowledge.Resource))
	}
	
	return cleaned, nil
}

// cleanupSingle 清理单个过期knowledge
func (t *ExpirationCleanupTask) cleanupSingle(ctx context.Context, knowledge *types.Knowledge) error {
	switch t.strategy {
	case StrategyHardDelete:
		return t.hardDelete(ctx, knowledge)
	case StrategySoftDelete:
		return t.softDelete(ctx, knowledge)
	case StrategyArchive:
		return t.archive(ctx, knowledge)
	default:
		return t.hardDelete(ctx, knowledge) // 默认硬删除
	}
}

// hardDelete 硬删除：完全从数据库中删除
func (t *ExpirationCleanupTask) hardDelete(ctx context.Context, knowledge *types.Knowledge) error {
	return t.core.Store().Transaction(ctx, func(txCtx context.Context) error {
		// 删除向量数据
		if err := t.core.Store().VectorStore().Delete(txCtx, knowledge.SpaceID, knowledge.ID, knowledge.ID); err != nil {
			slog.Warn("Failed to delete vector data", slog.String("knowledge_id", knowledge.ID), slog.Any("error", err))
		}
		
		// 删除chunk数据
		if err := t.core.Store().KnowledgeChunkStore().BatchDelete(txCtx, knowledge.SpaceID, knowledge.ID); err != nil {
			slog.Warn("Failed to delete chunk data", slog.String("knowledge_id", knowledge.ID), slog.Any("error", err))
		}
		
		// 删除knowledge记录
		return t.core.Store().KnowledgeStore().Delete(txCtx, knowledge.SpaceID, knowledge.ID)
	})
}

// softDelete 软删除：标记删除状态（需要在数据结构中添加deleted_at字段）
func (t *ExpirationCleanupTask) softDelete(ctx context.Context, knowledge *types.Knowledge) error {
	// 注意：这里需要在Knowledge表中添加deleted_at字段
	// 目前暂时使用硬删除，后续可以扩展软删除功能
	slog.Warn("Soft delete not implemented yet, using hard delete", 
		slog.String("knowledge_id", knowledge.ID))
	return t.hardDelete(ctx, knowledge)
}

// archive 归档：移动到归档表（需要创建归档表）
func (t *ExpirationCleanupTask) archive(ctx context.Context, knowledge *types.Knowledge) error {
	// 注意：这里需要创建专门的归档表
	// 目前暂时使用硬删除，后续可以扩展归档功能
	slog.Warn("Archive not implemented yet, using hard delete", 
		slog.String("knowledge_id", knowledge.ID))
	return t.hardDelete(ctx, knowledge)
}

// GetExpiredKnowledgesCount 获取过期knowledge的数量（用于监控）
func (t *ExpirationCleanupTask) GetExpiredKnowledgesCount(ctx context.Context) (uint64, error) {
	return t.core.Store().KnowledgeStore().Total(ctx, types.GetKnowledgeOptions{
		ExpiredOnly: true,
	})
}

// TaskScheduler 任务调度器
type TaskScheduler struct {
	core   *core.Core
	ctx    context.Context
	cancel context.CancelFunc
}

// NewTaskScheduler 创建任务调度器
func NewTaskScheduler(core *core.Core) *TaskScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskScheduler{
		core:   core,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动任务调度器
func (s *TaskScheduler) Start() {
	slog.Info("Starting task scheduler")
	
	// 启动过期清理任务，每小时执行一次
	go s.scheduleExpirationCleanup(time.Hour)
}

// Stop 停止任务调度器
func (s *TaskScheduler) Stop() {
	slog.Info("Stopping task scheduler")
	s.cancel()
}

// scheduleExpirationCleanup 调度过期清理任务
func (s *TaskScheduler) scheduleExpirationCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	// 立即执行一次（可选）
	s.runExpirationCleanup()
	
	for {
		select {
		case <-s.ctx.Done():
			slog.Info("Expiration cleanup scheduler stopped")
			return
		case <-ticker.C:
			s.runExpirationCleanup()
		}
	}
}

// runExpirationCleanup 运行过期清理任务
func (s *TaskScheduler) runExpirationCleanup() {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Minute)
	defer cancel()
	
	task := NewExpirationCleanupTask(s.core, StrategyHardDelete)
	
	// 先获取过期数量用于日志
	count, err := task.GetExpiredKnowledgesCount(ctx)
	if err != nil {
		slog.Error("Failed to get expired knowledges count", slog.Any("error", err))
	} else {
		slog.Info("Starting scheduled expiration cleanup", slog.Uint64("expired_count", count))
	}
	
	// 执行清理任务
	if err := task.Run(ctx); err != nil {
		slog.Error("Scheduled expiration cleanup failed", slog.Any("error", err))
	} else {
		slog.Info("Scheduled expiration cleanup completed successfully")
	}
}