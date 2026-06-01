package v1

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/knowledge"
	"github.com/quka-ai/quka-ai/pkg/types"
)

// NewKnowledgeToolsWithLogic 创建 knowledge tools,通过闭包注入 logic 层方法
func NewKnowledgeToolsWithLogic(core *core.Core, spaceID, sessionID, userID string) []tool.InvokableTool {
	ctx := context.Background()

	resourceLogic := NewResourceLogic(ctx, core)

	knowledgeFuncs := knowledge.KnowledgeLogicFunctions{
		InsertContentAsyncWithSource: func(ctx context.Context, spaceID, resource string, kind types.KnowledgeKind, content types.KnowledgeContent, contentType types.KnowledgeContentType, source types.KnowledgeSource, sourceRef string) (string, error) {
			return NewKnowledgeLogic(ctx, core).InsertContentAsyncWithSource(spaceID, resource, kind, content, contentType, source, sourceRef)
		},
		GetKnowledge: func(ctx context.Context, spaceID, id string) (*types.Knowledge, error) {
			return NewKnowledgeLogic(ctx, core).GetKnowledge(spaceID, id)
		},
		Update: func(ctx context.Context, spaceID, id string, args types.UpdateKnowledgeArgs) error {
			return NewKnowledgeLogic(ctx, core).Update(spaceID, id, args)
		},
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
