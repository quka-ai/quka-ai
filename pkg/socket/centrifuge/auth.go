package centrifuge

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"

	"github.com/centrifugal/centrifuge"
	"github.com/quka-ai/quka-ai/app/store"
	"github.com/quka-ai/quka-ai/pkg/types/protocol"
)

type Author interface {
	AccessTokenStore() store.AccessTokenStore
	UserStore() store.UserStore
	UserSpaceStore() store.UserSpaceStore
	ChatSessionStore() store.ChatSessionStore
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

// OnConnecting 处理连接认证 - 临时允许所有连接
func (a *SimpleAuthHandler) OnConnecting(ctx context.Context, event centrifuge.ConnectEvent) (centrifuge.ConnectReply, error) {
	slog.Info("WebSocket connection attempt")

	token, err := a.Store.AccessTokenStore().GetAccessToken(ctx, event.Headers["x-appid"], event.Token)
	if err != nil && err != sql.ErrNoRows {
		return centrifuge.ConnectReply{}, err
	}

	if token == nil || token.ExpiresAt < time.Now().Unix() {
		return centrifuge.ConnectReply{}, err
	}

	user, err := a.Store.UserStore().GetUser(ctx, token.Appid, token.UserID)
	if err != nil {
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
