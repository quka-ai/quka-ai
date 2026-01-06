package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/quka-ai/quka-ai/app/core"
)

// HttpSrv HTTP服务结构
type HttpSrv struct {
	Core   *core.Core
	Engine *gin.Engine
}