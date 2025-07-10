package v1

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/samber/lo"
)

type HistoryLogic struct {
	ctx  context.Context
	core *core.Core
	UserInfo
}

func NewHistoryLogic(ctx context.Context, core *core.Core) *HistoryLogic {
	return &HistoryLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}
}

type RelDoc struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Resource string `json:"resource"`
	SpaceID  string `json:"space_id"`
}

type ChatMessageExt struct {
	MessageID        string                     `json:"message_id"`
	SessionID        string                     `json:"session_id"`
	Evaluate         types.EvaluateType         `json:"evaluate"`
	GenerationStatus types.GenerationStatusType `json:"generation_status"`
	RelDocs          []RelDoc                   `json:"rel_docs"` // relevance docs
	Marks            map[string]string          `json:"marks"`
}

func (l *HistoryLogic) GetMessageExt(spaceID, sessionID, messageID string) (*ChatMessageExt, error) {
	data, err := l.core.Store().ChatMessageExtStore().GetChatMessageExt(l.ctx, spaceID, sessionID, messageID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("HistoryLogic.GetMessageExt.ChatMessageExtStore.GetChatMessageExt", i18n.ERROR_INTERNAL, err)
	}

	result := &ChatMessageExt{
		MessageID: messageID,
		SessionID: sessionID,
	}

	if data == nil {
		return result, nil
	}

	result.Evaluate = data.Evaluate
	result.GenerationStatus = data.GenerationStatus

	if len(data.RelDocs) > 0 {
		docs, err := l.core.Store().KnowledgeStore().ListKnowledges(l.ctx, types.GetKnowledgeOptions{
			IDs:     data.RelDocs,
			SpaceID: spaceID,
		}, types.NO_PAGINATION, types.NO_PAGINATION)
		if err != nil {
			return nil, errors.New("HistoryLogic.GetMessageExt.KnowledgeStore.ListKnowledges", i18n.ERROR_INTERNAL, err)
		}

		for _, v := range docs {
			result.RelDocs = append(result.RelDocs, RelDoc{
				Title:    v.Title,
				ID:       v.ID,
				Resource: v.Resource,
				SpaceID:  v.SpaceID,
			})
		}
	}

	return result, nil
}

type MessageDetail struct {
	Meta *types.MessageMeta `json:"meta"`
	Ext  *MessageExt        `json:"ext"`
}

type MessageExt struct {
	IsRead           []string           `json:"is_read"`
	RelDocs          []RelDoc           `json:"rel_docs"`
	Evaluate         types.EvaluateType `json:"evaluate"`
	IsEvaluateEnable bool               `json:"is_evaluate_enable"`
}

func (l *HistoryLogic) GetHistoryMessage(spaceID, sessionID, afterMsgID string, page, pageSize uint64) ([]*MessageDetail, int64, error) {
	list, err := l.core.Store().ChatMessageStore().ListSessionMessage(l.ctx, spaceID, sessionID, afterMsgID, page, pageSize)
	if err != nil && err != sql.ErrNoRows {
		return nil, 0, errors.New("HistoryLogic.GetHistoryMessage.ChatMessageStore.ListSessionMessage", i18n.ERROR_INTERNAL, err)
	}

	total, err := l.core.Store().ChatMessageStore().TotalSessionMessage(l.ctx, spaceID, sessionID, afterMsgID)
	if err != nil {
		return nil, 0, errors.New("HistoryLogic.GetHistoryMessage.TotalDialogMessage", i18n.ERROR_INTERNAL, err)
	}

	msgIDs := lo.Map(list, func(item *types.ChatMessage, _ int) string {
		return item.ID
	})

	extList, err := l.core.Store().ChatMessageExtStore().ListChatMessageExts(l.ctx, msgIDs)
	if err != nil {
		return nil, 0, errors.New("HistoryLogic.GetHistoryMessage.ChatMessageExtStore.ListChatMessageExts", i18n.ERROR_INTERNAL, err)
	}

	extMap := lo.SliceToMap(extList, func(item types.ChatMessageExt) (string, *types.ChatMessageExt) {
		return item.MessageID, &item
	})

	relDocsIDs := make(map[string]struct{})

	detailList := lo.Map(list, func(item *types.ChatMessage, _ int) *types.MessageDetail {
		ext := extMap[item.ID]
		if ext != nil {
			for _, v := range ext.RelDocs {
				relDocsIDs[v] = struct{}{}
			}
		}

		if item.IsEncrypt == types.MESSAGE_IS_ENCRYPT {
			tmp, err := l.core.DecryptData([]byte(item.Message))
			if err != nil {
				slog.Error("Failed to decrypt message content", slog.String("message_id", item.ID), slog.String("error", err.Error()))
			} else {
				item.Message = string(tmp)
			}
		}
		return chatMsgAndExtToMessageDetail(item, ext)
	})

	docs, err := l.core.Store().KnowledgeStore().ListLiteKnowledges(l.ctx, types.GetKnowledgeOptions{
		IDs: lo.MapToSlice(relDocsIDs, func(k string, _ struct{}) string {
			return k
		}),
		SpaceID: spaceID,
	}, types.NO_PAGINATION, types.NO_PAGINATION)

	docsMap := lo.SliceToMap(docs, func(v *types.KnowledgeLite) (string, *types.KnowledgeLite) {
		return v.ID, v
	})
	result := lo.Map(detailList, func(v *types.MessageDetail, k int) *MessageDetail {
		if v.Ext == nil {
			return &MessageDetail{
				Meta: v.Meta,
				Ext:  nil,
			}
		}
		var relDocs []RelDoc
		for _, v := range v.Ext.RelDocs {
			if doc, exist := docsMap[v]; exist {
				relDocs = append(relDocs, RelDoc{
					ID:       doc.ID,
					Title:    doc.Title,
					Resource: doc.Resource,
					SpaceID:  doc.SpaceID,
				})
			}
		}
		return &MessageDetail{
			Meta: v.Meta,
			Ext: &MessageExt{
				IsRead:           v.Ext.IsRead,
				Evaluate:         v.Ext.Evaluate,
				IsEvaluateEnable: v.Ext.IsEvaluateEnable,
				RelDocs:          relDocs,
			},
		}
	})

	return result, total, nil
}

func chatMsgAndExtToMessageDetail(msg *types.ChatMessage, ext *types.ChatMessageExt) *types.MessageDetail {
	if msg == nil {
		return nil
	}

	data := &types.MessageDetail{
		Meta: chatMsgToTextMsg(msg),
	}

	if ext != nil {
		data.Ext = &types.MessageExt{
			Evaluate:         ext.Evaluate,
			RelDocs:          ext.RelDocs,
			IsEvaluateEnable: lo.If(msg.Role == types.USER_ROLE_ASSISTANT, true).Else(false),
		}
	}

	return data
}
