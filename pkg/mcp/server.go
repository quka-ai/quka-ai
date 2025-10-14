package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/mcp/tools"
)

// MCPServer MCP 服务器
type MCPServer struct {
	server *mcp.Server
	core   *core.Core
}

// NewMCPServer 创建新的 MCP 服务器
func NewMCPServer(core *core.Core) *MCPServer {
	// 创建 MCP 服务器
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "quka-ai-mcp",
		Title:   "Quka AI MCP Server",
		Version: "v0.1.0",
	}, nil)

	// 注册所有工具
	tools.RegisterTools(server, core)

	return &MCPServer{
		server: server,
		core:   core,
	}
}
