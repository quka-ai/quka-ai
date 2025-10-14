package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quka-ai/quka-ai/app/core"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
)

type contextKey string

const userContextKey contextKey = "mcp_user_context"

// SetUserContext 将用户上下文设置到 context 中
func SetUserContext(ctx context.Context, userCtx *UserContext) context.Context {
	return context.WithValue(ctx, userContextKey, userCtx)
}

// GetUserContext 从 context 中获取用户上下文
func GetUserContext(ctx context.Context) (*UserContext, bool) {
	userCtx, ok := ctx.Value(userContextKey).(*UserContext)
	return userCtx, ok
}

// UserContext MCP 用户上下文
type UserContext struct {
	UserID   string
	Appid    string
	SpaceID  string
	Resource string
}

// ValidateRequest 验证 MCP 请求的认证信息
// 支持三种认证方式（优先级从高到低）：
// 1. URL 参数: ?token=xxx&space_id=xxx&resource=xxx
// 2. HTTP Header: Authorization, X-Space-ID, X-Resource
// 3. 环境变量: QUKA_ACCESS_TOKEN, QUKA_SPACE_ID, QUKA_RESOURCE
func ValidateRequest(c *gin.Context, core *core.Core) (*UserContext, error) {
	var token, spaceID, resource string

	// 方式 1: 从 URL 参数提取
	token = c.Query("token")
	spaceID = c.Query("space_id")
	resource = c.Query("resource")

	// 方式 2: 从 HTTP Header 提取（如果 URL 参数为空）
	if token == "" {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			} else {
				token = parts[0]
			}
		}
	}

	if spaceID == "" {
		spaceID = c.GetHeader("X-Space-ID")
	}

	if resource == "" {
		resource = c.GetHeader("X-Resource")
	}

	// 验证必需参数
	if token == "" {
		return nil, fmt.Errorf("missing access token (provide via URL param, Authorization header, or env var)")
	}
	if spaceID == "" {
		return nil, fmt.Errorf("missing space_id (provide via URL param, X-Space-ID header, or env var)")
	}
	if resource == "" {
		resource = "knowledge" // 默认值
	}

	// 验证 token
	appid := core.DefaultAppid()
	ctx := c.Request.Context()

	authLogic := v1.NewAuthLogic(ctx, core)
	accessToken, err := authLogic.GetAccessTokenDetail(appid, token)
	if err != nil {
		return nil, fmt.Errorf("invalid access token: %w", err)
	}

	// 检查 accessToken 是否为 nil (可能是 sql.ErrNoRows 被忽略的情况)
	if accessToken == nil {
		return nil, fmt.Errorf("access token not found")
	}

	// 检查过期
	if accessToken.ExpiresAt > 0 && accessToken.ExpiresAt < time.Now().Unix() {
		return nil, fmt.Errorf("access token expired")
	}

	// TODO: 验证用户对空间的访问权限
	// userSpaceStore := core.Store.UserSpace()
	// hasAccess, err := userSpaceStore.CheckUserSpaceAccess(accessToken.UserID, spaceID)
	// if err != nil || !hasAccess {
	//     return nil, fmt.Errorf("user does not have access to this space")
	// }

	return &UserContext{
		UserID:   accessToken.UserID,
		Appid:    accessToken.Appid,
		SpaceID:  spaceID,
		Resource: resource,
	}, nil
}
