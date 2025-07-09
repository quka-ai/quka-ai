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

type AuthLogic struct {
	ctx  context.Context
	core *core.Core
}

// 用户登录后可以创建多个 spaceid, 用户申请的token可以设置访问范围？
// 这个token能访问全部spaceid, 或这个token只能访问某些spaceid？
// 只从ToC的角度来看，token默认就可以访问他所代表用户的全部spaceid
func NewAuthLogic(ctx context.Context, core *core.Core) *AuthLogic {
	l := &AuthLogic{
		ctx:  ctx,
		core: core,
	}

	return l
}

func (l *AuthLogic) GetAccessTokenDetail(appid, token string) (*types.AccessToken, error) {
	data, err := l.core.Store().AccessTokenStore().GetAccessToken(l.ctx, appid, token)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("AuthLogic.GetAccessTokenDetail.AccessTokenStore.GetAccessToken", i18n.ERROR_INTERNAL, err)
	}

	return data, nil
}

func (l *AuthLogic) InitAdminUser(appid string) (string, error) {
	userID := utils.GenRandomID()
	var accessToken string
	l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		err := l.core.Store().UserStore().Create(l.ctx, types.User{
			ID:     userID,
			Appid:  appid,
			Name:   "Admin",
			Avatar: "/avatar/default.png",
			PlanID: types.USER_PLAN_ULTRA,
		})
		if err != nil {
			return errors.New("AuthLogic.InitAdminUser.CreateUser", i18n.ERROR_INTERNAL, err)
		}

		// 创建默认的空间
		tokenStore := l.core.Store().AccessTokenStore()
	REGEN:
		accessToken = utils.RandomStr(100)
		exist, err := tokenStore.GetAccessToken(l.ctx, appid, accessToken)
		if err != nil && err != sql.ErrNoRows {
			return errors.New("AuthLogic.GenNewAccessToken.GetAccessToken", i18n.ERROR_INTERNAL, err)
		}

		if exist != nil {
			// TODO: limit
			goto REGEN
		}

		err = tokenStore.Create(l.ctx, types.AccessToken{
			Appid:     appid,
			UserID:    userID,
			Version:   types.DEFAULT_ACCESS_TOKEN_VERSION,
			Token:     accessToken,
			ExpiresAt: time.Now().AddDate(999, 0, 0).Unix(),
			Info:      "Admin user token",
		})

		if err != nil {
			return errors.New("AuthLogic.GenNewAccessToken.Create", i18n.ERROR_INTERNAL, err)
		}
		return nil
	})

	return accessToken, nil
}

func (l *AuthedUserLogic) CreateAccessToken(info string) (string, error) {
	total, err := l.core.Store().AccessTokenStore().Total(l.ctx, l.GetUserInfo().Appid, l.GetUserInfo().User)
	if err != nil {
		return "", errors.New("AuthedUserLogic.CreateAccessToken.AccessTokenStore.Total", i18n.ERROR_INTERNAL, err)
	}

	if total > 10 {
		return "", errors.New("AuthedUserLogic.CreateAccessToken.AccessTokenStore.limit", i18n.ERROR_MORE_TAHN_MAX, nil).Code(http.StatusForbidden)
	}

	token := utils.RandomStr(64)
	err = l.core.Store().AccessTokenStore().Create(l.ctx, types.AccessToken{
		UserID:    l.GetUserInfo().User,
		Appid:     l.GetUserInfo().Appid,
		Info:      info,
		Version:   types.DEFAULT_ACCESS_TOKEN_VERSION,
		Token:     token,
		ExpiresAt: time.Now().Local().AddDate(999, 0, 0).Unix(),
		CreatedAt: time.Now().Unix(),
	})

	if err != nil {
		return "", errors.New("AuthedUserLogic.CreateAccessTokens.AccessTokenStore.Create", i18n.ERROR_INTERNAL, err)
	}

	return token, nil
}

func (l *AuthedUserLogic) GetAccessTokens(page, pageSize uint64) ([]types.AccessToken, error) {
	list, err := l.core.Store().AccessTokenStore().ListAccessTokens(l.ctx, l.GetUserInfo().Appid, l.GetUserInfo().User, page, pageSize)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("AuthedUserLogic.GetAccessTokens.AccessTokenStore.ListAccessTokens", i18n.ERROR_INTERNAL, err)
	}
	return list, err
}

func (l *AuthedUserLogic) DelAccessTokens(ids []int64) error {
	err := l.core.Store().AccessTokenStore().Deletes(l.ctx, l.GetUserInfo().Appid, l.GetUserInfo().User, ids)
	if err != nil {
		return errors.New("AuthedUserLogic.DelAccessTokens.AccessTokenStore.Delete", i18n.ERROR_INTERNAL, err)
	}
	return err
}
