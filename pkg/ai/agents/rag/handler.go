package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
	"github.com/sashabaranov/go-openai"
)

type ToolContext struct {
	Core        *core.Core
	MessageID   string
	SessionID   string
	SpaceID     string
	UserID      string
	marks       map[string]string
	ReceiveFunc types.ReceiveFunc
}

func GetToolsHandler(core *core.Core, sessionID, spaceID, userID string, marks map[string]string, receiveFunc types.ReceiveFunc) map[string]ai.ToolHandlerFunc {
	return map[string]ai.ToolHandlerFunc{
		// 查询用户知识库中的相关知识
		FUNCTION_NAME_SEARCH_USER_KNOWLEDGES: ai.WrapToolHandler(func() ToolContext {
			return ToolContext{Core: core, SessionID: sessionID, SpaceID: spaceID, UserID: userID, ReceiveFunc: receiveFunc, marks: marks}
		}, searchKnowledge),
	}
}

type searchKnowledgeParams struct {
	Query string `json:"query"`
}

func searchKnowledge(args ToolContext, funcCall openai.FunctionCall) ([]*types.MessageContext, error) {
	toolID := utils.GenUniqIDStr()
	args.ReceiveFunc(&types.ToolTips{
		ID:       toolID,
		ToolName: "SearchKnowledge",
		Status:   types.TOOL_STATUS_RUNNING,
		Content:  "Retrieving your knowledge...",
	}, types.MESSAGE_PROGRESS_GENERATING)

	var reviewedKnowledges int
	defer func() {
		args.ReceiveFunc(&types.ToolTips{
			ID:       toolID,
			ToolName: "SearchKnowledge",
			Status:   types.TOOL_STATUS_SUCCESS,
			Content:  fmt.Sprintf("%d knowledges reviewed", reviewedKnowledges),
		}, types.MESSAGE_PROGRESS_GENERATING)
	}()

	var params searchKnowledgeParams
	if err := json.Unmarshal([]byte(funcCall.Arguments), &params); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	enhanceResult, _ := EnhanceChatQuery(ctx, args.Core, params.Query, args.SpaceID, args.SessionID, args.MessageID)

	if enhanceResult.Usage != nil {
		process.NewRecordChatUsageRequest(enhanceResult.Model, types.USAGE_SUB_TYPE_QUERY_ENHANCE, args.MessageID, enhanceResult.Usage)
	}

	docs, usages, err := GetQueryRelevanceKnowledges(args.Core, args.SpaceID, args.UserID, enhanceResult.ResultQuery(), nil)
	if len(usages) > 0 {
		for _, v := range usages {
			process.NewRecordChatUsageRequest(v.Usage.Model, v.Subject, args.MessageID, v.Usage.Usage)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get query relevance knowledges: %w", err)
	}

	reviewedKnowledges = len(docs.Docs)

	// Supplement associated document content.
	SupplementSessionChatDocs(args.Core, args.SpaceID, args.SessionID, docs)

	for _, v := range docs.Docs {
		if v.SW == nil {
			continue
		}
		for fake, real := range v.SW.Map() {
			args.marks[fake] = real
		}
	}
	return []*types.MessageContext{
		{
			Role:    types.USER_ROLE_SYSTEM,
			Content: ai.BuildRAGPrompt(ai.GENERATE_PROMPT_TPL_CN, ai.NewDocs(docs.Docs), args.Core.Srv().AI()),
		},
	}, nil
}
