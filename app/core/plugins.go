package core

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/object-storage/s3"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type Plugins interface {
	Name() string
	Install(*Core) error
	DefaultAppid() string
	TryLock(ctx context.Context, key string) (bool, error)
	UseLimiter(c *gin.Context, key string, method string, opts ...LimitOption) Limiter
	FileStorage() FileStorage
	CreateUserDefaultPlan(ctx context.Context, appid, userID string) (string, error)
	AIChatLogic(agentType string) AIChatLogic
	GetChatSessionSeqID(ctx context.Context, spaceID, sessionID string) (int64, error)
	GenMessageID() string
	EncryptData(data []byte) ([]byte, error)
	DecryptData(data []byte) ([]byte, error)
	DeleteSpace(ctx context.Context, spaceID string) error
	Rerank(query string, knowledges []*types.Knowledge) ([]*types.Knowledge, *ai.Usage, error)
	AppendKnowledgeContentToDocs(docs []*types.PassageInfo, knowledges []*types.Knowledge) ([]*types.PassageInfo, error)
}

type LimitConfig struct {
	Limit int
	Every time.Duration
}

type LimitOption func(l *LimitConfig)

func WithLimit(limit int) LimitOption {
	return func(l *LimitConfig) {
		l.Limit = limit
	}
}

func WithRange(r time.Duration) LimitOption {
	return func(l *LimitConfig) {
		l.Every = r
	}
}

type AIChatLogic interface {
	RequestAssistant(ctx context.Context, reqMsgInfo *types.ChatMessage, receiver types.Receiver, opts *types.AICallOptions) error
}

type UploadFileMeta struct {
	UploadEndpoint string `json:"endpoint"`
	FullPath       string `json:"full_path"`
	Domain         string `json:"domain"`
	Status         string `json:"status"`
}

// FileStorage interface defines methods for file operations.
type FileStorage interface {
	GetStaticDomain() string
	GenUploadFileMeta(fullPath string, contentLength int64) (UploadFileMeta, error)
	SaveFile(fullPath string, content []byte) error
	DeleteFile(fullFilePath string) error
	GenGetObjectPreSignURL(url string) (string, error)
	DownloadFile(ctx context.Context, filePath string) (*s3.GetObjectResult, error)
}

type Limiter interface {
	Allow() bool
}

type SetupFunc func() Plugins

func (c *Core) InstallPlugins(p Plugins) {
	if err := p.Install(c); err != nil {
		panic(err)
	}
	c.Plugins = p

	// after plugins installed
	SetupSrv(c)
	// 为 sqlstore.Provider 设置 cache 函数
	c.stores().SetCacheFunc(func() types.Cache {
		return &Cache{
			redis: c.Redis(),
		}
	})
}

type Cache struct {
	redis redis.UniversalClient
}

func (c *Cache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.redis.Expire(ctx, key, expiration).Err()
}

func (c *Cache) SetEx(ctx context.Context, key, value string, expiresAt time.Duration) error {
	return c.redis.SetEx(ctx, key, value, expiresAt).Err()
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	return c.redis.Get(ctx, key).Result()
}
