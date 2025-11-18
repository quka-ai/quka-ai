package duckduckgo

import (
	"context"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
	"github.com/cloudwego/eino/components/tool"
)

func NewTool(ctx context.Context, region duckduckgo.Region) (tool.InvokableTool, error) {
	config := &duckduckgo.Config{
		MaxResults: 3, // Limit to return 3 results
		Region:     region,
		ToolName:   "WebSearch",
		ToolDesc:   "搜索互联网公开信息。当用户询问一般性知识、最新资讯、公开数据，或个人知识库(或记忆)查询无结果时使用此工具。",
	}

	return duckduckgo.NewTextSearchTool(ctx, config)
}
