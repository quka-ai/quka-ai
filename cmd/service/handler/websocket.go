package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
)

func Websocket(core *core.Core) func(c *gin.Context) {
	// 获取Centrifuge管理器
	centrifugeManager := core.Srv().Centrifuge()
	if centrifugeManager == nil {
		return func(c *gin.Context) {
			response.APIError(c, errors.New("api.Websocket", "this server not support websocket service", nil))
		}
	}

	return func(c *gin.Context) {
		// 认证已经在中间件中完成，这里直接处理WebSocket连接
		// 使用Centrifuge处理WebSocket连接
		if err := centrifugeManager.HandleWebSocket(c.Writer, c.Request); err != nil {
			response.APIError(c, errors.New("api.Websocket", "failed to handle websocket connection", err))
		}
	}
}
