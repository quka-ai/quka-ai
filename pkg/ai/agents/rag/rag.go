package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/pgvector/pgvector-go"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
)

func EnhanceChatQuery(ctx context.Context, core *core.Core, query string, spaceID, sessionID string, msgSequence int64) (ai.EnhanceQueryResult, error) {
	histories, err := core.Store().ChatMessageStore().ListSessionMessageUpToGivenID(ctx, spaceID, sessionID, msgSequence, 1, 6)
	if err != nil {
		slog.Error("Failed to get session message history", slog.String("space_id", spaceID), slog.String("session_id", sessionID),
			slog.Int64("message_sequence", msgSequence), slog.String("error", err.Error()))
	}

	if len(histories) <= 1 {
		return ai.EnhanceQueryResult{
			Original: query,
		}, nil
	}

	histories = lo.Reverse(histories)[:len(histories)-1]

	decryptMessageLists(core, histories)

	return EnhanceQuery(ctx, core, query, histories)
}

func decryptMessageLists(core *core.Core, messages []*types.ChatMessage) {
	for _, v := range messages {
		if v.IsEncrypt == types.MESSAGE_IS_ENCRYPT {
			value, _ := core.DecryptData([]byte(v.Message))
			v.Message = string(value)
		}
	}
}

func EnhanceQuery(ctx context.Context, core *core.Core, query string, histories []*types.ChatMessage) (ai.EnhanceQueryResult, error) {
	aiOpts := core.Srv().AI().NewEnhance(ctx)
	resp, err := aiOpts.WithPrompt(core.Prompt().EnhanceQuery).
		WithHistories(histories).
		EnhanceQuery(query)
	if err != nil {
		slog.Error("failed to enhance user query", slog.String("query", query), slog.String("error", err.Error()))
		// return nil, errors.New("KnowledgeLogic.GetRelevanceKnowledges.AI.EnhanceQuery", i18n.ERROR_INTERNAL, err)
	}

	resp.Original = query
	return resp, nil
}

// 补充 session pin docs to docs
func SupplementSessionChatDocs(core *core.Core, spaceID, sessionID string, docs types.RAGDocs) {
	if len(docs.Refs) == 0 {
		return
	}

	pin, err := core.Store().ChatSessionPinStore().GetBySessionID(context.Background(), sessionID)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("Failed to get chat session pin", slog.String("session_id", sessionID), slog.String("error", err.Error()))
		return
	}

	if pin == nil || len(pin.Content) == 0 || pin.Version != types.CHAT_SESSION_PIN_VERSION_V1 {
		return
	}

	var p types.ContentPinV1
	if err = json.Unmarshal(pin.Content, &p); err != nil {
		slog.Error("Failed to unmarshal chat session pin content", slog.String("session_id", sessionID), slog.String("error", err.Error()))
		return
	}

	if len(p.Knowledges) == 0 {
		return
	}

	differenceItems, _ := lo.Difference(p.Knowledges, lo.Map(docs.Refs, func(item types.QueryResult, _ int) string { return item.KnowledgeID }))
	if len(differenceItems) == 0 {
		return
	}

	// Get the knowledge content of the difference item
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	knowledges, err := core.Store().KnowledgeStore().ListKnowledges(ctx, types.GetKnowledgeOptions{
		SpaceID: spaceID,
		IDs:     differenceItems,
	}, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		slog.Error("Failed to get knowledge content", slog.String("session_id", sessionID), slog.String("error", err.Error()), slog.Any("knowledge_ids", differenceItems))
		return
	}

	for _, v := range knowledges {
		if v.Content, err = core.DecryptData(v.Content); err != nil {
			slog.Error("Failed to decrypt knowledge data", slog.String("session_id", sessionID), slog.String("error", err.Error()))
			return
		}
	}

	if docs.Docs, err = core.AppendKnowledgeContentToDocs(docs.Docs, knowledges); err != nil {
		slog.Error("Failed to append knowledge content to docs", slog.String("session_id", sessionID), slog.String("error", err.Error()))
		return
	}
}

func GetQueryRelevanceKnowledges(core *core.Core, spaceID, userID, query string, resource *types.ResourceQuery) (types.RAGDocs, []ai.UsageItem, error) {
	var (
		result types.RAGDocs
		usages []ai.UsageItem
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	vector, err := core.Srv().AI().EmbeddingForQuery(ctx, []string{query})
	if err != nil || len(vector.Data) == 0 {
		return types.RAGDocs{}, nil, fmt.Errorf("failed to get embedding for query: %w", err)
	}

	refs, err := core.Store().VectorStore().Query(ctx, types.GetVectorsOptions{
		SpaceID:  spaceID,
		UserID:   userID,
		Resource: resource,
	}, pgvector.NewVector(vector.Data[0]), 100)
	if err != nil {
		return types.RAGDocs{}, nil, fmt.Errorf("failed to query vector store: %w", err)
	}

	slog.Debug("got query result", slog.String("query", query), slog.Any("result", refs))
	if len(refs) == 0 {
		return types.RAGDocs{}, nil, nil
	}

	// rerank
	var (
		knowledgeIDs       []string
		cosLimit           float32 = 0.5
		highScoreKnowledge []types.QueryResult
	)

	if len(refs) > 10 && refs[0].Cos < 0.5 {
		cosLimit = refs[0].Cos - 0.1
	}
	for i, v := range refs {
		if i > 0 && (v.Cos < cosLimit && v.OriginalLength > 200) {
			if len(result.Refs) > 15 {
				break
			}
			// TODO：more and more verify best ratio
			continue
		}

		if i < 3 {
			highScoreKnowledge = append(highScoreKnowledge, v)
		}

		result.Refs = append(result.Refs, v)
	}

	result.Refs = lo.UniqBy(result.Refs, func(item types.QueryResult) string {
		return item.KnowledgeID
	})

	for _, v := range result.Refs {
		knowledgeIDs = append(knowledgeIDs, v.KnowledgeID)
	}

	knowledges, err := core.Store().KnowledgeStore().ListKnowledges(ctx, types.GetKnowledgeOptions{
		IDs:      knowledgeIDs,
		SpaceID:  spaceID,
		UserID:   userID,
		Resource: resource,
	}, 1, 100)
	if err != nil && err != sql.ErrNoRows {
		return types.RAGDocs{}, nil, fmt.Errorf("failed to list knowledges: %w", err)
	}

	for _, v := range knowledges {
		if v.Content, err = core.DecryptData(v.Content); err != nil {
			return types.RAGDocs{}, nil, fmt.Errorf("failed to decrypt knowledge data: %w", err)
		}

		if v.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS {
			content, err := editorjs.ConvertEditorJSRawToMarkdown(json.RawMessage(v.Content))
			if err != nil {
				slog.Error("Failed to convert editor blocks to markdown", slog.String("knowledge_id", v.ID), slog.String("error", err.Error()))
				continue
			}

			v.ContentType = types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN
			v.Content = types.KnowledgeContent(content)
		}
	}

	slog.Debug("match knowledges", slog.String("query", query), slog.Any("resource", resource), slog.Int("knowledge_length", len(knowledges)))
	if len(knowledges) == 0 {
		// return nil, errors.New("KnowledgeLogic.Query.KnowledgeStore.ListKnowledge.nil", i18n.ERROR_LOGIC_VECTOR_DB_NOT_MATCHED_CONTENT_DB, nil)
		return result, usages, nil
	}

	rankList, usage, err := core.Rerank(query, knowledges)
	if err != nil {
		slog.Error("Failed to request rerank api", slog.String("error", err.Error()))
		// return result, usage, errors.New("KnowledgeLogic.Query.Rerank", i18n.ERROR_INTERNAL, err)
		rankList = knowledges
	}

	// TODO: improve: map index
	for _, v := range highScoreKnowledge {
		_, exist := lo.Find(rankList, func(item *types.Knowledge) bool {
			return item.ID == v.KnowledgeID
		})
		if !exist {
			result, exist := lo.Find(knowledges, func(item *types.Knowledge) bool {
				return item.ID == v.KnowledgeID
			})
			if exist {
				rankList = append(rankList, result)
			}
		}
	}

	slog.Debug("rerank result", slog.Int("knowledge_length", len(rankList)))

	if usage != nil {
		usages = append(usages, ai.UsageItem{
			Usage: ai.Usage{
				Model: usage.Model,
				Usage: &openai.Usage{
					PromptTokens: usage.Usage.PromptTokens,
				},
			},
			Subject: types.USAGE_SUB_TYPE_RERANK,
		})
	}

	if result.Docs, err = core.AppendKnowledgeContentToDocs(result.Docs, rankList); err != nil {
		return result, usages, fmt.Errorf("failed to append knowledge content to docs: %w", err)
	}

	return result, usages, nil
}
