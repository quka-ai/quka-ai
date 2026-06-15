package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/quka-ai/quka-ai/app/core"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
)

func LLMGatewayAuthorization(core *core.Core) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token := bearerToken(c.GetHeader("Authorization")); token != "" {
			if ok := tryParseGatewayToken(c, token, core); ok {
				return
			}
			writeOpenAIError(c, http.StatusUnauthorized, "invalid_api_key", "Invalid LLM Gateway API key")
			return
		}

		matched, err := checkAccessToken(c, core)
		if err != nil {
			writeOpenAIError(c, http.StatusUnauthorized, "invalid_api_key", "Invalid access token")
			return
		}
		if matched {
			return
		}

		matched, err = checkAuthToken(c, core)
		if err != nil {
			writeOpenAIError(c, http.StatusUnauthorized, "invalid_api_key", "Invalid authorization token")
			return
		}
		if matched {
			return
		}

		writeOpenAIError(c, http.StatusUnauthorized, "missing_api_key", "Missing LLM Gateway API key")
	}
}

func LLMGatewayPaymentRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := v1.InjectTokenClaim(c)
		if !ok || claims.User == "" {
			writeOpenAIError(c, http.StatusUnauthorized, "invalid_api_key", "Invalid LLM Gateway API key")
			return
		}
		if claims.PlanID() == "" {
			writeOpenAIError(c, http.StatusPaymentRequired, "payment_required", "A paid plan is required to use the LLM Gateway")
			return
		}
	}
}

func bearerToken(header string) string {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func tryParseGatewayToken(c *gin.Context, token string, core *core.Core) bool {
	if ok, err := ParseAuthToken(c, token, core); err == nil && ok {
		return true
	}
	if ok, err := ParseAccessToken(c, token, core); err == nil && ok {
		return true
	}
	return false
}

func writeOpenAIError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    "invalid_request_error",
			"code":    code,
		},
	})
}
