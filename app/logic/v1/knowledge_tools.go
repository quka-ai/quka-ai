package v1

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/knowledge"
)

// NewKnowledgeToolsWithLogic 创建 knowledge tools,通过闭包注入 logic 层方法
func NewKnowledgeToolsWithLogic(core *core.Core, spaceID, sessionID, userID string) []tool.InvokableTool {
	ctx := context.Background()

	// 创建 logic 实例并封装方法
	knowledgeLogic := NewKnowledgeLogic(ctx, core)
	resourceLogic := NewResourceLogic(ctx, core)

	knowledgeFuncs := knowledge.KnowledgeLogicFunctions{
		InsertContentAsyncWithSource: knowledgeLogic.InsertContentAsyncWithSource,
		GetKnowledge:                 knowledgeLogic.GetKnowledge,
		Update:                       knowledgeLogic.Update,
	}

	resourceFuncs := knowledge.ResourceLogicFunctions{
		GetResource:       resourceLogic.GetResource,
		ListUserResources: resourceLogic.ListUserResources,
	}

	// 通过依赖注入方式创建 tools
	return knowledge.GetKnowledgeToolsWithLogic(
		core,
		spaceID,
		sessionID,
		userID,
		knowledgeFuncs,
		resourceFuncs,
	)
}
