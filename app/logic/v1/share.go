package v1

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/samber/lo"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type ManageShareLogic struct {
	ctx  context.Context
	core *core.Core
	UserInfo
}

func NewManageShareLogic(ctx context.Context, core *core.Core) *ManageShareLogic {
	l := &ManageShareLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}

	return l
}

type CreateKnowledgeShareTokenResult struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

func (l *ManageShareLogic) CreateKnowledgeShareToken(spaceID, knowledgeID, embeddingURL string) (CreateKnowledgeShareTokenResult, error) {
	res := CreateKnowledgeShareTokenResult{}

	user := l.GetUserInfo()
	if !l.core.Srv().RBAC().CheckPermission(user.GetRole(), srv.PermissionView) {
		return res, errors.New("ManageShareLogic.CreateKnowledgeShareToken.RBAC.CheckPermission", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	link, err := l.core.Store().ShareTokenStore().Get(l.ctx, types.SHARE_TYPE_KNOWLEDGE, spaceID, knowledgeID)
	if err != nil && err != sql.ErrNoRows {
		return res, errors.New("ManageShareLogic.CreateKnowledgeShareToken.ShareTokenStore.Get", i18n.ERROR_INTERNAL, err)
	}

	if link != nil {
		if link.ExpireAt < time.Now().AddDate(0, 0, -1).Unix() {
			// update link expire time
			if err = l.core.Store().ShareTokenStore().UpdateExpireTime(l.ctx, link.ID, time.Now().AddDate(0, 0, 7).Unix()); err != nil {
				slog.Error("Failed to update share link expire time", slog.String("error", err.Error()), slog.String("space_id", spaceID),
					slog.String("knowledge_id", knowledgeID))
			}
		}

		res.Token = link.Token
		res.URL = link.EmbeddingURL
		return res, nil
	}

	res.Token = utils.MD5(fmt.Sprintf("%s_%s_%d", spaceID, knowledgeID, utils.GenUniqID()))
	res.URL = strings.ReplaceAll(embeddingURL, "{token}", res.Token)

	err = l.core.Store().ShareTokenStore().Create(l.ctx, &types.ShareToken{
		Appid:        l.GetUserInfo().Appid,
		Type:         types.SHARE_TYPE_KNOWLEDGE,
		SpaceID:      spaceID,
		ObjectID:     knowledgeID,
		Token:        res.Token,
		ShareUserID:  l.GetUserInfo().User,
		EmbeddingURL: res.URL,
		ExpireAt:     time.Now().AddDate(0, 0, 7).Unix(),
		CreatedAt:    time.Now().Unix(),
	})
	if err != nil {
		return res, errors.New("ManageShareLogic.CreateKnowledgeShareToken.ShareTokenStore.Create", i18n.ERROR_INTERNAL, err)
	}

	return res, nil
}

type CreateSessionShareTokenResult struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

func (l *ManageShareLogic) CreateSessionShareToken(spaceID, sessionID, embeddingURL string) (CreateSessionShareTokenResult, error) {
	res := CreateSessionShareTokenResult{}

	user := l.GetUserInfo()
	if !l.core.Srv().RBAC().CheckPermission(user.GetRole(), srv.PermissionMember) {
		return res, errors.New("ManageShareLogic.CreateSessionShareToken.RBAC.CheckPermission", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	link, err := l.core.Store().ShareTokenStore().Get(l.ctx, types.SHARE_TYPE_SESSION, spaceID, sessionID)
	if err != nil && err != sql.ErrNoRows {
		return res, errors.New("ManageShareLogic.CreateSessionShareToken.ShareTokenStore.Get", i18n.ERROR_INTERNAL, err)
	}

	if link != nil {
		if link.ExpireAt < time.Now().AddDate(0, 0, -1).Unix() {
			// update link expire time
			if err = l.core.Store().ShareTokenStore().UpdateExpireTime(l.ctx, link.ID, time.Now().AddDate(0, 0, 7).Unix()); err != nil {
				slog.Error("Failed to update share link expire time", slog.String("error", err.Error()), slog.String("space_id", spaceID),
					slog.String("knowledge_id", sessionID))
			}
		}

		res.Token = link.Token
		res.URL = link.EmbeddingURL
		return res, nil
	}

	res.Token = utils.MD5(fmt.Sprintf("%s_%s_%d", spaceID, sessionID, utils.GenUniqID()))
	res.URL = strings.ReplaceAll(embeddingURL, "{token}", res.Token)

	err = l.core.Store().ShareTokenStore().Create(l.ctx, &types.ShareToken{
		Appid:        l.GetUserInfo().Appid,
		Type:         types.SHARE_TYPE_SESSION,
		SpaceID:      spaceID,
		ObjectID:     sessionID,
		Token:        res.Token,
		ShareUserID:  l.GetUserInfo().User,
		EmbeddingURL: res.URL,
		ExpireAt:     time.Now().AddDate(0, 0, 7).Unix(),
		CreatedAt:    time.Now().Unix(),
	})
	if err != nil {
		return res, errors.New("ManageShareLogic.CreateSessionShareToken.ShareTokenStore.Create", i18n.ERROR_INTERNAL, err)
	}

	return res, nil
}

type ShareLogic struct {
	ctx  context.Context
	core *core.Core
}

func NewShareLogic(ctx context.Context, core *core.Core) *ShareLogic {
	l := &ShareLogic{
		ctx:  ctx,
		core: core,
	}

	return l
}

type KnowledgeShareInfo struct {
	UserID       string                     `json:"user_id"`
	UserName     string                     `json:"user_name"`
	UserAvatar   string                     `json:"user_avatar"`
	KnowledgeID  string                     `json:"knowledge_id"`
	SpaceID      string                     `json:"space_id"`
	Kind         types.KnowledgeKind        `json:"kind" db:"kind"`
	Title        string                     `json:"title" db:"title"`
	Tags         pq.StringArray             `json:"tags" db:"tags"`
	Content      types.KnowledgeContent     `json:"content" db:"content"`
	ContentType  types.KnowledgeContentType `json:"content_type" db:"content_type"`
	CreatedAt    int64                      `json:"created_at"`
	EmbeddingURL string                     `json:"embedding_url"`
}

func (l *ShareLogic) GetShareLink(token string) (*types.ShareToken, error) {
	link, err := l.core.Store().ShareTokenStore().GetByToken(l.ctx, token)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ShareLogic.GetShareLink.ShareTokenStore.GetByToken", i18n.ERROR_INTERNAL, err)
	}

	if link == nil {
		return nil, errors.New("ShareLogic.GetShareLink.ShareTokenStore.GetByToken.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	return link, nil
}

func (l *ShareLogic) GetKnowledgeByShareToken(token string) (*KnowledgeShareInfo, error) {
	link, err := l.core.Store().ShareTokenStore().GetByToken(l.ctx, token)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ShareLogic.GetKnowledgeByShareToken.ShareTokenStore.GetByToken", i18n.ERROR_INTERNAL, err)
	}

	if link == nil {
		return nil, errors.New("ShareLogic.GetKnowledgeByShareToken.ShareTokenStore.GetByToken.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	knowledge, err := l.core.Store().KnowledgeStore().GetKnowledge(l.ctx, link.SpaceID, link.ObjectID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ShareLogic.GetKnowledgeByShareToken.KnowledgeStore.GetKnowledge", i18n.ERROR_INTERNAL, err)
	}

	if knowledge == nil {
		return nil, errors.New("ShareLogic.GetKnowledgeByShareToken.KnowledgeStore.GetKnowledge.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	user, err := l.core.Store().UserStore().GetUser(l.ctx, link.Appid, knowledge.UserID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ShareLogic.GetKnowledgeByShareToken.UserStore.GetUser", i18n.ERROR_INTERNAL, err)
	}

	if user == nil {
		user = &types.User{
			Name: "Null",
			ID:   "",
		}
	}

	if knowledge.Content, err = l.core.DecryptData(knowledge.Content); err != nil {
		return nil, errors.New("ShareLogic.GetKnowledgeByShareToken.DecryptData", i18n.ERROR_INTERNAL, err)
	}

	return &KnowledgeShareInfo{
		UserID:       user.ID,
		UserName:     user.Name,
		UserAvatar:   user.Avatar,
		KnowledgeID:  knowledge.ID,
		SpaceID:      knowledge.SpaceID,
		Kind:         knowledge.Kind,
		Title:        knowledge.Title,
		Tags:         knowledge.Tags,
		Content:      lo.If(knowledge.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS, knowledge.Content).Else(types.KnowledgeContent(fmt.Sprintf("\"%s\"", knowledge.Content))),
		ContentType:  knowledge.ContentType,
		CreatedAt:    knowledge.CreatedAt,
		EmbeddingURL: link.EmbeddingURL,
	}, nil
}

type SessionShareInfo struct {
	User         *types.User          `json:"user"`
	Session      *types.ChatSession   `json:"session"`
	Messages     []*types.ChatMessage `json:"messages"`
	EmbeddingURL string               `json:"embedding_url"`
}

func (l *ShareLogic) GetSessionByShareToken(token string) (*SessionShareInfo, error) {
	link, err := l.core.Store().ShareTokenStore().GetByToken(l.ctx, token)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ShareLogic.GetSessionByShareToken.ShareTokenStore.GetByToken", i18n.ERROR_INTERNAL, err)
	}

	if link == nil {
		return nil, errors.New("ShareLogic.GetSessionByShareToken.ShareTokenStore.GetByToken.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	session, err := l.core.Store().ChatSessionStore().GetChatSession(l.ctx, link.SpaceID, link.ObjectID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ShareLogic.GetSessionByShareToken.ChatSessionStore.GetChatSession", i18n.ERROR_INTERNAL, err)
	}

	if session == nil {
		return nil, errors.New("ShareLogic.GetSessionByShareToken.ChatSessionStore.GetChatSession.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	messageList, err := l.core.Store().ChatMessageStore().ListSessionMessage(l.ctx, link.SpaceID, link.ObjectID, "", types.NO_PAGING, types.NO_PAGING)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ShareLogic.GetSessionByShareToken.ChatMessageStore.ListSessionMessage", i18n.ERROR_INTERNAL, err)
	}

	if len(messageList) == 0 {
		return nil, errors.New("ShareLogic.GetSessionByShareToken.ChatMessageStore.ListSessionMessage.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	for _, v := range messageList {
		if v.IsEncrypt != types.MESSAGE_IS_ENCRYPT {
			continue
		}
		deData, err := l.core.DecryptData([]byte(v.Message))
		if err != nil {
			return nil, errors.New("ShareLogic.GetSessionByShareToken.ChatMessageStore.DecryptData", i18n.ERROR_INTERNAL, err)
		}

		v.Message = string(deData)
	}

	user, err := l.core.Store().UserStore().GetUser(l.ctx, link.Appid, link.ShareUserID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ShareLogic.GetKnowledgeByShareToken.UserStore.GetUser", i18n.ERROR_INTERNAL, err)
	}

	if user == nil {
		user = &types.User{
			Name:   "Null",
			Avatar: l.core.Cfg().Site.DefaultAvatar,
			ID:     "",
		}
	}

	return &SessionShareInfo{
		User:         user,
		Session:      session,
		Messages:     lo.Reverse(messageList),
		EmbeddingURL: link.EmbeddingURL,
	}, nil
}

func (l *ShareLogic) CopyKnowledgeByShareToken(token, toSpaceID, toResource string) error {
	reqUser, exist := InjectTokenClaim(l.ctx)
	if !exist {
		return errors.New("ShareLogic.CopyKnowledgeByShareToken.Unauthorization", i18n.ERROR_UNAUTHORIZED, nil).Code(http.StatusUnauthorized)
	}

	userSpace, err := l.core.Store().UserSpaceStore().GetUserSpaceRole(l.ctx, reqUser.User, toSpaceID)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("ShareLogic.CopyKnowledgeByShareToken.UserSpaceStore.GetUserSpaceRole", i18n.ERROR_INTERNAL, err)
	}

	if userSpace == nil {
		return errors.New("ShareLogic.CopyKnowledgeByShareToken.UserSpaceStore.GetUserSpaceRole.nil", i18n.ERROR_USER_SPACE_NOT_FOUND, nil).Code(http.StatusBadRequest)
	}

	link, err := l.core.Store().ShareTokenStore().GetByToken(l.ctx, token)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("ShareLogic.CopyKnowledgeByShareToken.ShareTokenStore.GetByToken", i18n.ERROR_INTERNAL, err)
	}

	if link == nil {
		return errors.New("ShareLogic.CopyKnowledgeByShareToken.ShareTokenStore.GetByToken.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	originKnowledge, err := l.core.Store().KnowledgeStore().GetKnowledge(l.ctx, link.SpaceID, link.ObjectID)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("ShareLogic.CopyKnowledgeByShareToken.KnowledgeStore.GetKnowledge", i18n.ERROR_INTERNAL, err)
	}

	if originKnowledge == nil {
		return errors.New("ShareLogic.CopyKnowledgeByShareToken.KnowledgeStore.GetKnowledge.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	knowledgeID := utils.MD5(originKnowledge.UserID + originKnowledge.ID)

	alreadyCopied, err := l.core.Store().KnowledgeStore().GetKnowledge(l.ctx, toSpaceID, knowledgeID)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("ShareLogic.CopyKnowledgeByShareToken.KnowledgeStore.GetKnowledge", i18n.ERROR_INTERNAL, err)
	}

	if alreadyCopied != nil {
		return errors.New("ShareLogic.CopyKnowledgeByShareToken.KnowledgeStore.GetKnowledge", i18n.ERROR_ALREADY_SAVED, nil).Code(http.StatusForbidden)
	}

	originKnowledgeVectors, err := l.core.Store().VectorStore().ListVectors(l.ctx, types.GetVectorsOptions{
		SpaceID:     originKnowledge.SpaceID,
		KnowledgeID: originKnowledge.ID,
	}, types.NO_PAGING, types.NO_PAGING)
	if err != nil {
		return errors.New("ShareLogic.CopyKnowledgeByShareToken.VectorStore.GetVector", i18n.ERROR_INTERNAL, err)
	}

	return l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		newKnowledge := *originKnowledge
		newKnowledge.ID = knowledgeID
		newKnowledge.UserID = reqUser.User
		newKnowledge.CreatedAt = time.Now().Unix()
		newKnowledge.UpdatedAt = time.Now().Unix()
		newKnowledge.SpaceID = toSpaceID
		newKnowledge.Resource = toResource
		if err = l.core.Store().KnowledgeStore().Create(ctx, newKnowledge); err != nil {
			return errors.New("ShareLogic.CopyKnowledgeByShareToken.KnowledgeStore.Create", i18n.ERROR_INTERNAL, err)
		}

		for _, originKnowledgeVector := range originKnowledgeVectors {
			newVector := originKnowledgeVector
			newVector.ID = utils.GenUniqIDStr()
			newVector.UserID = reqUser.User
			newVector.KnowledgeID = newKnowledge.ID
			newVector.SpaceID = toSpaceID
			newVector.Resource = toResource
			if err = l.core.Store().VectorStore().Create(ctx, newVector); err != nil {
				return errors.New("ShareLogic.CopyKnowledgeByShareToken.VectorStore.Create", i18n.ERROR_INTERNAL, err)
			}
		}

		return nil
	})
}

type CreateSpaceShareTokenResult struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

func (l *ManageShareLogic) CreateSpaceShareToken(spaceID, embeddingURL string) (CreateSpaceShareTokenResult, error) {
	res := CreateSpaceShareTokenResult{}

	user := l.GetUserInfo()
	if !l.core.Srv().RBAC().CheckPermission(user.GetRole(), srv.PermissionEdit) {
		return res, errors.New("ManageShareLogic.HandlerApplication.RBAC.CheckPermission", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	link, err := l.core.Store().ShareTokenStore().Get(l.ctx, types.SHARE_TYPE_SPACE_INVITE, spaceID, "")
	if err != nil && err != sql.ErrNoRows {
		return res, errors.New("ManageShareLogic.CreateSpaceShareToken.ShareTokenStore.Get", i18n.ERROR_INTERNAL, err)
	}

	if link != nil {
		if link.ExpireAt != 0 && link.ExpireAt < time.Now().AddDate(0, 0, -1).Unix() {
			// update link expire time
			if err = l.core.Store().ShareTokenStore().UpdateExpireTime(l.ctx, link.ID, time.Now().AddDate(0, 0, 7).Unix()); err != nil {
				slog.Error("Failed to update share link expire time", slog.String("error", err.Error()), slog.String("space_id", spaceID))
			}
		}

		res.Token = link.Token
		res.URL = link.EmbeddingURL
		return res, nil
	}

	res.Token = utils.MD5(fmt.Sprintf("%s_%d", spaceID, utils.GenUniqID()))
	res.URL = strings.ReplaceAll(embeddingURL, "{token}", res.Token)

	err = l.core.Store().ShareTokenStore().Create(l.ctx, &types.ShareToken{
		Appid:        l.GetUserInfo().Appid,
		Type:         types.SHARE_TYPE_SPACE_INVITE,
		SpaceID:      spaceID,
		ObjectID:     "",
		Token:        res.Token,
		ShareUserID:  l.GetUserInfo().User,
		EmbeddingURL: res.URL,
		ExpireAt:     0,
		CreatedAt:    time.Now().Unix(),
	})
	if err != nil {
		return res, errors.New("ManageShareLogic.CreateSpaceShareToken.ShareTokenStore.Create", i18n.ERROR_INTERNAL, err)
	}

	return res, nil
}

type SpaceShareInfo struct {
	Space        *types.Space `json:"space"`
	EmbeddingURL string       `json:"embedding_url"`
}

func (l *ShareLogic) GetSpaceByShareToken(token string) (*SpaceShareInfo, error) {
	link, err := l.core.Store().ShareTokenStore().GetByToken(l.ctx, token)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ManageShareLogic.GetSpaceByShareToken.ShareTokenStore.GetByToken", i18n.ERROR_INTERNAL, err)
	}

	if link == nil {
		return nil, errors.New("ManageShareLogic.GetSpaceByShareToken.ShareTokenStore.GetByToken.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	space, err := l.core.Store().SpaceStore().GetSpace(l.ctx, link.SpaceID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ManageShareLogic.GetSpaceByShareToken.SpaceStore.GetSpace", i18n.ERROR_INTERNAL, err)
	}

	if space == nil {
		return nil, errors.New("ManageShareLogic.GetSpaceByShareToken.SpaceStore.GetSpace.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	return &SpaceShareInfo{
		Space:        space,
		EmbeddingURL: link.EmbeddingURL,
	}, nil
}
