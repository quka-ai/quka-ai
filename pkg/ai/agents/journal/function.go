package journal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
)

const (
	FUNCTION_NAME_SEARCH_JOURNAL = "SearchJournal"
)

// JournalTool 基于 eino 框架的 Journal 工具
type JournalTool struct {
	core    *core.Core
	spaceID string
	userID  string
	agent   *JournalAgent
}

// NewJournalTool 创建新的 Journal 工具实例
func NewJournalTool(core *core.Core, spaceID, userID string, agent *JournalAgent) *JournalTool {
	return &JournalTool{
		core:    core,
		spaceID: spaceID,
		userID:  userID,
		agent:   agent,
	}
}

var _ tool.InvokableTool = (*JournalTool)(nil)

// Info 实现 BaseTool 接口，返回工具信息
func (j *JournalTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	// 创建参数定义
	params := map[string]*schema.ParameterInfo{
		"startDate": {
			Type:     schema.String,
			Desc:     "获取用户日记的开始日期，格式为 yyyy-mm-dd",
			Required: true,
		},
		"endDate": {
			Type:     schema.String,
			Desc:     "获取用户日记的截至日期，格式为 yyyy-mm-dd",
			Required: true,
		},
	}

	// 创建参数描述
	paramsOneOf := schema.NewParamsOneOfByParams(params)

	return &schema.ToolInfo{
		Name:        FUNCTION_NAME_SEARCH_JOURNAL,
		Desc:        "查询用户时间范围内的日记，最大查询范围为31天",
		ParamsOneOf: paramsOneOf,
	}, nil
}

// InvokableRun 实现 InvokableTool 接口，执行工具调用
func (j *JournalTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 解析输入参数
	var params struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	// 验证日期格式
	st, err := time.ParseInLocation("2006-01-02", params.StartDate, time.Local)
	if err != nil {
		return "", fmt.Errorf("invalid start date format, expected yyyy-mm-dd: %w", err)
	}

	et, err := time.ParseInLocation("2006-01-02", params.EndDate, time.Local)
	if err != nil {
		return "", fmt.Errorf("invalid end date format, expected yyyy-mm-dd: %w", err)
	}

	// 检查日期范围（最大31天）
	if et.Sub(st).Hours() > 24*31 {
		return "Failed to load user journal list, the max range is 31 days", nil
	}

	// 查询日记数据
	journals, err := j.agent.Query(j.spaceID, j.userID, params.StartDate, params.EndDate)
	if err != nil {
		return "", fmt.Errorf("failed to query journals: %w", err)
	}

	// 构建响应内容
	sb := strings.Builder{}

	if len(journals) == 0 {
		sb.WriteString("用户在这段时间内没有任何日记")
	} else {
		sb.WriteString("查询了 ")
		sb.WriteString(params.StartDate)
		sb.WriteString(" 至 ")
		sb.WriteString(params.EndDate)
		sb.WriteString(" 日期的日记信息  \n")
		sb.WriteString("以下是查询到的用户日记内容，格式为：\n------  \n{Date}  \n{Journal Content}  \n------\n")

		for _, v := range journals {
			// 解密日记内容
			content, err := j.core.DecryptData(v.Content)
			if err != nil {
				return "", fmt.Errorf("failed to decrypt journal content: %w", err)
			}

			// 将 EditorJS 格式转换为 Markdown
			md, err := editorjs.ConvertEditorJSRawToMarkdown(content)
			if err != nil {
				return "", fmt.Errorf("failed to convert journal content to markdown: %w", err)
			}

			sb.WriteString(v.Date)
			sb.WriteString("  \n")
			sb.WriteString(md)
			sb.WriteString("  \n------  \n")
		}
	}

	// 返回结果
	result := fmt.Sprintf("Tool '%s' Response:\n%s", FUNCTION_NAME_SEARCH_JOURNAL, sb.String())

	return result, nil
}

// GetJournalTools 返回所有Journal工具的列表
func GetJournalTools(core *core.Core, spaceID, userID string, agent *JournalAgent) []tool.InvokableTool {
	return []tool.InvokableTool{
		NewJournalTool(core, spaceID, userID, agent),
	}
}
