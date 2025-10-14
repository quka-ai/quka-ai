package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/rag"
	"github.com/quka-ai/quka-ai/pkg/mcp/auth"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func RegisterSearchKnowledgeTool(server *mcp.Server, core *core.Core) {
	handler := NewSearchKnowledgeHandler(core)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_knowledges",
		Description: "Search knowledges in user's knowledge base",
	}, handler.Handle)
}

// SearchKnowledgeInput 搜索知识的输入参数
type SearchKnowledgeInput struct {
	Query string `json:"query" jsonschema:"The search query or question"`
	// MaxResults int    `json:"max_results,omitempty" jsonschema:"Maximum number of results to return (default: 5, max: 20)"`
}

// SearchKnowledgeOutput 搜索知识的输出
type SearchKnowledgeOutput struct {
	Content      string   `json:"content"`
	Query        string   `json:"query"`
	Enhanced     string   `json:"enhanced,omitempty"`
	Count        int      `json:"count"`
	KnowledgeIDs []string `json:"knowledge_ids,omitempty"`
}

// SearchKnowledgeHandler 搜索知识的处理器
type SearchKnowledgeHandler struct {
	core *core.Core
}

// NewSearchKnowledgeHandler 创建新的搜索知识处理器
func NewSearchKnowledgeHandler(core *core.Core) *SearchKnowledgeHandler {
	return &SearchKnowledgeHandler{core: core}
}

// Handle 处理搜索知识请求
func (h *SearchKnowledgeHandler) Handle(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args SearchKnowledgeInput,
) (*mcp.CallToolResult, SearchKnowledgeOutput, error) {
	// 从 context 获取认证信息
	userCtx, ok := auth.GetUserContext(ctx)
	if !ok {
		return nil, SearchKnowledgeOutput{}, fmt.Errorf("user context not found")
	}

	// 验证 query
	if args.Query == "" {
		return nil, SearchKnowledgeOutput{}, fmt.Errorf("query is required")
	}

	tool := rag.NewRagTool(h.core, userCtx.SpaceID, userCtx.UserID, "", "", 0)

	result, err := tool.Handler(ctx, args.Query)
	if err != nil {
		return nil, SearchKnowledgeOutput{}, fmt.Errorf("failed to handle rag tool: %w", err)
	}

	return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: result.Result},
			},
		}, SearchKnowledgeOutput{
			Enhanced:     result.EnhanceQuery,
			Query:        args.Query,
			Content:      result.Result,
			Count:        result.Count,
			KnowledgeIDs: lo.Map(result.Knowledges, func(item *types.PassageInfo, _ int) string { return item.ID }),
		}, nil
}
