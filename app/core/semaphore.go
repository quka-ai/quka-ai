package core

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/quka-ai/quka-ai/pkg/types/protocol"
)

// DistributedSemaphore 分布式信号量，基于 Redis 实现
type DistributedSemaphore struct {
	redis      redis.UniversalClient
	key        string
	maxPermits int
	timeout    time.Duration
}

// NewDistributedSemaphore 创建分布式信号量
func NewDistributedSemaphore(redis redis.UniversalClient, key string, maxPermits int, timeout time.Duration) *DistributedSemaphore {
	return &DistributedSemaphore{
		redis:      redis,
		key:        key,
		maxPermits: maxPermits,
		timeout:    timeout,
	}
}

// TryAcquire 尝试获取信号量许可
func (s *DistributedSemaphore) TryAcquire() bool {
	ctx := context.Background()

	// 使用 Lua 脚本保证原子性
	script := `
		local key = KEYS[1]
		local max_permits = tonumber(ARGV[1])
		local timeout = tonumber(ARGV[2])

		local current = tonumber(redis.call('GET', key) or '0')

		if current < max_permits then
			redis.call('INCR', key)
			redis.call('EXPIRE', key, timeout)
			return 1
		else
			return 0
		end
	`

	result, err := s.redis.Eval(ctx, script, []string{s.key}, s.maxPermits, int(s.timeout.Seconds())).Int()
	if err != nil {
		return false
	}

	return result == 1
}

// Release 释放信号量许可
func (s *DistributedSemaphore) Release() {
	ctx := context.Background()

	// 使用 Lua 脚本保证原子性，避免减到负数
	script := `
		local key = KEYS[1]
		local current = tonumber(redis.call('GET', key) or '0')

		if current > 0 then
			redis.call('DECR', key)
			return 1
		else
			return 0
		end
	`

	s.redis.Eval(ctx, script, []string{s.key})
}

// GetCurrent 获取当前已使用的许可数
func (s *DistributedSemaphore) GetCurrent() int {
	ctx := context.Background()
	result, err := s.redis.Get(ctx, s.key).Int()
	if err != nil {
		return 0
	}
	return result
}

// SemaphoreManager 信号量管理器，统一管理所有分布式信号量
type SemaphoreManager struct {
	core                 *Core
	knowledgeSummary     *DistributedSemaphore
	knowledgeSummaryOnce sync.Once
}

// NewSemaphoreManager 创建信号量管理器
func NewSemaphoreManager(core *Core) *SemaphoreManager {
	return &SemaphoreManager{
		core: core,
	}
}

// KnowledgeSummary 获取知识总结信号量（懒加载）
// 默认限制：同时最多 10 个用户可以生成知识总结
func (m *SemaphoreManager) KnowledgeSummary() *DistributedSemaphore {
	m.knowledgeSummaryOnce.Do(func() {
		maxConcurrency := 10 // 默认值
		if m.core.cfg.Semaphore.Knowledge.SummaryMaxConcurrency > 0 {
			maxConcurrency = m.core.cfg.Semaphore.Knowledge.SummaryMaxConcurrency
		}

		m.knowledgeSummary = NewDistributedSemaphore(
			m.core.Redis(),
			protocol.GenKnowledgeSummaryGlobalSemaphoreKey(),
			maxConcurrency,
			time.Minute*5, // 5分钟超时
		)
	})
	return m.knowledgeSummary
}

// 未来可以添加更多信号量，例如：
// func (m *SemaphoreManager) FileUpload() *DistributedSemaphore { ... }
// func (m *SemaphoreManager) AIGeneration() *DistributedSemaphore { ... }
