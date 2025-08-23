package v1

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
	"github.com/sashabaranov/go-openai"
)

type ChatSessionLogic struct {
	ctx  context.Context
	core *core.Core
	UserInfo
}

func NewChatSessionLogic(ctx context.Context, core *core.Core) *ChatSessionLogic {
	return &ChatSessionLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}
}

func (l *ChatSessionLogic) CheckUserChatSession(spaceID, sessionID string) (*types.ChatSession, error) {
	session, err := l.core.Store().ChatSessionStore().GetChatSession(l.ctx, spaceID, sessionID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ChatSessionLogic.CheckUserChatSession.ChatSessionStore.GetChatSession", i18n.ERROR_INTERNAL, err)
	}
	if session == nil {
		return nil, errors.New("ChatSessionLogic.CheckUserChatSession.ChatSessionStore.GetChatSessionnil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNotFound)
	}

	if session.UserID != l.GetUserInfo().User {
		return nil, errors.New("ChatSessionLogic.CheckUserChatSession.unauth", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	return session, nil
}

func (l *ChatSessionLogic) DeleteChatSession(spaceID, sessionID string) error {
	l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		if err := l.core.Store().ChatSessionStore().Delete(ctx, spaceID, sessionID); err != nil {
			return errors.New("ChatSessionLogic.DeleteChatSession.ChatSessionStore.Delete", i18n.ERROR_INTERNAL, err)
		}

		if err := l.core.Store().ChatSessionPinStore().Delete(ctx, spaceID, sessionID); err != nil {
			return errors.New("ChatSessionLogic.DeleteChatSession.ChatSessionPinStore.Delete", i18n.ERROR_INTERNAL, err)
		}

		if err := l.core.Store().ChatMessageStore().DeleteSessionMessage(ctx, spaceID, sessionID); err != nil {
			return errors.New("ChatSessionLogic.DeleteChatSession.ChatMessageStore.DeleteSessionMessage", i18n.ERROR_INTERNAL, err)
		}

		if err := l.core.Store().ChatMessageExtStore().DeleteSessionMessageExt(ctx, spaceID, sessionID); err != nil {
			return errors.New("ChatSessionLogic.DeleteChatSession.ChatMessageExtStore.DeleteSessionMessageExt", i18n.ERROR_INTERNAL, err)
		}

		if err := l.core.Store().ChatSummaryStore().DeleteSessionSummary(ctx, sessionID); err != nil {
			return errors.New("ChatSessionLogic.DeleteChatSession.ChatSummaryStore.DeleteSessionSummary", i18n.ERROR_INTERNAL, err)
		}
		return nil
	})

	return nil
}

func (l *ChatSessionLogic) CreateChatSession(spaceID string) (string, error) {
	chatSession := types.ChatSession{
		ID:      utils.GenSpecIDStr(),
		UserID:  l.GetUserInfo().User,
		SpaceID: spaceID,
		Type:    types.CHAT_SESSION_TYPE_SINGLE,
		Status:  types.CHAT_SESSION_STATUS_UNOFFICIAL,
		Title:   fmt.Sprintf("Session At: %s", time.Now().Format("02/01 15:04:05")),
	}
	err := l.core.Store().ChatSessionStore().Create(l.ctx, chatSession)
	if err != nil {
		return "", errors.New("createDialog.ChatSessionStore.Create", i18n.ERROR_INTERNAL, err)
	}

	return chatSession.ID, nil
}

func (l *ChatSessionLogic) GetByID(spaceID, sessionID string) (*types.ChatSession, error) {
	cs, err := l.core.Store().ChatSessionStore().GetChatSession(l.ctx, spaceID, sessionID)
	if err != nil {
		return nil, errors.New("ChatSessionLogic.GetByID.ChatSessionStore.GetChatSession", i18n.ERROR_INTERNAL, err)
	}

	return cs, nil
}

func (l *ChatSessionLogic) ListUserChatSessions(page, pageSize uint64) ([]types.ChatSession, int64, error) {
	spaceID, _ := InjectSpaceID(l.ctx)
	list, err := l.core.Store().ChatSessionStore().List(l.ctx, spaceID, l.GetUserInfo().User, page, pageSize)
	if err != nil {
		return nil, 0, errors.New("ChatSessionLogic.ListUserChatSessions.ChatSessionStore.List", i18n.ERROR_INTERNAL, err)
	}

	total, err := l.core.Store().ChatSessionStore().Total(l.ctx, spaceID, l.GetUserInfo().User)
	if err != nil {
		return nil, 0, errors.New("ChatSessionLogic.ListUserChatSessions.ChatSessionStore.Total", i18n.ERROR_INTERNAL, err)
	}
	return list, total, nil
}

type NamedSessionResult struct {
	SessionID string `json:"session_id"`
	Name      string `json:"name"`
}

func (l *ChatSessionLogic) NamedSession(sessionID, firstQuery string) (NamedSessionResult, error) {
	impl := l.core.Srv().AI().GetChatAI(false)
	// raw, _ := json.Marshal(impl.Config())
	// fmt.Println("named session", string(raw))
	resp, err := impl.Generate(l.ctx, []*schema.Message{
		schema.SystemMessage(ai.PROMPT_NAMED_SESSION_DEFAULT_CN),
		schema.UserMessage(firstQuery),
	})

	if err != nil {
		return NamedSessionResult{}, errors.New("ChatSessionLogic.NamedSession.ai.Query", i18n.ERROR_INTERNAL, err)
	}

	spaceID, _ := InjectSpaceID(l.ctx)
	process.NewRecordSessionUsageRequest(impl.Config().ModelName, types.USAGE_SUB_TYPE_NAMED_CHAT, spaceID, sessionID, &openai.Usage{
		TotalTokens:      resp.ResponseMeta.Usage.TotalTokens,
		PromptTokens:     resp.ResponseMeta.Usage.PromptTokens,
		CompletionTokens: resp.ResponseMeta.Usage.CompletionTokens,
	})

	var titleBuilder strings.Builder
	titleBuilder.WriteString(resp.Content)
	title := titleBuilder.String()
	if len([]rune(title)) > 30 {
		title = string([]rune(title)[:30])
	}

	if err = l.core.Store().ChatSessionStore().UpdateSessionTitle(l.ctx, sessionID, title); err != nil {
		return NamedSessionResult{}, errors.New("ChatSessionLogic.NamedSession.ChatSessionStore.UpdateSessionTitle", i18n.ERROR_INTERNAL, err)
	}

	// l.core.Srv().Tower().PublishSessionReName(protocol.GenIMTopic(sessionID), sessionID, title)
	return NamedSessionResult{
		SessionID: sessionID,
		Name:      title,
	}, nil
}
