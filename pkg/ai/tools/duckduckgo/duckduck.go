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
	}

	return duckduckgo.NewTextSearchTool(ctx, config)
}
