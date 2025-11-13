package centrifuge

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/centrifugal/centrifuge"

	"github.com/quka-ai/quka-ai/app/store"
	"github.com/quka-ai/quka-ai/pkg/auth"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/types/protocol"
)

type Author interface {
	AccessTokenStore() store.AccessTokenStore
	UserStore() store.UserStore
	UserSpaceStore() store.UserSpaceStore
	ChatSessionStore() store.ChatSessionStore
	Cache() types.Cache
}

// SimpleAuthHandler 临时简化的JWT认证处理器
// 注意：这是临时方案，用于解决当前认证问题
type SimpleAuthHandler struct {
	Store Author
}

// NewSimpleJWTAuthHandler 创建简化的JWT认证处理器
func NewSimpleJWTAuthHandler(store Author) *SimpleAuthHandler {
	return &SimpleAuthHandler{
		Store: store,
	}
}

// OnConnecting 处理连接认证 - 支持 auth token 和 access token 验证
func (a *SimpleAuthHandler) OnConnecting(ctx context.Context, event centrifuge.ConnectEvent) (centrifuge.ConnectReply, error) {
	slog.Info("WebSocket connection attempt",
		slog.String("token", event.Token),
		slog.Any("headers", event.Headers))

	// 首先尝试从缓存中验证 auth token
	if event.Headers["x-auth-type"] == "authorization" {
		tokenMeta, err := auth.ValidateTokenFromCache(ctx, event.Token, a.Store.Cache())
		if err != nil {
			slog.Debug("Auth token validation failed", slog.String("error", err.Error()))
			return centrifuge.ConnectReply{}, err
		}

		// auth token 验证成功，获取用户信息
		user, err := a.Store.UserStore().GetUser(ctx, tokenMeta.Appid, tokenMeta.UserID)
		if err != nil {
			slog.Error("Failed to get user from auth token", slog.String("error", err.Error()))
			return centrifuge.ConnectReply{}, err
		}

		return centrifuge.ConnectReply{
			Credentials: &centrifuge.Credentials{
				UserID: user.ID,
				Info:   []byte(`{"user_id":"` + user.ID + `"}`),
			},
		}, nil
	}

	// 如果 auth token 验证失败，尝试 access token
	appid := event.Headers["x-appid"]
	if appid == "" {
		appid = "default" // 使用默认 appid
	}

	token, err := a.Store.AccessTokenStore().GetAccessToken(ctx, appid, event.Token)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("Failed to get access token", slog.String("error", err.Error()))
		return centrifuge.ConnectReply{}, err
	}

	if token == nil || token.ExpiresAt < time.Now().Unix() {
		slog.Warn("Access token is nil or expired")
		return centrifuge.ConnectReply{}, errors.New("access token is invalid or expired")
	}

	user, err := a.Store.UserStore().GetUser(ctx, token.Appid, token.UserID)
	if err != nil {
		slog.Error("Failed to get user from access token", slog.String("error", err.Error()))
		return centrifuge.ConnectReply{}, err
	}

	return centrifuge.ConnectReply{
		Credentials: &centrifuge.Credentials{
			UserID: user.ID,
			Info:   []byte(`{"user_id":"` + user.ID + `"}`),
		},
	}, nil
}

// OnSubscribe 处理频道订阅权限验证 - 临时允许所有订阅
func (a *SimpleAuthHandler) OnSubscribe(ctx context.Context, client *centrifuge.Client, event centrifuge.SubscribeEvent) (centrifuge.SubscribeReply, error) {
	userID := client.UserID()
	channel := event.Channel

	slog.Debug("user subscribing to channel",
		slog.String("user_id", userID),
		slog.String("channel", channel))

	switch true {
	case strings.Contains(channel, protocol.UserTopicPrefix):
		userID := strings.TrimPrefix(channel, protocol.UserTopicPrefix)
		if userID != client.UserID() {
			return centrifuge.SubscribeReply{}, centrifuge.ErrorPermissionDenied
		}
	case strings.Contains(channel, protocol.KnowledgeListTopicPrefix):
		spaceID := strings.TrimPrefix(channel, protocol.KnowledgeListTopicPrefix)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		_, err := a.Store.UserSpaceStore().GetUserSpaceRole(ctx, userID, spaceID)
		if err != nil && err != sql.ErrNoRows {
			return centrifuge.SubscribeReply{}, err
		}
		if err == sql.ErrNoRows {
			return centrifuge.SubscribeReply{}, centrifuge.ErrorPermissionDenied
		}
	case strings.Contains(channel, protocol.ChatSessionIMTopicPrefix):
		sessionInfo := strings.Split(channel, "/")
		if len(sessionInfo) < 4 {
			return centrifuge.SubscribeReply{}, centrifuge.ErrorPermissionDenied
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		session, err := a.Store.ChatSessionStore().GetChatSession(ctx, sessionInfo[2], sessionInfo[3])
		if err != nil && err != sql.ErrNoRows {
			return centrifuge.SubscribeReply{}, err
		}
		if session == nil || session.UserID != userID {
			return centrifuge.SubscribeReply{}, centrifuge.ErrorPermissionDenied
		}
	default:
		slog.Warn("Unknown channel prefix, please add permission check", slog.String("channel", channel))
	}

	// 临时方案：允许所有订阅
	// TODO: 实现真正的权限验证
	return centrifuge.SubscribeReply{}, nil
}
