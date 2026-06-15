package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
)

func (s *HttpSrv) LLMGatewayChatCompletions(c *gin.Context) {
	err := v1.NewLLMGatewayLogic(c, s.Core).ProxyChatCompletions(c.Writer, c.Request)
	if err != nil {
		writeLLMGatewayError(c, err)
		return
	}
	c.Abort()
}

func (s *HttpSrv) LLMGatewayModels(c *gin.Context) {
	models, err := v1.NewLLMGatewayLogic(c, s.Core).ListModels()
	if err != nil {
		writeLLMGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, models)
	c.Abort()
}

func writeLLMGatewayError(c *gin.Context, err error) {
	var gatewayErr *v1.LLMGatewayError
	if errors.As(err, &gatewayErr) {
		c.AbortWithStatusJSON(gatewayErr.Status, gin.H{
			"error": gin.H{
				"message": gatewayErr.Message,
				"type":    "invalid_request_error",
				"code":    gatewayErr.Code,
			},
		})
		return
	}

	response.APIError(c, err)
}
