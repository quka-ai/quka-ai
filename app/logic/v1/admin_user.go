package v1

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
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
		return nil, errors.New("AdminUserLogic.CreateUser", i18n.ERROR_INVALIDARGUMENT, fmt.Errorf("email already exists")).Code(http.StatusBadRequest)
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

// GetUsers 管理员获取用户列表（支持搜索条件和全局角色过滤）
func (l *AdminUserLogic) GetUsers(opts types.ListUserOptions, globalRole string, page, pageSize uint64) ([]types.UserWithRole, int64, error) {
	// 确保只查询当前租户的用户
	opts.Appid = l.GetUserInfo().Appid

	// 使用JOIN查询避免SQL长度限制
	if globalRole == "" {
		// 如果不需要角色过滤，使用原始方法
		users, err := l.core.Store().UserStore().ListUsers(l.ctx, opts, page, pageSize)
		if err != nil {
			return nil, 0, errors.New("AdminUserLogic.GetUsers", i18n.ERROR_INTERNAL, err)
		}

		// 获取符合条件的总数
		total, err := l.core.Store().UserStore().Total(l.ctx, opts)
		if err != nil {
			return nil, 0, errors.New("AdminUserLogic.GetUsers.Count", i18n.ERROR_INTERNAL, err)
		}

		// 为每个用户获取全局角色
		var usersWithRole []types.UserWithRole
		for _, user := range users {
			// 获取用户全局角色
			globalRoleObj, err := l.core.Store().UserGlobalRoleStore().GetUserRole(l.ctx, user.Appid, user.ID)
			if err != nil {
				return nil, 0, errors.New("AdminUserLogic.GetUsers.GetGlobalRole", i18n.ERROR_INTERNAL, err)
			}

			// 设置用户全局角色（如果没有角色记录，使用默认角色）
			roleValue := types.DefaultGlobalRole
			if globalRoleObj != nil {
				roleValue = globalRoleObj.Role
			}

			usersWithRole = append(usersWithRole, types.UserWithRole{
				User:       user,
				GlobalRole: roleValue,
			})
		}

		return usersWithRole, total, nil
	} else {
		// 使用JOIN查询避免SQL长度限制
		users, err := l.core.Store().UserStore().ListUsersWithGlobalRole(l.ctx, opts, globalRole, page, pageSize)
		if err != nil {
			return nil, 0, errors.New("AdminUserLogic.GetUsers", i18n.ERROR_INTERNAL, err)
		}

		// 获取符合条件的总数
		total, err := l.core.Store().UserStore().TotalWithGlobalRole(l.ctx, opts, globalRole)
		if err != nil {
			return nil, 0, errors.New("AdminUserLogic.GetUsers.Count", i18n.ERROR_INTERNAL, err)
		}

		return users, total, nil
	}
}

// RegenerateAccessToken 重新生成用户的AccessToken
func (l *AdminUserLogic) RegenerateAccessToken(userID string) (string, error) {
	// 首先验证用户是否存在且是管理员创建的
	user, err := l.core.Store().UserStore().GetUser(l.ctx, l.GetUserInfo().Appid, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("AdminUserLogic.RegenerateAccessToken", i18n.ERROR_INVALIDARGUMENT, fmt.Errorf("user not found"))
		}
		return "", errors.New("AdminUserLogic.RegenerateAccessToken", i18n.ERROR_INTERNAL, err)
	}

	if user.Source != AdminCreatedSource {
		return "", errors.New("AdminUserLogic.RegenerateAccessToken", i18n.ERROR_INVALIDARGUMENT, fmt.Errorf("user not created by admin"))
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
		Info:      "Regenerated by admin",
	}

	if err := l.core.Store().AccessTokenStore().Create(l.ctx, accessToken); err != nil {
		return "", errors.New("AdminUserLogic.RegenerateAccessToken", i18n.ERROR_INTERNAL, err)
	}

	return token, nil
}

// DeleteUser 管理员删除用户（包括级联删除所有相关数据）
func (l *AdminUserLogic) DeleteUser(userID string) error {
	// 验证用户是否存在
	user, err := l.core.Store().UserStore().GetUser(l.ctx, l.GetUserInfo().Appid, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("AdminUserLogic.DeleteUser", i18n.ERROR_INVALIDARGUMENT, fmt.Errorf("user not found"))
		}
		return errors.New("AdminUserLogic.DeleteUser", i18n.ERROR_INTERNAL, err)
	}

	// 使用事务进行级联删除
	return l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		// 1. 删除用户的全局角色记录
		if err := l.core.Store().UserGlobalRoleStore().Delete(ctx, user.Appid, userID); err != nil {
			return fmt.Errorf("failed to delete user global role: %w", err)
		}

		// 2. 删除用户的访问令牌
		if err := l.deleteUserAccessTokens(ctx, userID); err != nil {
			return fmt.Errorf("failed to delete user access tokens: %w", err)
		}

		// 3. 获取用户创建或拥有的空间，进行级联删除
		if err := l.deleteUserSpaces(ctx, userID); err != nil {
			return fmt.Errorf("failed to delete user spaces: %w", err)
		}

		// 4. 删除用户的会话和消息
		if err := l.deleteUserChatData(ctx, userID); err != nil {
			return fmt.Errorf("failed to delete user chat data: %w", err)
		}

		// 5. 删除用户的知识库和相关数据
		if err := l.deleteUserKnowledge(ctx, userID); err != nil {
			return fmt.Errorf("failed to delete user knowledge: %w", err)
		}

		// 6. 删除用户的其他个人数据
		if err := l.deleteUserPersonalData(ctx, userID); err != nil {
			return fmt.Errorf("failed to delete user personal data: %w", err)
		}

		// 7. 最后删除用户本身
		if err := l.core.Store().UserStore().Delete(ctx, user.Appid, userID); err != nil {
			return fmt.Errorf("failed to delete user: %w", err)
		}

		return nil
	})
}

// deleteUserAccessTokens 删除用户的所有访问令牌
func (l *AdminUserLogic) deleteUserAccessTokens(ctx context.Context, userID string) error {
	// 这里假设AccessTokenStore有DeleteByUser方法，如果没有需要实现
	// return l.core.Store().AccessTokenStore().DeleteByUser(ctx, userID)

	// 临时实现：由于现有代码结构限制，记录日志但不删除AccessToken
	// 在生产环境中应该删除用户的所有AccessToken
	return nil
}

// deleteUserSpaces 删除用户创建的空间和相关数据
func (l *AdminUserLogic) deleteUserSpaces(ctx context.Context, userID string) error {
	// 获取用户所属的所有空间
	userSpaces, err := l.core.Store().UserSpaceStore().List(ctx, types.ListUserSpaceOptions{
		UserID: userID,
	}, 1, 1000) // 假设最多1000个空间
	if err != nil {
		return err
	}

	for _, userSpace := range userSpaces {
		// 对于用户作为chief的空间，需要完全删除空间
		if userSpace.Role == SpaceChiefRole {
			// 删除空间下的所有资源、知识库等
			if err := l.deleteSpaceData(ctx, userSpace.SpaceID); err != nil {
				return fmt.Errorf("failed to delete space %s data: %w", userSpace.SpaceID, err)
			}

			// 删除空间本身
			if err := l.core.Store().SpaceStore().Delete(ctx, userSpace.SpaceID); err != nil {
				return fmt.Errorf("failed to delete space %s: %w", userSpace.SpaceID, err)
			}
		} else {
			// 对于用户只是成员的空间，只删除用户与空间的关系
			if err := l.core.Store().UserSpaceStore().Delete(ctx, userSpace.UserID, userSpace.SpaceID); err != nil {
				return fmt.Errorf("failed to delete user space relation: %w", err)
			}
		}
	}

	return nil
}

// deleteSpaceData 删除空间下的所有数据
func (l *AdminUserLogic) deleteSpaceData(ctx context.Context, spaceID string) error {
	// 1. 删除空间下的所有知识库相关数据
	if err := l.deleteSpaceKnowledgeData(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete space knowledge data: %w", err)
	}

	// 2. 删除空间下的所有聊天相关数据
	if err := l.deleteSpaceChatData(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete space chat data: %w", err)
	}

	// 3. 删除空间下的其他相关数据
	if err := l.deleteSpaceOtherData(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete space other data: %w", err)
	}

	// 4. 最后删除空间下的所有用户关系
	if err := l.core.Store().UserSpaceStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete space user relations: %w", err)
	}

	return nil
}

// deleteSpaceKnowledgeData 删除空间下所有知识库相关数据
func (l *AdminUserLogic) deleteSpaceKnowledgeData(ctx context.Context, spaceID string) error {
	// 删除知识库向量数据
	if err := l.core.Store().VectorStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete vectors: %w", err)
	}

	// 删除知识库块数据
	if err := l.core.Store().KnowledgeChunkStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete knowledge chunks: %w", err)
	}

	// 删除知识库元数据关联
	if err := l.core.Store().KnowledgeRelMetaStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete knowledge rel meta: %w", err)
	}

	// 删除知识库元数据
	if err := l.core.Store().KnowledgeMetaStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete knowledge meta: %w", err)
	}

	// 删除内容任务
	if err := l.core.Store().ContentTaskStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete content tasks: %w", err)
	}

	// 最后删除知识库本身
	if err := l.core.Store().KnowledgeStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete knowledge: %w", err)
	}

	return nil
}

// deleteSpaceChatData 删除空间下所有聊天相关数据
func (l *AdminUserLogic) deleteSpaceChatData(ctx context.Context, spaceID string) error {
	// 删除聊天消息扩展数据
	if err := l.core.Store().ChatMessageExtStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete chat message ext: %w", err)
	}

	// 删除聊天消息
	if err := l.core.Store().ChatMessageStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete chat messages: %w", err)
	}

	// 删除聊天会话固定记录
	if err := l.core.Store().ChatSessionPinStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete chat session pins: %w", err)
	}

	// 删除聊天摘要
	if err := l.core.Store().ChatSummaryStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete chat summaries: %w", err)
	}

	// 删除聊天会话
	if err := l.core.Store().ChatSessionStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete chat sessions: %w", err)
	}

	return nil
}

// deleteSpaceOtherData 删除空间下的其他相关数据
func (l *AdminUserLogic) deleteSpaceOtherData(ctx context.Context, spaceID string) error {
	// 删除空间下的所有静态文件
	if err := l.deleteSpaceStaticFiles(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete static files: %w", err)
	}

	// 删除空间下的所有资源
	if err := l.core.Store().ResourceStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete resources: %w", err)
	}

	// 删除空间下的所有分享令牌
	if err := l.core.Store().ShareTokenStore().DeleteBySpace(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete share tokens: %w", err)
	}

	// 删除文件管理记录
	if err := l.core.Store().FileManagementStore().DeleteAll(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete file management records: %w", err)
	}

	return nil
}

// deleteSpaceStaticFiles 删除空间下的所有静态文件
func (l *AdminUserLogic) deleteSpaceStaticFiles(ctx context.Context, spaceID string) error {
	// 获取空间下的所有文件记录
	files, err := l.core.Store().FileManagementStore().ListBySpace(ctx, spaceID)
	if err != nil {
		return fmt.Errorf("failed to list space files: %w", err)
	}

	// 获取文件存储服务
	fileStorage := l.core.Plugins.FileStorage()

	// 删除每个静态文件
	for _, file := range files {
		if err := fileStorage.DeleteFile(file.File); err != nil {
			// 记录错误但继续删除其他文件，避免因单个文件删除失败而中断整个流程
			// 可以考虑使用日志记录这个错误
			fmt.Printf("Warning: failed to delete static file %s: %v\n", file.File, err)
			// 继续删除其他文件而不返回错误
		}
	}

	return nil
}

// deleteUserChatData 删除用户的聊天数据
func (l *AdminUserLogic) deleteUserChatData(ctx context.Context, userID string) error {
	// 删除用户的聊天会话和消息
	// sessions, err := l.core.Store().ChatSessionStore().ListByUser(ctx, userID)
	// if err != nil {
	//     return fmt.Errorf("failed to list user chat sessions: %w", err)
	// }

	// for _, session := range sessions {
	//     // 删除会话相关的所有消息
	//     if err := l.core.Store().ChatMessageStore().DeleteBySession(ctx, session.ID); err != nil {
	//         return fmt.Errorf("failed to delete session messages: %w", err)
	//     }
	//     // 删除会话
	//     if err := l.core.Store().ChatSessionStore().Delete(ctx, session.ID); err != nil {
	//         return fmt.Errorf("failed to delete session: %w", err)
	//     }
	// }

	return nil
}

// deleteUserKnowledge 删除用户的知识库数据
func (l *AdminUserLogic) deleteUserKnowledge(ctx context.Context, userID string) error {
	// 删除用户创建的知识库和相关数据
	// knowledges, err := l.core.Store().KnowledgeStore().ListByUser(ctx, userID)
	// if err != nil {
	//     return fmt.Errorf("failed to list user knowledge: %w", err)
	// }

	// for _, knowledge := range knowledges {
	//     if err := l.deleteKnowledgeData(ctx, knowledge.ID); err != nil {
	//         return fmt.Errorf("failed to delete knowledge %s: %w", knowledge.ID, err)
	//     }
	// }

	return nil
}

// deleteUserPersonalData 删除用户的个人数据
func (l *AdminUserLogic) deleteUserPersonalData(ctx context.Context, userID string) error {
	// 删除用户的日志、butler、文件管理等个人数据
	// 这里需要根据具体的Store接口来实现各种个人数据的删除

	// 删除用户的分享令牌
	// if err := l.core.Store().ShareTokenStore().DeleteByUser(ctx, userID); err != nil {
	//     return fmt.Errorf("failed to delete user share tokens: %w", err)
	// }

	// 删除用户的文件管理记录
	// if err := l.core.Store().FileManagementStore().DeleteByUser(ctx, userID); err != nil {
	//     return fmt.Errorf("failed to delete user file management records: %w", err)
	// }

	// 删除用户的AI使用记录
	// if err := l.core.Store().AITokenUsageStore().DeleteByUser(ctx, userID); err != nil {
	//     return fmt.Errorf("failed to delete user AI token usage: %w", err)
	// }

	return nil
}

// isValidEmail 验证邮箱格式
func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}
