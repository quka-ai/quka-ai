package selfhost

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"golang.org/x/time/rate"

	"github.com/quka-ai/quka-ai/app/core"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/mark"
	"github.com/quka-ai/quka-ai/pkg/plugins"
	"github.com/quka-ai/quka-ai/pkg/plugins/selfhost/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/plugins/selfhost/srv"
	"github.com/quka-ai/quka-ai/pkg/safe"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
)

func NewSingleLock() *SingleLock {
	return &SingleLock{
		locks: make(map[string]bool),
	}
}

type SingleLock struct {
	mu    sync.Mutex
	locks map[string]bool
}

func (s *SingleLock) TryLock(ctx context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks[key] {
		return false, nil
	}
	s.locks[key] = true
	go safe.Run(func() {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			defer s.mu.Unlock()
			delete(s.locks, key)
		}
	})
	return true, nil
}

func init() {
	plugins.RegisterProvider("selfhost", newSelfHostMode())
}

var _ core.Plugins = (*SelfHostPlugin)(nil)

func newSelfHostMode() *SelfHostPlugin {
	return &SelfHostPlugin{
		Appid:      types.DEFAULT_APPID,
		singleLock: NewSingleLock(),
	}
}

type Cache struct{}

func (c *Cache) SetEx(ctx context.Context, key, value string, expiresAt time.Duration) error {
	return nil
}

func (c *Cache) Expire(ctx context.Context, key string, expiresAt time.Duration) error {
	return nil
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

type SelfHostPlugin struct {
	core       *srv.PluginCore
	Appid      string
	singleLock *SingleLock
	storage    core.FileStorage
	cache      *Cache
}

func (s *SelfHostPlugin) RegisterHTTPEngine(e *gin.Engine) {
	return
}

func (s *SelfHostPlugin) Name() string {
	return "selfhost"
}

func (s *SelfHostPlugin) DefaultAppid() string {
	return s.Appid
}

func (s *SelfHostPlugin) Install(c *core.Core) error {
	fmt.Println("Start initialize.")
	utils.SetupIDWorker(1)

	var err error
	if s.core, err = srv.NewPluginCore(c); err != nil {
		return err
	}

	s.cache = &Cache{}

	defer func() {
		process.SetupProcess(s.core)
	}()

	var tokenCount int
	if err := s.core.AppCore.Store().GetMaster().Get(&tokenCount, "SELECT COUNT(*) FROM "+types.TABLE_ACCESS_TOKEN.Name()+" WHERE true"); err != nil {
		return fmt.Errorf("Initialize sql error: %w", err)
	}

	if tokenCount > 0 {
		fmt.Println("System is already initialized. Skip.")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	var (
		token   string
		spaceID string
	)

	err = s.core.AppCore.Store().Transaction(ctx, func(ctx context.Context) error {
		authLogic := v1.NewAuthLogic(ctx, s.core.AppCore)
		token, err = authLogic.InitAdminUser(s.Appid)
		if err != nil {
			return err
		}

		tokenInfo, err := authLogic.GetAccessTokenDetail(s.Appid, token)
		if err != nil {
			return err
		}

		claims, err := tokenInfo.TokenClaims()
		if err != nil {
			return err
		}
		ctx = context.WithValue(ctx, v1.TOKEN_CONTEXT_KEY, claims)
		spaceID, err = v1.NewSpaceLogic(ctx, s.core.AppCore).CreateUserSpace("default", "default", "", "")
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return err
	}

	fmt.Println("Appid:", s.Appid)
	fmt.Println("Access token:", token)
	fmt.Println("Space id:", spaceID)

	return nil
}

func (s *SelfHostPlugin) Cache() core.Cache {
	return s.cache
}

func (s *SelfHostPlugin) TryLock(ctx context.Context, key string) (bool, error) {
	return s.singleLock.TryLock(ctx, key)
}

type AIChatLogic struct {
	core *core.Core
	plugins.Assistant
}

func (s *SelfHostPlugin) GetChatSessionSeqID(ctx context.Context, spaceID, sessionID string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	latestChat, err := s.core.AppCore.Store().ChatMessageStore().GetSessionLatestMessage(ctx, spaceID, sessionID)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	if latestChat == nil {
		return 1, nil
	}
	return latestChat.Sequence + 1, nil
}

func (s *SelfHostPlugin) GenMessageID() string {
	return utils.GenRandomID()
}

func (s *SelfHostPlugin) AIChatLogic(agentType string) core.AIChatLogic {
	switch agentType {
	case types.AGENT_TYPE_BUTLER:
		return &AIChatLogic{
			core:      s.core.AppCore,
			Assistant: v1.NewBulterAssistant(s.core.AppCore, agentType),
		}
	case types.AGENT_TYPE_JOURNAL:
		return &AIChatLogic{
			core:      s.core.AppCore,
			Assistant: v1.NewJournalAssistant(s.core.AppCore, agentType),
		}
	case types.AGENT_TYPE_NORMAL:
		return &AIChatLogic{
			core:      s.core.AppCore,
			Assistant: v1.NewNormalAssistant(s.core.AppCore, agentType),
		}
	default:
		return &AIChatLogic{
			core:      s.core.AppCore,
			Assistant: v1.NewAutoAssistant(s.core.AppCore, agentType),
		}
	}
}

var limiter = make(map[string]*rate.Limiter)

// ratelimit 代表每分钟允许的数量
func (s *SelfHostPlugin) UseLimiter(c *gin.Context, key string, method string, opts ...core.LimitOption) core.Limiter {
	cfg := &core.LimitConfig{
		Limit: 60,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	l, exist := limiter[key]
	if !exist {
		limit := rate.Every(time.Minute / time.Duration(cfg.Limit))
		limiter[key] = rate.NewLimiter(limit, cfg.Limit*2)
		l = limiter[key]
	}

	return l
}

func (s *SelfHostPlugin) FileStorage() core.FileStorage {
	if s.storage != nil {
		return s.storage
	}

	s.storage = plugins.SetupObjectStorage(s.core.AppCore.Cfg().ObjectStorage)

	return s.storage
}

func (s *SelfHostPlugin) CreateUserDefaultPlan(ctx context.Context, appid, userID string) (string, error) {
	return "pro", nil
}

func (s *SelfHostPlugin) EncryptData(data []byte) ([]byte, error) {
	if s.core.Cfg.EncryptKey == "" {
		return data, nil
	}

	return utils.EncryptCFB(data, []byte(s.core.Cfg.EncryptKey))
}

func (s *SelfHostPlugin) DecryptData(data []byte) ([]byte, error) {
	if s.core.Cfg.EncryptKey == "" {
		return data, nil
	}

	return utils.DecryptCFB(data, []byte(s.core.Cfg.EncryptKey))
}

func (s *SelfHostPlugin) DeleteSpace(ctx context.Context, spaceID string) error {
	return s.core.AppCore.Store().Transaction(ctx, func(ctx context.Context) error {
		if err := s.core.AppCore.Store().ContentTaskStore().DeleteAll(ctx, spaceID); err != nil {
			return err
		}

		if err := s.core.AppCore.Store().KnowledgeRelMetaStore().DeleteAll(ctx, spaceID); err != nil {
			return err
		}

		if err := s.core.AppCore.Store().KnowledgeMetaStore().DeleteAll(ctx, spaceID); err != nil {
			return err
		}
		return nil
	})
}

func (s *SelfHostPlugin) AppendKnowledgeContentToDocs(docs []*types.PassageInfo, knowledges []*types.Knowledge) ([]*types.PassageInfo, error) {
	if len(knowledges) == 0 {
		return docs, nil
	}

	spaceID := knowledges[0].SpaceID
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	spaceResources, err := s.core.AppCore.Store().ResourceStore().ListResources(ctx, spaceID, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		slog.Error("Failed to get space resources", slog.String("space_id", spaceID), slog.String("error", err.Error()))
		return docs, err
	}

	resourceTitle := lo.SliceToMap(spaceResources, func(item types.Resource) (string, string) {
		return item.ID, item.Title
	})

	for _, v := range knowledges {
		content := string(v.Content)
		if v.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS {
			if content, err = editorjs.ConvertEditorJSRawToMarkdown(json.RawMessage(v.Content)); err != nil {
				slog.Error("Failed to convert editor blocks to markdown", slog.String("knowledge_id", v.ID), slog.String("error", err.Error()))
				continue
			}
		}

		// 对所有转换后的markdown内容进行预签名URL替换
		content = editorjs.ReplaceMarkdownStaticResourcesWithPresignedURL(content, s.FileStorage())

		sw := mark.NewSensitiveWork()
		docs = append(docs, &types.PassageInfo{
			ID:       v.ID,
			Content:  sw.Do(content),
			DateTime: v.MaybeDate,
			Resource: lo.If(resourceTitle[v.Resource] != "", resourceTitle[v.Resource]).Else(v.Resource),
			SW:       sw,
		})
	}
	return docs, nil
}

func (s *SelfHostPlugin) Rerank(query string, knowledges []*types.Knowledge) ([]*types.Knowledge, *ai.Usage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if len(knowledges) > 10 {
		res, usage, err := s.core.AppCore.Srv().AI().Rerank(ctx, query, lo.Map(knowledges, func(item *types.Knowledge, _ int) *ai.RerankDoc {
			sw := mark.NewSensitiveWork()
			content := sw.Do(lo.If(item.ContentType == types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN, string(item.Content)).Else(item.Content.String()))
			return &ai.RerankDoc{
				ID:      item.ID,
				Content: content,
			}
		}))

		if err != nil {
			if errors.Is(err, errors.ERROR_UNSUPPORTED_FEATURE) {
				return knowledges, nil, nil
			}
			return nil, usage, err
		}

		firstScore := res[0].Score
		latestScore := firstScore - 0.2
		if firstScore < 0.3 {
			return nil, usage, nil
		}
		if firstScore < 0.5 {
			latestScore = firstScore - 0.05
		}

		docsMap := lo.SliceToMap(knowledges, func(item *types.Knowledge) (string, *types.Knowledge) {
			return item.ID, item
		})

		var result []*types.Knowledge
		for _, v := range res {
			if v.Score < latestScore {
				break
			}
			result = append(result, docsMap[v.ID])
		}
		return result, usage, nil
	}

	return knowledges, nil, nil
}
