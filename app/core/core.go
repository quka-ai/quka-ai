package core

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/app/store"
	"github.com/quka-ai/quka-ai/app/store/sqlstore"
)

type Core struct {
	cfg       CoreConfig
	cfgReader io.Reader
	srv       *srv.Srv

	prompt Prompt

	stores     func() *sqlstore.Provider
	httpClient *http.Client
	httpEngine *gin.Engine

	metrics *Metrics
	Plugins
}

func MustSetupCore(cfg CoreConfig) *Core {
	{
		var writer io.Writer = os.Stdout
		if cfg.Log.Path != "" {
			writer = &lumberjack.Logger{
				Filename:   cfg.Log.Path,
				MaxSize:    500, // megabytes
				MaxBackups: 3,
				MaxAge:     28,   //days
				Compress:   true, // disabled by default
			}
		}
		l := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level: cfg.Log.SlogLevel(),
		}))
		slog.SetDefault(l)
	}

	core := &Core{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: time.Second * 3},
		metrics:    NewMetrics("quka", "core"),
		httpEngine: gin.New(),
		prompt:     cfg.Prompt,
	}

	// setup store
	setupMysqlStore(core)

	core.srv = srv.SetupSrvs(srv.ApplyAI(cfg.AI), // ai provider select
		// web socket
		srv.ApplyTower())

	return core
}

// TODO: gen with redis
type sg struct {
	msgStore store.ChatMessageStore
}

func (s *sg) GetChatMessageSequence(ctx context.Context, spaceID, sessionID string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	latestChat, err := s.msgStore.GetSessionLatestMessage(ctx, spaceID, sessionID)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	if latestChat == nil {
		return 1, nil
	}
	return latestChat.Sequence + 1, nil
}

func (s *Core) Cfg() CoreConfig {
	return s.cfg
}

func (s *Core) Prompt() Prompt {
	return s.prompt
}

func (s *Core) UpdatePrompt(p Prompt) {
	s.prompt = p
}

func (s *Core) HttpEngine() *gin.Engine {
	return s.httpEngine
}

func (s *Core) Metrics() *Metrics {
	return s.metrics
}

func setupMysqlStore(core *Core) {
	core.stores = sqlstore.MustSetup(core.cfg.Postgres)
}

func (s *Core) Store() *sqlstore.Provider {
	return s.stores()
}

func (s *Core) Srv() *srv.Srv {
	return s.srv
}
