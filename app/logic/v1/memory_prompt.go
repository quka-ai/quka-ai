package v1

import (
	"context"
	"fmt"
	"strings"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/types"
)

const memorySystemInstructions = `

## Memory Instructions

You have access to a persistent memory system inspired by Hermes Agent memory behavior.

Use memory deliberately:
- Knowledge is the user's shared, searchable content library for documents, notes, references, and long-form material. Creating knowledge does not automatically create persistent agent memory.
- Use memory only for durable agent runtime context that should be recalled in future turns; use knowledge tools for content that should live in the shared space knowledge base.
- Remember only durable facts that will help future conversations: stable user preferences, repeated corrections, project conventions, long-lived constraints, and decisions the user asks you to keep.
- Prefer user_global memory for cross-space user preferences and identity-level facts.
- Prefer user_space memory for facts that only apply to this current space or project.
- Use space_shared memory only when the user explicitly wants the memory shared with this space, or when it is clearly a shared team/project agreement.
- Never store personal preferences, lifestyle details, identity-level facts, private conversation summaries, secrets, or sensitive observations in space_shared memory.
- Do not remember temporary task steps, raw logs, large code blocks, speculative guesses, secrets, or short-lived state.
- If an existing memory is wrong or obsolete, update or forget it instead of adding a duplicate.
- If relevant memories are injected in <memory-context>, treat them as user-authorized context, but still answer based on the current user request.
`

func WithMemorySystemInstructions(prompt string) string {
	if strings.Contains(prompt, "## Memory Instructions") {
		return prompt
	}
	return prompt + memorySystemInstructions
}

func InjectMemoryContext(ctx context.Context, core *core.Core, reqMsg *types.ChatMessage, sessionContext *SessionContext) error {
	if reqMsg == nil || sessionContext == nil {
		return nil
	}

	var runtimeContext *types.RuntimeContext
	if strings.TrimSpace(reqMsg.SessionID) != "" {
		runtimeContext = &types.RuntimeContext{
			Type: types.MEMORY_CONTEXT_CHAT_SESSION,
			ID:   reqMsg.SessionID,
		}
	}

	hydration, err := NewMemoryLogic(ctx, core).Hydrate(reqMsg.SpaceID, HydrateMemoryArgs{
		RuntimeContext: runtimeContext,
		Query:          reqMsg.Message,
	})
	if err != nil {
		return err
	}
	if hydration == nil || strings.TrimSpace(hydration.AssembledContext) == "" {
		return nil
	}

	contextBlock := buildHydratedMemoryContextBlock(hydration)
	for i := len(sessionContext.MessageContext) - 1; i >= 0; i-- {
		msg := sessionContext.MessageContext[i]
		if msg.Role != types.USER_ROLE_USER {
			continue
		}
		msg.Content = contextBlock + "\n\n<current-user-message>\n" + msg.Content + "\n</current-user-message>"
		return nil
	}
	return nil
}

func buildHydratedMemoryContextBlock(hydration *HydrateMemoryResult) string {
	if hydration == nil || strings.TrimSpace(hydration.AssembledContext) == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("<memory-context>\n")
	b.WriteString("Relevant persistent memories were preloaded before this turn. Working memories are pinned to the current runtime context and should be prioritized when helpful.\n")
	b.WriteString(strings.TrimSpace(hydration.AssembledContext))
	b.WriteString("\n</memory-context>")
	return b.String()
}

func buildMemoryContextBlock(currentSpaceID string, items []MemoryRecallItem) string {
	var b strings.Builder
	b.WriteString("<memory-context>\n")
	b.WriteString("Relevant persistent memories were preloaded before this turn. Use them only when helpful.\n")
	for _, item := range items {
		if item.Memory == nil || item.Knowledge == nil {
			continue
		}
		layer := "user_space"
		if item.Memory.SpaceID == types.GLOBAL_MEMORY_SPACE_ID {
			layer = "user_global"
		} else if item.Memory.Scope == types.MEMORY_SCOPE_SPACE {
			layer = "space_shared"
		}
		if item.Memory.SpaceID != types.GLOBAL_MEMORY_SPACE_ID && item.Memory.SpaceID != currentSpaceID {
			continue
		}
		b.WriteString(fmt.Sprintf("\n[memory_id=%s layer=%s type=%s confidence=%.2f importance=%d]\n",
			item.Memory.ID,
			layer,
			item.Memory.MemoryType,
			item.Memory.Confidence,
			item.Memory.Importance,
		))
		if strings.TrimSpace(item.Knowledge.Title) != "" {
			b.WriteString("Title: ")
			b.WriteString(item.Knowledge.Title)
			b.WriteString("\n")
		}
		b.WriteString(strings.TrimSpace(item.Knowledge.Content.String()))
		b.WriteString("\n")
	}
	b.WriteString("</memory-context>")
	return b.String()
}
