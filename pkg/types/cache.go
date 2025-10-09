package types

import (
	"context"
	"time"
)

// Cache 接口定义了缓存操作的基本方法
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	SetEx(ctx context.Context, key, value string, expiresAt time.Duration) error
	Expire(ctx context.Context, key string, expiration time.Duration) error
}