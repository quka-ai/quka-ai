package process

import (
	"context"
	"log/slog"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/register"
)

type JournalProcess struct {
	core *core.Core
}

func NewJournalProcess(core *core.Core) *JournalProcess {
	return &JournalProcess{core: core}
}

func (p *JournalProcess) ClearOldJournals(ctx context.Context) error {
	// 获取31天前的日期
	date := time.Now().AddDate(0, 0, -31).Format("2006-01-02")

	// 清理31天前的journals
	err := p.core.Store().JournalStore().DeleteByDate(ctx, date)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	register.RegisterFunc(ProcessKey{}, func(provider *Process) {
		provider.Cron().AddFunc("0 4 * * *", func() {
			err := NewJournalProcess(provider.Core()).ClearOldJournals(context.Background())
			if err != nil {
				slog.Error("Failed to clear old journals", slog.String("error", err.Error()))
			} else {
				slog.Info("Successfully cleared old journals")
			}
		})
	})
}
