package process

import (
	"context"
	"log/slog"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type MessageProcess struct {
	core *core.Core
}

func NewMessageProcess(core *core.Core) *MessageProcess {
	return &MessageProcess{core: core}
}

func (p *MessageProcess) ClearOldSession(ctx context.Context) error {
	// 获取31天前的日期
	date := time.Now().AddDate(0, 0, -31)

	// 清理31天前的Session
	list, err := p.core.Store().ChatSessionStore().ListBeforeTime(ctx, date, 1, 50)
	if err != nil {
		return err
	}

	for _, v := range list {
		err = p.core.Store().Transaction(ctx, func(ctx context.Context) error {
			if err := p.core.Store().ChatSessionStore().Delete(ctx, v.SpaceID, v.ID); err != nil {
				return err
			}

			if err := p.core.Store().ChatSessionPinStore().Delete(ctx, v.SpaceID, v.ID); err != nil {
				return err
			}

			if err := p.core.Store().ChatMessageStore().DeleteSessionMessage(ctx, v.SpaceID, v.ID); err != nil {
				return err
			}

			if err := p.core.Store().ChatMessageExtStore().DeleteSessionMessageExt(ctx, v.SpaceID, v.ID); err != nil {
				return err
			}

			if err := p.core.Store().ChatSummaryStore().DeleteSessionSummary(ctx, v.ID); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *MessageProcess) EncryptMessage(ctx context.Context) error {
	res, err := p.core.Store().ChatMessageStore().ListUnEncryptMessage(ctx, 1, 50)
	if err != nil {
		return err
	}

	for _, v := range res {
		if v.IsEncrypt == types.MESSAGE_IS_ENCRYPT || v.Complete != types.MESSAGE_PROGRESS_COMPLETE {
			continue
		}

		tmp, err := p.core.EncryptData([]byte(v.Message))
		if err != nil {
			slog.Error("Failed to encrypt message", slog.String("error", err.Error()), slog.String("id", v.ID))
			continue
		}

		if err = p.core.Store().ChatMessageStore().SaveEncrypt(ctx, v.ID, tmp); err != nil {
			return err
		}
	}

	return nil
}

func init() {
	register.RegisterFunc(ProcessKey{}, func(provider *Process) {
		provider.Cron().AddFunc("*/5 * * * *", func() {
			err := NewMessageProcess(provider.Core()).EncryptMessage(context.Background())
			if err != nil {
				slog.Error("Failed to encrypt chat message", slog.String("error", err.Error()))
			} else {
				slog.Info("Successfully encrypt chat message")
			}
		})

		provider.Cron().AddFunc("0 3 * * *", func() {
			err := NewMessageProcess(provider.Core()).ClearOldSession(context.Background())
			if err != nil {
				slog.Error("Failed to clear old session", slog.String("error", err.Error()))
			} else {
				slog.Info("Successfully clear old session")
			}
		})
	})
}
