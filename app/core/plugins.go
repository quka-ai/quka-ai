package core

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

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
	Cache() types.Cache
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

// Cache 类型别名，用于向后兼容
type Cache = types.Cache

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
	
	// 为 sqlstore.Provider 设置 cache 函数
	c.stores().SetCacheFunc(func() types.Cache {
		return c.Plugins.Cache()
	})
}
