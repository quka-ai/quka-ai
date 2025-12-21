package journal

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
)

type JournalAgent struct {
	core *core.Core
}

func NewJournalAgent(core *core.Core) *JournalAgent {
	return &JournalAgent{core: core}
}

var FunctionDefine = lo.Map([]*openai.FunctionDefinition{
	{
		Name:        "SearchJournal",
		Description: "查询用户时间范围内的日记",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"startDate": {
					Type:        jsonschema.String,
					Description: "获取用户日记的开始日期，格式为 yyyy-mm-dd",
				},
				"endDate": {
					Type:        jsonschema.String,
					Description: "获取用户日记的截至日期，格式为 yyyy-mm-dd",
				},
			},
			Required: []string{"startDate", "endDate"},
		},
	},
}, func(item *openai.FunctionDefinition, _ int) openai.Tool {
	return openai.Tool{
		Function: item,
	}
})

func (b *JournalAgent) Query(spaceID, userID, startDate, endDate string) ([]types.Journal, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	journals, err := b.core.Store().JournalStore().ListWithDate(ctx, spaceID, userID, startDate, endDate)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return journals, nil
}

// ToolContext包含执行工具所需的所有上下文信息
type ToolContext struct {
	Agent   *JournalAgent
	SpaceID string
	UserID  string
}

func searchJournal(ctx ToolContext, funcCall openai.FunctionCall) ([]*types.MessageContext, error) {
	var params struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	}

	if err := json.Unmarshal([]byte(funcCall.Arguments), &params); err != nil {
		return nil, err
	}

	st, err := time.ParseInLocation("2006-01-02", params.StartDate, time.Local)
	if err != nil {
		return nil, err
	}

	et, err := time.ParseInLocation("2006-01-02", params.EndDate, time.Local)
	if err != nil {
		return nil, err
	}

	if et.Sub(st).Hours() > 24*31 {
		return []*types.MessageContext{
			{
				Role:    types.USER_ROLE_SYSTEM,
				Content: "Failed to load user journal list, the max range is 31 days",
			},
		}, nil
	}

	res, err := ctx.Agent.Query(ctx.SpaceID, ctx.UserID, params.StartDate, params.EndDate)
	if err != nil {
		return nil, err
	}

	sb := strings.Builder{}

	if len(res) == 0 {
		sb.WriteString("用户在这段时间内没有任何日记")
	} else {
		sb.WriteString("查询了 ")
		sb.WriteString(params.StartDate)
		sb.WriteString(" 至 ")
		sb.WriteString(params.EndDate)
		sb.WriteString(" 日期的日记信息  \n")
		sb.WriteString("以下是查询到的用户日记内容，格式为：\n------  \n{Date}  \n{Journal Content}  \n------\n")

		for _, v := range res {
			content, err := ctx.Agent.core.DecryptData(v.Content)
			if err != nil {
				return nil, err
			}
			md, err := editorjs.ConvertEditorJSRawToMarkdown(content)
			if err != nil {
				return nil, err
			}
			sb.WriteString(v.Date)
			sb.WriteString("  \n")
			sb.WriteString(md)
			sb.WriteString("  \n------  \n")
		}
	}

	return []*types.MessageContext{
		{
			Role:    types.USER_ROLE_SYSTEM,
			Content: sb.String(),
		},
	}, nil
}

func (b *JournalAgent) GetToolsHandler(spaceID, userID string, messages []openai.ChatCompletionMessage) map[string]ai.ToolHandlerFunc {
	return map[string]ai.ToolHandlerFunc{
		"SearchJournal": ai.WrapToolHandler(func() ToolContext {
			return ToolContext{Agent: b, SpaceID: spaceID, UserID: userID}
		}, searchJournal),
	}
}
