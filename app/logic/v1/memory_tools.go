package v1

import (
	"context"

	"github.com/cloudwego/eino/components/tool"

	"github.com/quka-ai/quka-ai/app/core"
	memoryagent "github.com/quka-ai/quka-ai/pkg/ai/agents/memory"
)

func NewMemoryToolsWithLogic(core *core.Core, spaceID, sessionID, userID string) []tool.InvokableTool {
	memoryFuncs := memoryagent.MemoryLogicFunctions{
		Recall: func(ctx context.Context, spaceID string, args memoryagent.RecallArgs) ([]memoryagent.RecallItem, error) {
			items, err := NewMemoryLogic(ctx, core).Recall(spaceID, RecallMemoryArgs{
				Query:       args.Query,
				Limit:       args.Limit,
				Scopes:      args.Scopes,
				MemoryTypes: args.MemoryTypes,
			})
			if err != nil {
				return nil, err
			}
			res := make([]memoryagent.RecallItem, 0, len(items))
			for _, item := range items {
				res = append(res, memoryagent.RecallItem{
					Memory:    item.Memory,
					Knowledge: item.Knowledge,
				})
			}
			return res, nil
		},
		Remember: func(ctx context.Context, spaceID string, args memoryagent.RememberArgs) (string, string, error) {
			return NewMemoryLogic(ctx, core).Remember(spaceID, RememberMemoryArgs{
				Resource:        args.Resource,
				Title:           args.Title,
				Content:         args.Content,
				ContentType:     args.ContentType,
				Kind:            args.Kind,
				MemoryType:      args.MemoryType,
				Scope:           args.Scope,
				AuthorType:      args.AuthorType,
				EpistemicStatus: args.EpistemicStatus,
				EntityKey:       args.EntityKey,
				Importance:      args.Importance,
				Confidence:      args.Confidence,
				SourceKind:      args.SourceKind,
				SourceRef:       args.SourceRef,
			})
		},
		Forget: func(ctx context.Context, spaceID, id string, deleteKnowledge bool) error {
			return NewMemoryLogic(ctx, core).Forget(spaceID, id, deleteKnowledge)
		},
	}

	return memoryagent.GetMemoryToolsWithLogic(core, spaceID, sessionID, userID, memoryFuncs)
}
