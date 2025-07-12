package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v9"
	"github.com/samber/lo"

	"github.com/quka-ai/quka-ai/app/core"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/security"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

func I18n() gin.HandlerFunc {
	var allowList []string
	for k := range i18n.ALLOW_LANG {
		allowList = append(allowList, k)
	}
	l := i18n.NewLocalizer(allowList...)

	return response.ProvideResponseLocalizer(l)
}

// AcceptLanguage 目前服务端支持 en: English, zh-CN: 简体中文
func AcceptLanguage() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		lang := ctx.Request.Header.Get("Accept-Language")
		if lang == "" {
			ctx.Set(v1.LANGUAGE_KEY, types.LANGUAGE_EN_KEY)
			return
		}

		res := utils.ParseAcceptLanguage(lang)
		if len(res) == 0 {
			ctx.Set(v1.LANGUAGE_KEY, types.LANGUAGE_EN_KEY)
			return
		}

		ctx.Set(v1.LANGUAGE_KEY, lo.If(strings.Contains(res[0].Tag, "zh"), types.LANGUAGE_CN_KEY).Else(types.LANGUAGE_EN_KEY))
	}
}

const (
	ACCESS_TOKEN_HEADER_KEY = "X-Access-Token"
	AUTH_TOKEN_HEADER_KEY   = "X-Authorization"
	APPID_HEADER            = "X-Appid"
)

func AuthorizationFromQuery(core *core.Core) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenValue := c.Query("token")
		tokenType := c.Query("token-type")

		var (
			passed bool
			err    error
		)

		if tokenType == "authorization" {
			passed, err = ParseAuthToken(c, tokenValue, core)
		} else {
			passed, err = ParseAccessToken(c, tokenValue, core)
		}

		if err != nil {
			response.APIError(c, err)
			return
		}

		if !passed {
			response.APIError(c, errors.New("middleware.AuthorizationFromQuery", i18n.ERROR_UNAUTHORIZED, nil).Code(http.StatusUnauthorized))
			return
		}
	}
}

func Authorization(core *core.Core) gin.HandlerFunc {
	tracePrefix := "middleware.TryGetAccessToken"
	return func(ctx *gin.Context) {
		matched, err := checkAccessToken(ctx, core)
		if err != nil {
			response.APIError(ctx, errors.Trace(tracePrefix, err))
			return
		}

		if matched {
			return
		}

		if matched, err = checkAuthToken(ctx, core); err != nil {
			response.APIError(ctx, errors.Trace(tracePrefix, err))
			return
		}

		if !matched {
			response.APIError(ctx, errors.New(tracePrefix, i18n.ERROR_UNAUTHORIZED, err).Code(http.StatusUnauthorized))
		}
	}
}

func SetAppid(core *core.Core) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// appid := ctx.Request.Header.Get(APPID_HEADER)
		// check appid exist
		ctx.Set(v1.APPID_KEY, core.DefaultAppid())
	}
}

func checkAccessToken(c *gin.Context, core *core.Core) (bool, error) {
	tokenValue := c.GetHeader(ACCESS_TOKEN_HEADER_KEY)
	if tokenValue == "" {
		// try get
		// errors.New("checkAccessToken.GetHeader.ACCESS_TOKEN_HEADER_KEY.nil", i18n.ERROR_UNAUTHORIZED, nil).Code(http.StatusUnauthorized)
		return false, nil
	}

	return ParseAccessToken(c, tokenValue, core)
}

func ParseAccessToken(c *gin.Context, tokenValue string, core *core.Core) (bool, error) {
	if tokenValue == "" {
		return false, nil
	}

	appid, exist := v1.InjectAppid(c)
	if !exist {
		appid = core.DefaultAppid()
	}

	token, err := core.Store().AccessTokenStore().GetAccessToken(c, appid, tokenValue)
	if err != nil && err != sql.ErrNoRows {
		return false, errors.New("ParseAccessToken.AccessTokenStore.GetAccessToken", i18n.ERROR_INTERNAL, err)
	}

	if token == nil || token.ExpiresAt < time.Now().Unix() {
		return false, errors.New("ParseAccessToken.token.check", i18n.ERROR_UNAUTHORIZED, fmt.Errorf("nil token")).Code(http.StatusUnauthorized)
	}

	user, err := core.Store().UserStore().GetUser(c, token.Appid, token.UserID)
	if err != nil {
		return false, errors.New("ParseAccessToken.UserStore.GetUser", i18n.ERROR_INTERNAL, err)
	}

	c.Set(v1.TOKEN_CONTEXT_KEY, security.NewTokenClaims(user.Appid, "brew", user.ID, user.PlanID, "", token.ExpiresAt))
	return true, nil
}

func checkAuthToken(c *gin.Context, core *core.Core) (bool, error) {
	tokenValue := c.GetHeader(AUTH_TOKEN_HEADER_KEY)
	if tokenValue == "" {
		// try get
		// errors.New("checkAuthToken.GetHeader.AUTH_TOKEN_HEADER_KEY.nil", i18n.ERROR_UNAUTHORIZED, nil).Code(http.StatusUnauthorized)
		return false, nil
	}

	return ParseAuthToken(c, tokenValue, core)
}

func FlexibleAuth(core *core.Core) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 尝试 Header 认证 (X-Access-Token)
		matched, err := checkAccessToken(c, core)
		if err != nil {
			response.APIError(c, errors.Trace("middleware.FlexibleAuth.checkAccessToken", err))
			return
		}

		if matched {
			return
		}

		// 2. 尝试 Header 认证 (X-Authorization)
		matched, err = checkAuthToken(c, core)
		if err != nil {
			response.APIError(c, errors.Trace("middleware.FlexibleAuth.checkAuthToken", err))
			return
		}

		if matched {
			return
		}

		// 3. 尝试 Cookie 认证 (quka-auth)
		if cookieToken, err := c.Cookie("quka-auth"); err == nil && cookieToken != "" {
			passed, authErr := ParseAuthToken(c, cookieToken, core)
			if authErr != nil {
				response.APIError(c, errors.Trace("middleware.FlexibleAuth.ParseCookieToken", authErr))
				return
			}

			if passed {
				return
			}
		}

		// 4. 尝试查询参数认证
		tokenValue := c.Query("token")
		tokenType := c.Query("token-type")

		if tokenValue != "" {
			var passed bool
			var authErr error

			if tokenType == "authorization" {
				passed, authErr = ParseAuthToken(c, tokenValue, core)
			} else {
				passed, authErr = ParseAccessToken(c, tokenValue, core)
			}

			if authErr != nil {
				response.APIError(c, errors.Trace("middleware.FlexibleAuth.ParseQueryToken", authErr))
				return
			}

			if passed {
				return
			}
		}

		response.APIError(c, errors.New("middleware.FlexibleAuth", i18n.ERROR_UNAUTHORIZED, nil).Code(http.StatusUnauthorized))
	}
}

func PaymentRequired(c *gin.Context) {
	tokenClaim, exist := c.Get(v1.TOKEN_CONTEXT_KEY)
	if !exist {
		response.APIError(c, errors.New("middleware.PaymentRequired.GetToken", i18n.ERROR_UNAUTHORIZED, nil).Code(http.StatusUnauthorized))
		return
	}

	tc, ok := tokenClaim.(security.TokenClaims)
	if !ok {
		response.APIError(c, errors.New("middleware.PaymentRequired.TokenClaims", i18n.ERROR_UNAUTHORIZED, nil).Code(http.StatusUnauthorized))
		return
	}

	if tc.PlanID() == "" {
		response.APIError(c, errors.New("middleware.PaymentRequired.Check.Plan", i18n.ERROR_PAYMENT_REQUIRED, nil).Code(http.StatusPaymentRequired))
		return
	}
}

func ParseAuthToken(c *gin.Context, tokenValue string, core *core.Core) (bool, error) {
	if tokenValue == "" {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	tokenMetaStr, err := core.Plugins.Cache().Get(ctx, fmt.Sprintf("user:token:%s", utils.MD5(tokenValue)))
	if err != nil && err != redis.Nil {
		return false, errors.New("ParseAuthToken.GetFromCache", i18n.ERROR_INTERNAL, err)
	}

	if tokenMetaStr == "" {
		return false, errors.New("ParseAuthToken.tokenMetaStr.check", i18n.ERROR_UNAUTHORIZED, fmt.Errorf("nil token")).Code(http.StatusUnauthorized)
	}

	var tokenMeta types.UserTokenMeta
	if err := json.Unmarshal([]byte(tokenMetaStr), &tokenMeta); err != nil {
		return false, errors.New("ParseAuthToken.GetFromCache.json.Unmarshal", i18n.ERROR_INTERNAL, err).Code(http.StatusUnauthorized)
	}

	user, err := core.Store().UserStore().GetUser(ctx, tokenMeta.Appid, tokenMeta.UserID)
	if err != nil {
		return false, errors.New("ParseAuthToken.UserStore.GetUser", i18n.ERROR_INTERNAL, err)
	}

	c.Set(v1.TOKEN_CONTEXT_KEY, security.NewTokenClaims(tokenMeta.Appid, "brew", tokenMeta.UserID, user.PlanID, "", tokenMeta.ExpireAt))
	core.Plugins.Cache().Expire(ctx, fmt.Sprintf("user:token:%s", utils.MD5(tokenValue)), time.Hour*24*7)

	return true, nil
}

func VerifySpaceIDPermission(core *core.Core, permission string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		spaceID, _ := ctx.Params.Get("spaceid")

		claims, _ := v1.InjectTokenClaim(ctx)

		result, err := core.Store().UserSpaceStore().GetUserSpaceRole(ctx, claims.User, spaceID)
		if err != nil && err != sql.ErrNoRows {
			response.APIError(ctx, errors.New("middleware.VerifySpaceIDPermission.UserSpaceStore.GetUserSpaceRole", i18n.ERROR_INTERNAL, err))
			return
		}

		if result == nil {
			response.APIError(ctx, errors.New("middleware.VerifySpaceIDPermission.UserSpaceStore.GetUserSpaceRole.nil", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden))
			return
		}

		claims.Fields["role"] = result.Role

		if !core.Srv().RBAC().CheckPermission(result.Role, permission) {
			response.APIError(ctx, errors.New("middleware.VerifySpaceIDPermission.CheckPermission", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden))
			return
		}

		ctx.Set(v1.SPACEID_CONTEXT_KEY, spaceID)
	}
}

func Cors(c *gin.Context) {
	method := c.Request.Method
	origin := c.Request.Header.Get("Origin")
	if origin != "" {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization, X-Access-Token, X-Authorization, X-Appid")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
	}
	if method == "OPTIONS" {
		c.AbortWithStatus(http.StatusNoContent)
	}
	c.Next()
}

type LimiterFunc func(key string, opts ...core.LimitOption) gin.HandlerFunc

func UseLimit(appCore *core.Core, operation string, genKeyFunc func(c *gin.Context) string, opts ...core.LimitOption) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !appCore.UseLimiter(c, genKeyFunc(c), operation, opts...).Allow() {
			response.APIError(c, errors.New("middleware.limiter", i18n.ERROR_TOO_MANY_REQUESTS, nil).Code(http.StatusTooManyRequests))
		}
	}
}
