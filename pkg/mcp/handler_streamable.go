package mcp

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/mcp/auth"
)

// MCPStreamableHandler 创建基于 MCP SDK StreamableHTTPHandler 的处理器
// 这是推荐的实现方式，完全符合 MCP 规范
func MCPStreamableHandler(appCore *core.Core) gin.HandlerFunc {
	// 创建 MCP Server（只创建一次，所有会话共享）
	mcpServer := NewMCPServer(appCore)

	// 创建 StreamableHTTPHandler
	streamableHandler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			// 每个请求都返回同一个 server 实例
			// SDK 会自动管理会话
			return mcpServer.server
		},
		&mcp.StreamableHTTPOptions{
			// 使用 JSON 响应格式（而不是 SSE）
			// 适合 Claude Code CLI 这样的客户端
			JSONResponse: true,

			// Stateless: false 表示保持会话状态
			// SDK 会自动处理 session ID
			Stateless: false,
		},
	)

	slog.Info("MCP Streamable Handler initialized")

	// 返回 Gin Handler，在调用 SDK handler 之前进行认证
	return func(c *gin.Context) {
		slog.Info("MCP streamable request received",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"session_id", c.Request.Header.Get("Mcp-Session-Id"),
		)

		// 1. 认证检查
		userCtx, err := auth.ValidateRequest(c, appCore)
		if err != nil {
			slog.Error("MCP auth failed", "error", err.Error())
			c.JSON(http.StatusUnauthorized, gin.H{
				"jsonrpc": "2.0",
				"error": map[string]interface{}{
					"code":    -32000,
					"message": "Authentication failed: " + err.Error(),
				},
				"id": nil,
			})
			return
		}

		slog.Info("MCP auth success",
			"user_id", userCtx.UserID,
			"space_id", userCtx.SpaceID,
			"resource", userCtx.Resource,
		)

		// 2. 将用户上下文注入到 Request Context
		// 这样工具处理器可以从 context 中获取用户信息
		ctx := auth.SetUserContext(c, userCtx)
		c.Request = c.Request.WithContext(ctx)

		// 3. 调用 SDK 的 StreamableHTTPHandler
		// SDK 会自动处理：
		// - JSON-RPC 消息解析
		// - 会话管理
		// - 方法路由（initialize, tools/list, tools/call 等）
		// - 响应格式化
		streamableHandler.ServeHTTP(c.Writer, c.Request)

		slog.Info("MCP request completed")
	}
}
