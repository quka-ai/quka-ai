package v1

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type JournalLogic struct {
	UserInfo
	ctx  context.Context
	core *core.Core
}

func NewJournalLogic(ctx context.Context, core *core.Core) *JournalLogic {
	return &JournalLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}
}

func (l *JournalLogic) CreateJournal(spaceID, date string, content types.KnowledgeContent) error {
	if l.GetUserInfo().User == "" {
		return errors.New("JournalLogic.CreateJournal.check", i18n.ERROR_UNAUTHORIZED, nil).Code(http.StatusUnauthorized)
	}

	_, err := l.core.Store().JournalStore().Get(l.ctx, spaceID, l.GetUserInfo().User, date)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("JournalLogic.CreateJournal.JournalStore.Get", i18n.ERROR_INTERNAL, err)
	}

	if content, err = l.core.EncryptData(content); err != nil {
		return errors.New("JournalLogic.CreateJournal.EncryptData", i18n.ERROR_INTERNAL, err)
	}
	err = l.core.Store().JournalStore().Create(l.ctx, types.Journal{
		ID:        utils.GenUniqID(),
		SpaceID:   spaceID,
		UserID:    l.GetUserInfo().User,
		Date:      date,
		Content:   content,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	})
	if err != nil {
		return errors.New("JournalLogic.CreateJournal.JournalStore.Create", i18n.ERROR_INTERNAL, err)
	}
	return nil
}

func (l *JournalLogic) UpsertJournal(spaceID, date string, content types.KnowledgeContent) error {
	journal, err := l.core.Store().JournalStore().Get(l.ctx, spaceID, l.GetUserInfo().User, date)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("JournalLogic.UpsertJournal.JournalStore.Get", i18n.ERROR_INTERNAL, err)
	}

	if journal == nil {
		if err = l.CreateJournal(spaceID, date, content); err != nil {
			return errors.Trace("JournalLogic.CreateJournal", err)
		}

		journal, err = l.core.Store().JournalStore().Get(l.ctx, spaceID, l.GetUserInfo().User, date)
		if err != nil {
			return errors.New("JournalLogic.UpsertJournal.JournalStore.Get", i18n.ERROR_INTERNAL, err)
		}
	}

	if journal.UserID != l.GetUserInfo().User {
		return errors.New("JournalLogic.UpsertJournal.auth.check", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	if content, err = l.core.EncryptData(content); err != nil {
		return errors.New("JournalLogic.UpsertJournal.EncryptData", i18n.ERROR_INTERNAL, err)
	}
	err = l.core.Store().JournalStore().Update(l.ctx, journal.ID, content)
	if err != nil {
		return errors.New("JournalLogic.UpsertJournal.JournalStore.Update", i18n.ERROR_INTERNAL, err)
	}
	return nil
}

func (l *JournalLogic) GetJournal(spaceID, date string) (*types.Journal, error) {
	journal, err := l.core.Store().JournalStore().Get(l.ctx, spaceID, l.GetUserInfo().User, date)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("JournalLogic.GetJournal.JournalStore.Get", i18n.ERROR_INTERNAL, err)
	}

	if journal == nil {
		return nil, nil
	}

	if len(journal.Content) > 0 {
		if journal.Content, err = l.core.DecryptData(journal.Content); err != nil {
			return nil, errors.New("JournalLogic.GetJournal.DecryptData", i18n.ERROR_INTERNAL, err)
		}
	}

	return journal, nil
}

func (l *JournalLogic) ListJournals(spaceID, startDate, endDate string) ([]types.Journal, error) {
	// 如果没有提供开始和结束日期，默认展示最近7天的记录
	if startDate == "" || endDate == "" {
		endDate = time.Now().Format("2006-01-02")
		startDate = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}

	list, err := l.core.Store().JournalStore().ListWithDate(l.ctx, spaceID, l.GetUserInfo().User, startDate, endDate)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("JournalLogic.ListJournals.JournalStore.ListWithDate", i18n.ERROR_INTERNAL, err)
	}
	return list, nil
}

func (l *JournalLogic) UpdateJournal(spaceID, date string, content types.KnowledgeContent) error {
	journal, err := l.core.Store().JournalStore().Get(l.ctx, spaceID, l.GetUserInfo().User, date)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("JournalLogic.UpdateJournal.JournalStore.Get", i18n.ERROR_INTERNAL, err)
	}

	if journal == nil {
		return errors.New("JournalLogic.UpdateJournal.JournalStore.Get.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusForbidden)
	}

	err = l.core.Store().JournalStore().Update(l.ctx, journal.ID, content)
	if err != nil {
		return errors.New("JournalLogic.UpdateJournal.JournalStore.Update", i18n.ERROR_INTERNAL, err)
	}
	return nil
}

func (l *JournalLogic) DeleteJournal(spaceID, date string) error {
	journal, err := l.core.Store().JournalStore().Get(l.ctx, spaceID, l.GetUserInfo().User, date)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("JournalLogic.DeleteJournal.JournalStore.Get", i18n.ERROR_INTERNAL, err)
	}

	if journal == nil {
		return errors.New("JournalLogic.DeleteJournal.JournalStore.Get.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusForbidden)
	}

	err = l.core.Store().JournalStore().Delete(l.ctx, journal.ID)
	if err != nil {
		return errors.New("JournalLogic.DeleteJournal.JournalStore.Delete", i18n.ERROR_INTERNAL, err)
	}
	return nil
}
