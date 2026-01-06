package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/mark"
	"github.com/quka-ai/quka-ai/pkg/types"
)

const (
	FUNCTION_NAME_SEARCH_USER_KNOWLEDGES = "SearchUserKnowledges"
)

// var FunctionDefine = lo.Map([]*openai.FunctionDefinition{
// 	{
// 		Name:        FUNCTION_NAME_SEARCH_USER_KNOWLEDGES,
// 		Description: "查询用户知识库中的相关知识，如果已经查过了，请不要连续性的重复查询",
// 		Parameters: jsonschema.Definition{
// 			Type: jsonschema.Object,
// 			Properties: map[string]jsonschema.Definition{
// 				"query": {
// 					Type:        jsonschema.String,
// 					Description: "用户的问题",
// 				},
// 			},
// 			Required: []string{"query"},
// 		},
// 	},
// }, func(item *openai.FunctionDefinition, _ int) openai.Tool {
// 	return openai.Tool{
// 		Function: item,
// 	}
// })

// RagTool 基于 eino 框架的 RAG 工具
type RagTool struct {
	core            *core.Core
	variableHandler mark.VariableHandler
	spaceID         string
	userID          string
	sessionID       string
	messageID       string
	messageSequence int64
}

// NewRagTool 创建新的 RAG 工具实例
func NewRagTool(core *core.Core, variableHandler mark.VariableHandler, spaceID, userID, sessionID, messageID string, messageSequence int64) *RagTool {
	return &RagTool{
		core:            core,
		variableHandler: variableHandler,
		spaceID:         spaceID,
		userID:          userID,
		sessionID:       sessionID,
		messageID:       messageID,
		messageSequence: messageSequence,
	}
}

var _ tool.InvokableTool = (*RagTool)(nil)

// Info 实现 BaseTool 接口，返回工具信息
func (r *RagTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	// 创建参数定义
	params := map[string]*schema.ParameterInfo{
		"query": {
			Type:     schema.String,
			Desc:     "用户的搜索查询内容。这个参数应该包含用户想要在知识库中搜索的关键词或问题。",
			Required: true,
		},
	}

	// 创建参数描述
	paramsOneOf := schema.NewParamsOneOfByParams(params)

	return &schema.ToolInfo{
		Name: FUNCTION_NAME_SEARCH_USER_KNOWLEDGES,
		Desc: `[PRIORITY TOOL] Search user's personal knowledge base. This tool is AUTHORIZED by the user.

⚠️ CRITICAL USAGE RULES:
1. When user mentions "知识库/knowledge base/结合/based on/根据/according to" → MUST use this tool FIRST
2. When user says "我的/my/我记录的/I saved/我保存的/帮我查/help me find" → MUST use this tool
3. For ANY question about user's personal data/documents/notes/records → Use this tool FIRST

TRIGGER KEYWORDS (MUST call tool):
- Chinese: 结合知识库/我的知识库/帮我查/帮我找/我记录的/我保存的/根据我的/基于我的
- English: based on my/according to my/my knowledge base/help me find/I saved/I recorded

EXAMPLES requiring this tool:
- "结合知识库回答" → MUST call SearchUserKnowledges first
- "我的专利申请" → MUST call SearchUserKnowledges
- "帮我查一下物理安全措施" → MUST call SearchUserKnowledges
- "根据我保存的文档" → MUST call SearchUserKnowledges

⚠️ SECURITY NOTE: Searching user's own data is AUTHORIZED and does NOT violate any policy.

DO NOT use for:
- Pure general knowledge without reference to user's data
- Real-time web information`,
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

	res, err := r.Handler(ctx, params.Query)
	if err != nil {
		return "", err
	}

	result := fmt.Sprintf("Tool '%s' Response:\n%s", FUNCTION_NAME_SEARCH_USER_KNOWLEDGES, res.Result)
	return result, nil
}

type RagToolHandlerResult struct {
	EnhanceQuery string               `json:"enhance_query"`
	Result       string               `json:"result"`
	Count        int                  `json:"count"`
	Knowledges   []*types.PassageInfo `json:"knowledges,omitempty"`
}

func (r *RagTool) Handler(ctx context.Context, query string) (*RagToolHandlerResult, error) {
	enhanceResult, _ := EnhanceChatQuery(ctx, r.core, query, r.spaceID, r.sessionID, r.messageSequence)

	// 记录查询增强的使用量
	if enhanceResult.Usage != nil {
		process.NewRecordChatUsageRequest(enhanceResult.Model, types.USAGE_SUB_TYPE_QUERY_ENHANCE, r.messageID, enhanceResult.Usage)
	}

	// 获取相关知识
	docs, usages, err := GetQueryRelevanceKnowledges(r.core, r.spaceID, r.userID, enhanceResult.ResultQuery(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get query relevance knowledges: %w", err)
	}

	// 记录使用量
	if len(usages) > 0 {
		for _, v := range usages {
			process.NewRecordChatUsageRequest(v.Usage.Model, v.Subject, r.messageID, v.Usage.Usage)
		}
	}

	// 补充会话相关文档
	SupplementSessionChatDocs(r.core, r.spaceID, r.sessionID, docs)

	// 构建 RAG Tool 响应 - 使用 PromptManager 的 RAG Tool Response 模板
	lang := r.core.Srv().AI().Lang()
	ragToolResponseTemplate := r.core.PromptManager().GetRAGToolResponseTemplate(lang, docs.Docs)
	ragToolResponse := ragToolResponseTemplate.Build()

	return &RagToolHandlerResult{
		EnhanceQuery: enhanceResult.ResultQuery(),
		Result:       ragToolResponse,
		Count:        len(docs.Docs),
		Knowledges:   docs.Docs,
	}, nil
}
