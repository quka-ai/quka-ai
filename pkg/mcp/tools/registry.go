package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/quka-ai/quka-ai/app/core"
)

// RegisterTools 注册所有 MCP 工具
func RegisterTools(server *mcp.Server, core *core.Core) {
	// 注册 create_knowledge 工具
	RegisterCreateKnowledgeTool(server, core)

	// 注册 get_knowledge 工具
	RegisterGetKnowledgeTool(server, core)

	// 注册 search_knowledge 工具
	RegisterSearchKnowledgeTool(server, core)

	// 未来可添加更多工具
	// RegisterUpdateKnowledgeTool(server, core)
	// RegisterDeleteKnowledgeTool(server, core)
}
