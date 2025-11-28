package v1

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// logic for unlogin
type UserLogic struct {
	ctx  context.Context
	core *core.Core
}

func NewUserLogic(ctx context.Context, core *core.Core) *UserLogic {
	l := &UserLogic{
		ctx:  ctx,
		core: core,
	}

	return l
}

func (l *UserLogic) Register(appid, name, email, password, workspaceName string) (string, error) {
	salt := utils.RandomStr(10)
	userID := utils.GenUniqIDStr()

	err := l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		defaultPlan, err := l.core.Plugins.CreateUserDefaultPlan(ctx, appid, userID)
		if err != nil {
			return errors.New("UserLogic.Register.Plugins.CreateUserDefaultPlan", i18n.ERROR_INTERNAL, err)
		}

		err = l.core.Store().UserStore().Create(ctx, types.User{
			ID:        userID,
			Appid:     appid,
			Name:      name,
			Email:     email,
			Avatar:    l.core.Cfg().Site.DefaultAvatar,
			Salt:      salt,
			Source:    "platform",
			PlanID:    defaultPlan,
			Password:  utils.GenUserPassword(salt, password),
			UpdatedAt: time.Now().Unix(),
			CreatedAt: time.Now().Unix(),
		})
		if err != nil {
			return errors.New("UserLogic.Register.UserStore.Create", i18n.ERROR_INTERNAL, err)
		}

		spaceID := utils.GenRandomID()
		err = l.core.Store().SpaceStore().Create(ctx, types.Space{
			SpaceID:     spaceID,
			Title:       workspaceName,
			Description: "default space",
			CreatedAt:   time.Now().Unix(),
		})
		if err != nil {
			return errors.New("UserLogic.Register.SpaceStore.Create", i18n.ERROR_INTERNAL, err)
		}

		err = l.core.Store().UserSpaceStore().Create(ctx, types.UserSpace{
			UserID:    userID,
			SpaceID:   spaceID,
			Role:      srv.RoleChief,
			CreatedAt: time.Now().Unix(),
		})
		if err != nil {
			return errors.New("UserLogic.Register.UserSpaceStore.Create", i18n.ERROR_INTERNAL, err)
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	return userID, nil
}

func (l *UserLogic) Login(appid, email, password string) (string, error) {
	user, err := l.core.Store().UserStore().GetByEmail(l.ctx, appid, email)
	if err != nil && err != sql.ErrNoRows {
		return "", errors.New("UserLogic.Login.UserStore.GetByEmail", i18n.ERROR_INTERNAL, err)
	}

	if user == nil || user.Password != utils.GenUserPassword(user.Salt, password) {
		return "", errors.New("UserLogic.Login.Password.check", i18n.ERROR_INVALID_ACCOUNT, err).Code(http.StatusBadRequest)
	}

	accessToken := utils.MD5(user.ID + utils.GenRandomID())
	err = l.core.Store().AccessTokenStore().Create(l.ctx, types.AccessToken{
		UserID:    user.ID,
		Token:     accessToken,
		Version:   types.DEFAULT_ACCESS_TOKEN_VERSION,
		Info:      "login",
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return "", errors.New("UserLogic.Login.AccessTokenStore.Login", i18n.ERROR_INTERNAL, err)
	}

	return accessToken, nil
}

type UserBaseInfo struct {
	ID         string `json:"id" db:"id"`                 // 用户ID，主键
	Appid      string `json:"appid" db:"appid"`           // 租户id
	Name       string `json:"name" db:"name"`             // 用户名
	Avatar     string `json:"avatar" db:"avatar"`         // 用户头像URL
	Email      string `json:"email" db:"email"`           // 用户邮箱，唯一约束
	Source     string `json:"-" db:"source"`              // 用户注册来源
	PlanID     string `json:"plan_id" db:"plan_id"`       // 会员方案ID
	UpdatedAt  int64  `json:"updated_at" db:"updated_at"` // 更新时间，Unix时间戳
	CreatedAt  int64  `json:"created_at" db:"created_at"` // 创建时间，Unix时间戳
	SystemRole string `json:"system_role"`                // 用户全局角色
}

func (l *UserLogic) GetUser(appid, id string) (*UserBaseInfo, error) {
	user, err := l.core.Store().UserStore().GetUser(l.ctx, appid, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("AuthedUserLogin.GetUser.UserStore.GetUser", i18n.ERROR_INTERNAL, err)
	}

	if user == nil {
		return nil, errors.New("AuthedUserLogin.GetUser.UserStore.GetUser.nil", i18n.ERROR_INTERNAL, nil).Code(http.StatusNotFound)
	}

	// 处理存储URL，如果是本地存储的文件则生成预签名URL
	if user.Avatar, err = utils.ProcessStorageURL(user.Avatar, l.core.Plugins.FileStorage().GetStaticDomain(), l.core.Plugins.FileStorage().GenGetObjectPreSignURL); err != nil {
		return nil, errors.New("AuthedUserLogin.GetUser.FileStorage.GenGetObjectPreSignURL", i18n.ERROR_INTERNAL, err)
	}

	role, err := l.core.Store().UserGlobalRoleStore().GetUserRole(l.ctx, appid, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("AuthedUserLogin.GetUser.UserGlobalRoleStore.GetUserRole", i18n.ERROR_INTERNAL, err)
	}

	userInfo := &UserBaseInfo{
		ID:         user.ID,
		Appid:      user.Appid,
		Name:       user.Name,
		Avatar:     user.Avatar,
		Email:      user.Email,
		Source:     user.Source,
		PlanID:     user.PlanID,
		UpdatedAt:  user.UpdatedAt,
		CreatedAt:  user.CreatedAt,
		SystemRole: types.GlobalRoleMember,
	}

	if role != nil {
		userInfo.SystemRole = role.Role
	}

	return userInfo, nil
}

type AuthedUserLogic struct {
	UserInfo
	ctx  context.Context
	core *core.Core
}

func NewAuthedUserLogic(ctx context.Context, core *core.Core) *AuthedUserLogic {
	l := &AuthedUserLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}

	return l
}

func (l *AuthedUserLogic) UpdateUserProfile(userName, email, avatar string) error {
	// 检测avatar的host是否为对象存储的静态host，如果是则去除host只保留路径
	if avatar != "" {
		staticDomain := l.core.Plugins.FileStorage().GetStaticDomain()
		if staticDomain != "" && strings.HasPrefix(avatar, staticDomain) {
			// 去除静态host，只保留路径部分
			avatar = strings.TrimPrefix(avatar, staticDomain)
			// 确保路径以/开头
			if !strings.HasPrefix(avatar, "/") {
				avatar = "/" + avatar
			}
		}
	}

	err := l.core.Store().UserStore().UpdateUserProfile(l.ctx, l.GetUserInfo().Appid, l.GetUserInfo().User, userName, email, avatar)
	if err != nil {
		return errors.New("AuthedUserLogic.UpdateUserProfile.UserStore.UpdateUserProfile", i18n.ERROR_INTERNAL, err)
	}
	return nil
}
