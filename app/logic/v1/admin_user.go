package v1

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// 管理员用户管理常量
const (
	DefaultUserPlan      = "basic"         // 默认用户方案
	SpaceChiefRole       = "chief"         // 空间管理员角色
	AdminCreatedSource   = "admin_created" // 管理员创建用户来源标识
	TokenExpiryYears     = 999             // AccessToken过期年数（永久有效）
	DefaultSpaceTitle    = "个人空间"          // 默认空间标题
	DefaultSpaceDesc     = "默认个人空间"        // 默认空间描述
	SaltLength           = 10              // 密码盐值长度
	RandomPasswordLength = 16              // 随机密码长度
	AccessTokenLength    = 32              // AccessToken长度
	AccessTokenVersion   = "v2"            // AccessToken版本
)

type AdminUserLogic struct {
	ctx  context.Context
	core *core.Core
	UserInfo
}

type CreateUserRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

type CreateUserResult struct {
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	AccessToken string `json:"access_token"`
	CreatedAt   int64  `json:"created_at"`
}

func NewAdminUserLogic(ctx context.Context, core *core.Core) *AdminUserLogic {
	return &AdminUserLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}
}

// CreateUser 管理员创建新用户
func (l *AdminUserLogic) CreateUser(req CreateUserRequest) (*CreateUserResult, error) {
	// 验证邮箱格式
	if !isValidEmail(req.Email) {
		return nil, errors.New("AdminUserLogic.CreateUser", i18n.ERROR_INVALIDARGUMENT, fmt.Errorf("invalid email format"))
	}

	// 检查邮箱是否已存在
	exists, err := l.checkEmailExists(req.Email)
	if err != nil {
		return nil, errors.New("AdminUserLogic.CreateUser", i18n.ERROR_INTERNAL, err)
	}
	if exists {
		return nil, errors.New("AdminUserLogic.CreateUser", i18n.ERROR_INVALIDARGUMENT, fmt.Errorf("email already exists"))
	}

	// 使用事务创建用户
	var result *CreateUserResult
	err = l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		// 创建用户
		userID := utils.GenSpecIDStr()
		salt := utils.RandomStr(SaltLength)
		randomPassword := utils.RandomStr(RandomPasswordLength)
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(randomPassword+salt), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		user := types.User{
			ID:        userID,
			Appid:     l.GetUserInfo().Appid,
			Name:      req.Name,
			Email:     req.Email,
			Password:  string(hashedPassword),
			Salt:      salt,
			Source:    AdminCreatedSource,
			PlanID:    DefaultUserPlan,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		}

		if err := l.core.Store().UserStore().Create(ctx, user); err != nil {
			return err
		}

		// 生成AccessToken
		token := utils.RandomStr(AccessTokenLength)
		expiresAt := time.Now().AddDate(TokenExpiryYears, 0, 0).Unix()

		accessToken := types.AccessToken{
			Appid:     l.GetUserInfo().Appid,
			UserID:    userID,
			Token:     token,
			Version:   AccessTokenVersion,
			CreatedAt: time.Now().Unix(),
			ExpiresAt: expiresAt,
		}

		if err := l.core.Store().AccessTokenStore().Create(ctx, accessToken); err != nil {
			return err
		}

		// 创建默认空间
		spaceID, err := l.createDefaultSpace(ctx, userID)
		if err != nil {
			return err
		}

		// 设置用户为空间管理员
		if err := l.core.Store().UserSpaceStore().Create(ctx, types.UserSpace{
			UserID:    userID,
			SpaceID:   spaceID,
			Role:      SpaceChiefRole,
			CreatedAt: time.Now().Unix(),
		}); err != nil {
			return err
		}

		// 创建用户全局角色记录（管理员创建的用户默认为普通用户）
		globalRole := types.UserGlobalRole{
			UserID:    userID,
			Appid:     l.GetUserInfo().Appid,
			Role:      types.GlobalRoleMember,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		}
		if err := l.core.Store().UserGlobalRoleStore().Create(ctx, globalRole); err != nil {
			return err
		}

		result = &CreateUserResult{
			UserID:      userID,
			Name:        req.Name,
			Email:       req.Email,
			AccessToken: token,
			CreatedAt:   user.CreatedAt,
		}

		return nil
	})

	if err != nil {
		return nil, errors.New("AdminUserLogic.CreateUser", i18n.ERROR_INTERNAL, err)
	}

	return result, nil
}

// 检查邮箱是否已存在
func (l *AdminUserLogic) checkEmailExists(email string) (bool, error) {
	user, err := l.core.Store().UserStore().GetByEmail(l.ctx, l.GetUserInfo().Appid, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return user != nil, nil
}

// createDefaultSpace 创建用户的默认空间
func (l *AdminUserLogic) createDefaultSpace(ctx context.Context, userID string) (string, error) {
	spaceID := utils.GenSpecIDStr()
	space := &types.Space{
		SpaceID:     spaceID,
		Title:       DefaultSpaceTitle,
		Description: DefaultSpaceDesc,
		CreatedAt:   time.Now().Unix(),
	}

	if err := l.core.Store().SpaceStore().Create(ctx, *space); err != nil {
		return "", err
	}

	return spaceID, nil
}

// GetCreatedUsers 获取管理员创建的用户列表
func (l *AdminUserLogic) GetCreatedUsers(page, pageSize uint64) ([]types.User, int64, error) {
	opts := types.ListUserOptions{
		Appid: l.GetUserInfo().Appid,
	}

	users, err := l.core.Store().UserStore().ListUsers(l.ctx, opts, page, pageSize)
	if err != nil {
		return nil, 0, errors.New("AdminUserLogic.GetCreatedUsers", i18n.ERROR_INTERNAL, err)
	}

	total, err := l.core.Store().UserStore().Total(l.ctx, opts)
	if err != nil {
		return nil, 0, errors.New("AdminUserLogic.GetCreatedUsers", i18n.ERROR_INTERNAL, err)
	}

	return users, total, nil
}

// RegenerateAccessToken 重新生成用户的AccessToken
func (l *AdminUserLogic) RegenerateAccessToken(userID string) (string, error) {
	// 检查用户是否存在
	_, err := l.core.Store().UserStore().GetUser(l.ctx, l.GetUserInfo().Appid, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("AdminUserLogic.RegenerateAccessToken", i18n.ERROR_INVALIDARGUMENT, fmt.Errorf("user not found"))
		}
		return "", errors.New("AdminUserLogic.RegenerateAccessToken", i18n.ERROR_INTERNAL, err)
	}

	// 删除旧的AccessToken
	if err := l.core.Store().AccessTokenStore().ClearUserTokens(l.ctx, l.GetUserInfo().Appid, userID); err != nil {
		return "", errors.New("AdminUserLogic.RegenerateAccessToken", i18n.ERROR_INTERNAL, err)
	}

	// 生成新的AccessToken
	token := utils.RandomStr(AccessTokenLength)
	expiresAt := time.Now().AddDate(TokenExpiryYears, 0, 0).Unix()

	accessToken := types.AccessToken{
		Appid:     l.GetUserInfo().Appid,
		UserID:    userID,
		Token:     token,
		Version:   AccessTokenVersion,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: expiresAt,
	}

	if err := l.core.Store().AccessTokenStore().Create(l.ctx, accessToken); err != nil {
		return "", errors.New("AdminUserLogic.RegenerateAccessToken", i18n.ERROR_INTERNAL, err)
	}

	return token, nil
}

// BatchCreateUsers 批量创建用户（优化版本）
func (l *AdminUserLogic) BatchCreateUsers(users []CreateUserRequest) (*BatchCreateResult, error) {
	if len(users) == 0 {
		return &BatchCreateResult{}, nil
	}

	// 预先验证所有邮箱格式
	for _, user := range users {
		if !isValidEmail(user.Email) {
			return nil, errors.New("AdminUserLogic.BatchCreateUsers", i18n.ERROR_INVALIDARGUMENT,
				fmt.Errorf("invalid email format: %s", user.Email))
		}
	}

	// 批量检查邮箱是否已存在
	emails := make([]string, len(users))
	for i, user := range users {
		emails[i] = user.Email
	}

	existingEmails, err := l.batchCheckEmailExists(emails)
	if err != nil {
		return nil, errors.New("AdminUserLogic.BatchCreateUsers", i18n.ERROR_INTERNAL, err)
	}

	var result *BatchCreateResult
	err = l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		results := make([]CreateUserResult, 0, len(users))
		errors := make([]BatchCreateError, 0)
		successCount := 0

		for _, user := range users {
			// 检查邮箱是否已存在
			if existingEmails[user.Email] {
				errors = append(errors, BatchCreateError{
					Name:   user.Name,
					Email:  user.Email,
					Error:  "email already exists",
					Status: "failed",
				})
				continue
			}

			// 创建用户
			userResult, err := l.createSingleUserInTransaction(ctx, user)
			if err != nil {
				errors = append(errors, BatchCreateError{
					Name:   user.Name,
					Email:  user.Email,
					Error:  err.Error(),
					Status: "failed",
				})
				continue
			}

			results = append(results, *userResult)
			successCount++
		}

		result = &BatchCreateResult{
			SuccessCount: successCount,
			FailedCount:  len(users) - successCount,
			Results:      results,
			Errors:       errors,
		}

		return nil
	})

	if err != nil {
		return nil, errors.New("AdminUserLogic.BatchCreateUsers", i18n.ERROR_INTERNAL, err)
	}

	return result, nil
}

// createSingleUserInTransaction 在事务中创建单个用户
func (l *AdminUserLogic) createSingleUserInTransaction(ctx context.Context, req CreateUserRequest) (*CreateUserResult, error) {
	userID := utils.GenSpecIDStr()
	salt := utils.RandomStr(SaltLength)
	randomPassword := utils.RandomStr(RandomPasswordLength)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(randomPassword+salt), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := types.User{
		ID:        userID,
		Appid:     l.GetUserInfo().Appid,
		Name:      req.Name,
		Email:     req.Email,
		Password:  string(hashedPassword),
		Salt:      salt,
		Source:    AdminCreatedSource,
		PlanID:    DefaultUserPlan,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	if err := l.core.Store().UserStore().Create(ctx, user); err != nil {
		return nil, err
	}

	// 生成AccessToken
	token := utils.RandomStr(AccessTokenLength)
	expiresAt := time.Now().AddDate(TokenExpiryYears, 0, 0).Unix()

	accessToken := types.AccessToken{
		Appid:     l.GetUserInfo().Appid,
		UserID:    userID,
		Token:     token,
		Version:   AccessTokenVersion,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: expiresAt,
	}

	if err := l.core.Store().AccessTokenStore().Create(ctx, accessToken); err != nil {
		return nil, err
	}

	// 创建默认空间
	spaceID, err := l.createDefaultSpace(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 设置用户为空间管理员
	if err := l.core.Store().UserSpaceStore().Create(ctx, types.UserSpace{
		UserID:    userID,
		SpaceID:   spaceID,
		Role:      SpaceChiefRole,
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		return nil, err
	}

	// 创建用户全局角色记录（管理员创建的用户默认为普通用户）
	globalRole := types.UserGlobalRole{
		UserID:    userID,
		Appid:     l.GetUserInfo().Appid,
		Role:      types.GlobalRoleMember,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	if err := l.core.Store().UserGlobalRoleStore().Create(ctx, globalRole); err != nil {
		return nil, err
	}

	return &CreateUserResult{
		UserID:      userID,
		Name:        req.Name,
		Email:       req.Email,
		AccessToken: token,
		CreatedAt:   user.CreatedAt,
	}, nil
}

// batchCheckEmailExists 批量检查邮箱是否存在
func (l *AdminUserLogic) batchCheckEmailExists(emails []string) (map[string]bool, error) {
	if len(emails) == 0 {
		return make(map[string]bool), nil
	}

	// 构建批量查询
	query := fmt.Sprintf("SELECT email FROM %s WHERE appid = ? AND email IN (?%s)",
		types.TABLE_USER.Name(), strings.Repeat(",?", len(emails)-1))

	args := make([]interface{}, 0, len(emails)+1)
	args = append(args, l.GetUserInfo().Appid)
	for _, email := range emails {
		args = append(args, email)
	}

	rows, err := l.core.Store().GetMaster().Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	existingEmails := make(map[string]bool)
	for _, email := range emails {
		existingEmails[email] = false
	}

	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		existingEmails[email] = true
	}

	return existingEmails, nil
}

// BatchCreateResult 批量创建结果
type BatchCreateResult struct {
	SuccessCount int                `json:"success_count"`
	FailedCount  int                `json:"failed_count"`
	Results      []CreateUserResult `json:"results"`
	Errors       []BatchCreateError `json:"errors"`
}

// BatchCreateError 批量创建错误
type BatchCreateError struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Error  string `json:"error"`
	Status string `json:"status"`
}

// isValidEmail 验证邮箱格式
func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}
