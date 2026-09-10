package centrifuge

import (
	"context"
	"testing"
	"time"

	"github.com/centrifugal/centrifuge"

	"github.com/quka-ai/quka-ai/app/store"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func TestOnConnectingUsesDefaultAppIDWhenHeadersAndDataAreEmpty(t *testing.T) {
	accessTokenStore := &fakeAccessTokenStore{
		token: &types.AccessToken{
			Appid:     "default",
			UserID:    "user-1",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		},
	}
	userStore := &fakeUserStore{
		user: &types.User{
			ID:    "user-1",
			Appid: "default",
		},
	}
	authHandler := NewSimpleJWTAuthHandler(&fakeAuthor{
		accessTokenStore: accessTokenStore,
		userStore:        userStore,
	})

	reply, err := authHandler.OnConnecting(context.Background(), centrifuge.ConnectEvent{
		Token:   "access-token",
		Headers: nil,
		Data:    nil,
	})
	if err != nil {
		t.Fatalf("OnConnecting returned error: %v", err)
	}

	if accessTokenStore.appid != "default" {
		t.Fatalf("expected default appid lookup, got %q", accessTokenStore.appid)
	}
	if accessTokenStore.tokenValue != "access-token" {
		t.Fatalf("expected access token lookup, got %q", accessTokenStore.tokenValue)
	}
	if reply.Credentials == nil || reply.Credentials.UserID != "user-1" {
		t.Fatalf("unexpected credentials: %#v", reply.Credentials)
	}
}

func TestOnConnectingReadsAppIDFromDataWhenHeadersAreEmpty(t *testing.T) {
	accessTokenStore := &fakeAccessTokenStore{
		token: &types.AccessToken{
			Appid:     "custom-app",
			UserID:    "user-2",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		},
	}
	userStore := &fakeUserStore{
		user: &types.User{
			ID:    "user-2",
			Appid: "custom-app",
		},
	}
	authHandler := NewSimpleJWTAuthHandler(&fakeAuthor{
		accessTokenStore: accessTokenStore,
		userStore:        userStore,
	})

	reply, err := authHandler.OnConnecting(context.Background(), centrifuge.ConnectEvent{
		Token: "access-token",
		Data:  []byte(`{"x-appid":"custom-app","x-auth-type":"access"}`),
	})
	if err != nil {
		t.Fatalf("OnConnecting returned error: %v", err)
	}

	if accessTokenStore.appid != "custom-app" {
		t.Fatalf("expected custom appid lookup, got %q", accessTokenStore.appid)
	}
	if reply.Credentials == nil || reply.Credentials.UserID != "user-2" {
		t.Fatalf("unexpected credentials: %#v", reply.Credentials)
	}
}

type fakeAuthor struct {
	accessTokenStore store.AccessTokenStore
	userStore        store.UserStore
}

func (f *fakeAuthor) AccessTokenStore() store.AccessTokenStore {
	return f.accessTokenStore
}

func (f *fakeAuthor) UserStore() store.UserStore {
	return f.userStore
}

func (f *fakeAuthor) UserSpaceStore() store.UserSpaceStore {
	return nil
}

func (f *fakeAuthor) ChatSessionStore() store.ChatSessionStore {
	return nil
}

func (f *fakeAuthor) Cache() types.Cache {
	return nil
}

type fakeAccessTokenStore struct {
	token      *types.AccessToken
	appid      string
	tokenValue string
}

func (f *fakeAccessTokenStore) GetTable(...interface{}) string {
	return ""
}

func (f *fakeAccessTokenStore) Create(context.Context, types.AccessToken) error {
	return nil
}

func (f *fakeAccessTokenStore) GetAccessToken(_ context.Context, appid, token string) (*types.AccessToken, error) {
	f.appid = appid
	f.tokenValue = token
	return f.token, nil
}

func (f *fakeAccessTokenStore) Delete(context.Context, string, string, int64) error {
	return nil
}

func (f *fakeAccessTokenStore) Deletes(context.Context, string, string, []int64) error {
	return nil
}

func (f *fakeAccessTokenStore) ListAccessTokens(context.Context, string, string, uint64, uint64) ([]types.AccessToken, error) {
	return nil, nil
}

func (f *fakeAccessTokenStore) ClearUserTokens(context.Context, string, string) error {
	return nil
}

func (f *fakeAccessTokenStore) Total(context.Context, string, string) (int64, error) {
	return 0, nil
}

type fakeUserStore struct {
	user *types.User
}

func (f *fakeUserStore) GetTable(...interface{}) string {
	return ""
}

func (f *fakeUserStore) Create(context.Context, types.User) error {
	return nil
}

func (f *fakeUserStore) GetUser(context.Context, string, string) (*types.User, error) {
	return f.user, nil
}

func (f *fakeUserStore) GetByEmail(context.Context, string, string) (*types.User, error) {
	return nil, nil
}

func (f *fakeUserStore) UpdateUserProfile(context.Context, string, string, string, string, string) error {
	return nil
}

func (f *fakeUserStore) UpdateUserPassword(context.Context, string, string, string, string) error {
	return nil
}

func (f *fakeUserStore) Delete(context.Context, string, string) error {
	return nil
}

func (f *fakeUserStore) ListUsers(context.Context, types.ListUserOptions, uint64, uint64) ([]types.User, error) {
	return nil, nil
}

func (f *fakeUserStore) Total(context.Context, types.ListUserOptions) (int64, error) {
	return 0, nil
}

func (f *fakeUserStore) ListUsersWithGlobalRole(context.Context, types.ListUserOptions, string, uint64, uint64) ([]types.UserWithRole, error) {
	return nil, nil
}

func (f *fakeUserStore) TotalWithGlobalRole(context.Context, types.ListUserOptions, string) (int64, error) {
	return 0, nil
}

func (f *fakeUserStore) UpdateUserPlan(context.Context, string, string, string) error {
	return nil
}

func (f *fakeUserStore) BatchUpdateUserPlan(context.Context, string, []string, string) error {
	return nil
}
