package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/types"
)

const (
	FUNCTION_NAME_SEARCH_USER_KNOWLEDGES = "SearchUserKnowledges"
)

var FunctionDefine = lo.Map([]*openai.FunctionDefinition{
	{
		Name:        FUNCTION_NAME_SEARCH_USER_KNOWLEDGES,
		Description: "查询用户知识库中的相关知识，如果已经查过了，请不要连续性的重复查询",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"query": {
					Type:        jsonschema.String,
					Description: "用户的问题",
				},
			},
			Required: []string{"query"},
		},
	},
}, func(item *openai.FunctionDefinition, _ int) openai.Tool {
	return openai.Tool{
		Function: item,
	}
})

// RagTool 基于 eino 框架的 RAG 工具
type RagTool struct {
	core      *core.Core
	spaceID   string
	userID    string
	sessionID string
	messageID string
}

// NewRagTool 创建新的 RAG 工具实例
func NewRagTool(core *core.Core, spaceID, userID, sessionID, messageID string) *RagTool {
	return &RagTool{
		core:      core,
		spaceID:   spaceID,
		userID:    userID,
		sessionID: sessionID,
		messageID: messageID,
	}
}

var _ tool.InvokableTool = (*RagTool)(nil)

// Info 实现 BaseTool 接口，返回工具信息
func (r *RagTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	// 创建参数定义
	params := map[string]*schema.ParameterInfo{
		"query": {
			Type:     schema.String,
			Desc:     "用户的问题",
			Required: true,
		},
	}

	// 创建参数描述
	paramsOneOf := schema.NewParamsOneOfByParams(params)

	return &schema.ToolInfo{
		Name:        FUNCTION_NAME_SEARCH_USER_KNOWLEDGES,
		Desc:        "查询用户知识库中的相关知识，如果已经查过了，请不要连续性的重复查询",
		ParamsOneOf: paramsOneOf,
	}, nil
}

// InvokableRun 实现 InvokableTool 接口，执行工具调用
func (r *RagTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 解析输入参数
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	// 执行查询增强
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	enhanceResult, _ := EnhanceChatQuery(ctx, r.core, params.Query, r.spaceID, r.sessionID, r.messageID)

	// 记录查询增强的使用量
	if enhanceResult.Usage != nil {
		process.NewRecordChatUsageRequest(enhanceResult.Model, types.USAGE_SUB_TYPE_QUERY_ENHANCE, r.messageID, enhanceResult.Usage)
	}

	// 获取相关知识
	docs, usages, err := GetQueryRelevanceKnowledges(r.core, r.spaceID, r.userID, enhanceResult.ResultQuery(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get query relevance knowledges: %w", err)
	}

	// 记录使用量
	if len(usages) > 0 {
		for _, v := range usages {
			process.NewRecordChatUsageRequest(v.Usage.Model, v.Subject, r.messageID, v.Usage.Usage)
		}
	}

	// 补充会话相关文档
	SupplementSessionChatDocs(r.core, r.spaceID, r.sessionID, docs)

	// 构建 RAG 提示词
	ragPrompt := ai.BuildRAGPrompt(BasePrompt, ai.NewDocs(docs.Docs), r.core.Srv().AI())

	// 返回结果
	result := fmt.Sprintf("Tool '%s' Response:\n%s", FUNCTION_NAME_SEARCH_USER_KNOWLEDGES, ragPrompt)

	return result, nil
}
