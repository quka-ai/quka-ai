package v1

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/samber/lo"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
)

type FixedPinLogic struct {
	UserInfo
	ctx  context.Context
	core *core.Core
}

func NewFixedPinLogic(ctx context.Context, core *core.Core) *FixedPinLogic {
	return &FixedPinLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}
}

func (l *FixedPinLogic) Get(spaceID string) (*types.FixedPin, error) {
	pin, err := l.core.Store().FixedPinStore().Get(l.ctx, spaceID, l.GetUserInfo().User)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("FixedPinLogic.Get.FixedPinStore.Get", i18n.ERROR_INTERNAL, err)
	}
	if pin == nil {
		return nil, nil
	}

	if len(pin.Content) > 0 {
		if pin.Content, err = l.core.DecryptData(pin.Content); err != nil {
			return nil, errors.New("FixedPinLogic.Get.DecryptData", i18n.ERROR_INTERNAL, err)
		}
	}

	if pin.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS_V2 {
		pin.Content = editorjs.ReplaceBlockNoteBlocksJsonStaticResourcesWithPresignedURL(pin.Content, l.core.Plugins.FileStorage())
	}

	return pin, nil
}

func (l *FixedPinLogic) Upsert(spaceID string, content types.KnowledgeContent, contentType types.KnowledgeContentType) (*types.FixedPin, error) {
	if l.GetUserInfo().User == "" {
		return nil, errors.New("FixedPinLogic.Upsert.UserInfo", i18n.ERROR_UNAUTHORIZED, nil).Code(http.StatusUnauthorized)
	}

	if contentType == "" {
		contentType = types.KNOWLEDGE_CONTENT_TYPE_BLOCKS_V2
	}
	if contentType != types.KNOWLEDGE_CONTENT_TYPE_BLOCKS_V2 {
		return nil, errors.New("FixedPinLogic.Upsert.ContentType", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	content, err := l.normalizeBlockNoteContent(content)
	if err != nil {
		return nil, errors.Trace("FixedPinLogic.Upsert.normalizeBlockNoteContent", err)
	}

	encrypted, err := l.core.EncryptData([]byte(content.String()))
	if err != nil {
		return nil, errors.New("FixedPinLogic.Upsert.EncryptData", i18n.ERROR_INTERNAL, err)
	}

	now := time.Now().Unix()
	if err = l.core.Store().FixedPinStore().Upsert(l.ctx, types.FixedPin{
		ID:          utils.GenRandomID(),
		SpaceID:     spaceID,
		UserID:      l.GetUserInfo().User,
		Content:     encrypted,
		ContentType: contentType,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		return nil, errors.New("FixedPinLogic.Upsert.FixedPinStore.Upsert", i18n.ERROR_INTERNAL, err)
	}

	pin, err := l.Get(spaceID)
	if err != nil {
		return nil, errors.Trace("FixedPinLogic.Upsert.Get", err)
	}
	return pin, nil
}

func (l *FixedPinLogic) Delete(spaceID string) error {
	pin, err := l.core.Store().FixedPinStore().Get(l.ctx, spaceID, l.GetUserInfo().User)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("FixedPinLogic.Delete.FixedPinStore.Get", i18n.ERROR_INTERNAL, err)
	}
	if pin == nil {
		return nil
	}

	actData, err := l.core.DecryptData(pin.Content)
	if err != nil {
		slog.Error("Failed to decrypt fixed pin data for mark file status to delete",
			slog.String("error", err.Error()),
			slog.String("space_id", spaceID),
			slog.String("user_id", l.GetUserInfo().User))
		actData = pin.Content
	}
	if err = UpdateFilesToDelete(l.ctx, l.core, spaceID, actData, types.KNOWLEDGE_CONTENT_TYPE_BLOCKS_V2); err != nil {
		slog.Error("Failed to remark fixed pin files to delete status",
			slog.String("fixed_pin_id", pin.ID),
			slog.String("space_id", spaceID),
			slog.Any("error", err))
	}

	if err = l.core.Store().FixedPinStore().Delete(l.ctx, spaceID, l.GetUserInfo().User); err != nil {
		return errors.New("FixedPinLogic.Delete.FixedPinStore.Delete", i18n.ERROR_INTERNAL, err)
	}
	return nil
}

func (l *FixedPinLogic) normalizeBlockNoteContent(content types.KnowledgeContent) (types.KnowledgeContent, error) {
	var blocks []editorjs.BlockNoteBlock
	if err := json.Unmarshal(json.RawMessage(content), &blocks); err != nil {
		return nil, errors.New("FixedPinLogic.normalizeBlockNoteContent.ParseBlockNoteBlocks", i18n.ERROR_INVALIDARGUMENT, err).Code(http.StatusBadRequest)
	}

	blocks = editorjs.RemoveBlockNoteFileBlockHost(blocks, lo.If(l.core.Cfg().ObjectStorage.S3.UsePathStyle, l.core.Cfg().ObjectStorage.S3.Bucket).Else(""))
	data, err := json.Marshal(blocks)
	if err != nil {
		return nil, errors.New("FixedPinLogic.normalizeBlockNoteContent.RemoveBlockNoteFileBlockHost", i18n.ERROR_INTERNAL, err)
	}

	return data, nil
}
