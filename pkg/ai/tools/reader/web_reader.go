package reader

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/quka-ai/quka-ai/app/core/srv"
)

type ReaderTool struct {
	reader srv.ReaderAI
}

func (t *ReaderTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "reader",
		Desc: "读取外部URL，如果用户或上下文中提到了外部URL，你可以使用该工具进行URL读取，在同一个上下文中请不要重复读取相同的内容",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"endpoint": {
				Desc:     "URL,Endpoint",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

type ReaderRequest struct {
	Endpoint string `json:"endpoint"`
}

func (t *ReaderTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args ReaderRequest

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", err
	}

	result, err := t.reader.Reader(ctx, args.Endpoint)
	if err != nil {
		return "", err
	}

	sb := strings.Builder{}
	sb.WriteString(result.Title)
	sb.WriteString("\n")
	sb.WriteString(result.Description)
	sb.WriteString("\n")
	sb.WriteString(result.Content)

	return sb.String(), nil
}

func NewTool(ctx context.Context, reader srv.ReaderAI) (tool.InvokableTool, error) {
	return &ReaderTool{
		reader: reader,
	}, nil
}
