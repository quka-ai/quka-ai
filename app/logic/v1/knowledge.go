package v1

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/safe"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

const CONTEXT_SCENE_PROMPT = `
以下是关于回答用户提问的“参考内容”，这些内容都是历史记录，其中提到的时间点无法与当前时间进行参照：
--------------------------------------
{solt}
--------------------------------------
你需要结合“参考内容”来回答用户的提问，
注意，“参考内容”中可能有部分内容描述的是同一件事情，但是发生的时间不同，当你无法选择应该参考哪一天的内容时，可以结合用户提出的问题进行分析。
如果你从上述内容中找到了用户想要的答案，可以结合内容相关的属性来给到用户更多的帮助，比如参考“事件发生时间”来告诉用户这件事发生在哪天。
请你使用 {lang} 语言，以Markdown格式回复用户。
`

var (
	userSetting = map[string][]ai.OptionFunc{
		"context": {
			func(opts *ai.QueryOptions) {
				opts.WithPrompt(CONTEXT_SCENE_PROMPT)
				opts.WithDocsSoltName("{solt}")
			},
		},
	}
	// resource setting
	// model,prompt,docs_solt,cycle(int days)
)

type KnowledgeLogic struct {
	UserInfo
	ctx  context.Context
	core *core.Core
}

func NewKnowledgeLogic(ctx context.Context, core *core.Core) *KnowledgeLogic {
	l := &KnowledgeLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}

	return l
}

func (l *KnowledgeLogic) GetKnowledge(spaceID, id string) (*types.Knowledge, error) {
	data, err := l.core.Store().KnowledgeStore().GetKnowledge(l.ctx, spaceID, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("KnowledgeLogic.GetKnowledge.KnowledgeStore.GetKnowledge", i18n.ERROR_INTERNAL, err)
	}

	if data == nil {
		return nil, errors.New("KnowledgeLogic.GetKnowledge.KnowledgeStore.GetKnowledge.nil", i18n.ERROR_NOT_FOUND, err).Code(http.StatusNotFound)
	}

	if len(data.Content) > 0 {
		if data.Content, err = l.core.DecryptData(data.Content); err != nil {
			return nil, errors.New("KnowledgeLogic.GetJournal.DecryptData", i18n.ERROR_INTERNAL, err)
		}
	}

	return data, nil
}

func (l *KnowledgeLogic) GetTimeRangeLiteKnowledges(spaceID string, st, et time.Time) ([]*types.KnowledgeLite, error) {
	if et.Sub(st).Hours() > 48 {
		return nil, errors.New("KnowledgeLogic.GetTimeRangeLiteKnowledges.InvalidTimeRange", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}
	data, err := l.core.Store().KnowledgeStore().ListLiteKnowledges(l.ctx, types.GetKnowledgeOptions{
		SpaceID: spaceID,
		UserID:  l.GetUserInfo().User,
		TimeRange: &struct {
			St int64
			Et int64
		}{
			St: st.Unix(),
			Et: et.Unix(),
		},
	}, types.NO_PAGING, types.NO_PAGING)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("KnowledgeLogic.GetTimeRangeLiteKnowledges.KnowledgeStore.ListLiteKnowledges", i18n.ERROR_INTERNAL, err)
	}

	return data, nil
}

func (l *KnowledgeLogic) ListKnowledges(spaceID string, keywords string, resource *types.ResourceQuery, page, pagesize uint64) ([]*types.Knowledge, uint64, error) {
	opts := types.GetKnowledgeOptions{
		SpaceID:  spaceID,
		Resource: resource,
		Keywords: keywords,
	}
	list, err := l.core.Store().KnowledgeStore().ListKnowledges(l.ctx, opts, page, pagesize)
	if err != nil && err != sql.ErrNoRows {
		return nil, 0, errors.New("KnowledgeLogic.ListKnowledge.KnowledgeStore.ListKnowledge", i18n.ERROR_INTERNAL, err)
	}

	for _, v := range list {
		if v.Content, err = l.core.DecryptData(v.Content); err != nil {
			return nil, 0, errors.New("KnowledgeLogic.ListKnowledges.DecryptData", i18n.ERROR_INTERNAL, err)
		}
	}

	total, err := l.core.Store().KnowledgeStore().Total(l.ctx, opts)
	if err != nil {
		return nil, 0, errors.New("KnowledgeLogic.ListKnowledge.KnowledgeStore.Total", i18n.ERROR_INTERNAL, err)
	}

	return list, total, nil
}

func (l *KnowledgeLogic) Delete(spaceID, id string) error {
	user := l.GetUserInfo()
	if err := l.core.Srv().RBAC().Check(user, l.lazyRolerFromKnowledgeID(spaceID, id), srv.PermissionEdit); err != nil {
		return errors.Trace("KnowledgeLogic.Delete", err)
	}

	knowledge, err := l.core.Store().KnowledgeStore().GetKnowledge(l.ctx, spaceID, id)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("KnowledgeLogic.KnowledgeStore.GetKnowledge", i18n.ERROR_INTERNAL, err)
	}
	if knowledge == nil {
		return nil
	}

	return l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		if knowledge.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS {
			if err = UpdateFilesToDelete(ctx, l.core, spaceID, knowledge.Content); err != nil {
				slog.Error("Failed to remark knowledge files to delete status", slog.String("knowledge_id", id), slog.String("space_id", spaceID), slog.Any("error", err))
			}
		}

		if err := l.core.Store().KnowledgeStore().Delete(ctx, spaceID, id); err != nil {
			return errors.New("KnowledgeLogic.Delete.KnowledgeStore.Delete", i18n.ERROR_INTERNAL, err)
		}

		if err := l.core.Store().KnowledgeChunkStore().BatchDelete(ctx, spaceID, id); err != nil {
			return errors.New("KnowledgeLogic.Delete.KnowledgeChunkStore.BatchDelete", i18n.ERROR_INTERNAL, err)
		}

		if err := l.core.Store().VectorStore().BatchDelete(ctx, spaceID, id); err != nil {
			return errors.New("KnowledgeLogic.Delete.VectorStore.Delete", i18n.ERROR_INTERNAL, err)
		}

		return nil
	})
}

func (l *KnowledgeLogic) Update(spaceID, id string, args types.UpdateKnowledgeArgs) error {
	oldKnowledge, err := l.core.Store().KnowledgeStore().GetKnowledge(l.ctx, spaceID, id)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("KnowledgeLogic.Update.KnowledgeStore.GetKnowledge", i18n.ERROR_INTERNAL, err)
	}

	if oldKnowledge == nil || oldKnowledge.UserID != l.GetUserInfo().User {
		return errors.New("KnowledgeLogic.Update.KnowledgeStore.GetKnowledge", i18n.ERROR_NOT_FOUND, err).Code(http.StatusNotFound)
	}

	tagsChanged := false
	if len(args.Tags) != 0 {
		if len(args.Tags) != len(oldKnowledge.Tags) {
			tagsChanged = true
		} else {
			for _, v := range args.Tags {
				matched := false
				for _, vv := range oldKnowledge.Tags {
					if v == vv {
						matched = true
						break
					}
				}
				if !matched {
					tagsChanged = true
					break
				}
			}
		}
	}

	if oldKnowledge.Content, err = l.core.DecryptData(oldKnowledge.Content); err != nil {
		return errors.New("KnowledgeLogic.Update.DecryptData.oldKnowledge", i18n.ERROR_INTERNAL, err)
	}

	var summary []string
	if !tagsChanged {
		summary = append(summary, "tags")
	}
	if string(args.Content) != string(oldKnowledge.Content) {
		summary = append(summary, "content")
	}
	if args.Title == "" {
		summary = append(summary, "title")
	}

	if args.Content, err = l.core.EncryptData([]byte(args.Content.String())); err != nil {
		return errors.New("KnowledgeLogic.Update.EncryptData", i18n.ERROR_INTERNAL, err)
	}

	err = l.core.Store().KnowledgeStore().Update(l.ctx, spaceID, id, types.UpdateKnowledgeArgs{
		Resource:    args.Resource,
		Title:       args.Title,
		Content:     args.Content,
		ContentType: args.ContentType,
		Tags:        args.Tags,
		Stage:       types.KNOWLEDGE_STAGE_SUMMARIZE,
		Kind:        args.Kind,
		Summary:     strings.Join(summary, ","),
	})
	if err != nil {
		return errors.New("KnowledgeLogic.Update.KnowledgeStore.Update", i18n.ERROR_INTERNAL, err)
	}

	go safe.Run(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
		defer cancel()
		knowledge, err := l.core.Store().KnowledgeStore().GetKnowledge(ctx, spaceID, id)
		if err != nil {
			slog.Error("Failed to get new knowledge after update, knowledge process stopped",
				slog.String("space_id", spaceID),
				slog.String("knowledge_id", id),
				slog.String("error", err.Error()))
			return
		}

		if knowledge.Content, err = l.core.DecryptData(knowledge.Content); err != nil {
			slog.Error("Failed to decrypt knowledge content",
				slog.String("space_id", spaceID),
				slog.String("knowledge_id", id),
				slog.String("error", err.Error()))
			return
		}

		if err = l.processKnowledgeAsync(*knowledge); err != nil {
			slog.Error("Process knowledge async failed",
				slog.String("space_id", knowledge.SpaceID),
				slog.String("knowledge_id", knowledge.ID),
				slog.Any("error", err))
		}
	})

	return nil
}

func EnhanceChatQuery(ctx context.Context, core *core.Core, query string, spaceID, sessionID, messageID string) (ai.EnhanceQueryResult, error) {
	histories, err := core.Store().ChatMessageStore().ListSessionMessageUpToGivenID(ctx, spaceID, sessionID, messageID, 1, 6)
	if err != nil {
		slog.Error("Failed to get session message history", slog.String("space_id", spaceID), slog.String("session_id", sessionID),
			slog.String("message_id", messageID), slog.String("error", err.Error()))
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

type UsageItem struct {
	Subject string
	Usage   ai.Usage
}

func (l *KnowledgeLogic) GetQueryRelevanceKnowledges(spaceID, userID, query string, resource *types.ResourceQuery) (types.RAGDocs, []UsageItem, error) {
	var (
		result types.RAGDocs
		usages []UsageItem
	)

	vector, err := l.core.Srv().AI().EmbeddingForQuery(l.ctx, []string{query})
	if err != nil || len(vector.Data) == 0 {
		return types.RAGDocs{}, nil, errors.New("KnowledgeLogic.GetRelevanceKnowledges.AI.EmbeddingForQuery", i18n.ERROR_INTERNAL, err)
	}

	refs, err := l.core.Store().VectorStore().Query(l.ctx, types.GetVectorsOptions{
		SpaceID:  spaceID,
		UserID:   userID,
		Resource: resource,
	}, pgvector.NewVector(vector.Data[0]), 100)
	if err != nil {
		return types.RAGDocs{}, nil, errors.New("KnowledgeLogic.GetRelevanceKnowledges.VectorStore.Query", i18n.ERROR_INTERNAL, err)
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

	knowledges, err := l.core.Store().KnowledgeStore().ListKnowledges(l.ctx, types.GetKnowledgeOptions{
		IDs:      knowledgeIDs,
		SpaceID:  spaceID,
		UserID:   userID,
		Resource: resource,
	}, 1, 100)
	if err != nil && err != sql.ErrNoRows {
		return types.RAGDocs{}, nil, errors.New("KnowledgeLogic.Query.KnowledgeStore.ListKnowledge", i18n.ERROR_INTERNAL, err)
	}

	for _, v := range knowledges {
		if v.Content, err = l.core.DecryptData(v.Content); err != nil {
			return types.RAGDocs{}, nil, errors.New("KnowledgeLogic.Query.DecryptData", i18n.ERROR_INTERNAL, err)
		}

		if v.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS {
			content, err := utils.ConvertEditorJSBlocksToMarkdown(json.RawMessage(v.Content))
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

	rankList, usage, err := l.core.Rerank(query, knowledges)
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
		usages = append(usages, UsageItem{
			Usage: ai.Usage{
				Model: usage.Model,
				Usage: &openai.Usage{
					PromptTokens: usage.Usage.PromptTokens,
				},
			},
			Subject: types.USAGE_SUB_TYPE_RERANK,
		})
	}

	if result.Docs, err = l.core.AppendKnowledgeContentToDocs(result.Docs, rankList); err != nil {
		return result, usages, errors.New("KnowledgeLogic.Query.AppendKnowledgeContentToDocs", i18n.ERROR_INTERNAL, err)
	}

	return result, usages, nil
}

type KnowledgeQueryResult struct {
	Refs    []types.QueryResult `json:"-"`
	Message string              `json:"message"`
}

func (l *KnowledgeLogic) Query(spaceID, agent string, resource *types.ResourceQuery, query string) (*KnowledgeQueryResult, error) {
	msgArgs := &types.ChatMessage{
		ID:        utils.GenUniqIDStr(),
		UserID:    l.GetUserInfo().User,
		SpaceID:   spaceID,
		SessionID: "", // session 为空则表示为 query
		Message:   query,
		MsgType:   types.MESSAGE_TYPE_TEXT,
		SendTime:  time.Now().Unix(),
		Role:      types.USER_ROLE_USER,
		Complete:  types.MESSAGE_PROGRESS_COMPLETE,
	}

	if err := l.core.Store().ChatMessageStore().Create(l.ctx, msgArgs); err != nil {
		return nil, errors.New("KnowledgeLogic.Query.ChatMessageStore.Create", i18n.ERROR_INTERNAL, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()

	var (
		responseChan = make(chan types.MessageContent, 1)
		receiver     = NewQueryReceiver(ctx, l.core, responseChan)

		result = &KnowledgeQueryResult{}
		err    error
	)

	containsAgent := types.FilterAgent(msgArgs.Message)
	if containsAgent == types.AGENT_TYPE_NONE {
		containsAgent = agent
	}

	// check agents call
	switch containsAgent {
	case types.AGENT_TYPE_BUTLER:
		err = ButlerHandle(l.core, receiver, msgArgs)
		if err != nil {
			slog.Error("Failed to handle butler message", slog.String("msg_id", msgArgs.ID), slog.String("error", err.Error()))
		}
	case types.AGENT_TYPE_JOURNAL:
		err = JournalHandle(l.core, receiver, msgArgs)
		if err != nil {
			slog.Error("Failed to handle journal message", slog.String("msg_id", msgArgs.ID), slog.String("error", err.Error()))
		}
	case types.AGENT_TYPE_NORMAL:

		enhanceResult, _ := EnhanceChatQuery(l.ctx, l.core, msgArgs.Message, msgArgs.SpaceID, msgArgs.SessionID, msgArgs.ID)

		process.NewRecordChatUsageRequest(enhanceResult.Model, types.USAGE_SUB_TYPE_QUERY_ENHANCE, msgArgs.ID, enhanceResult.Usage)

		docs, usages, err := NewKnowledgeLogic(l.ctx, l.core).GetQueryRelevanceKnowledges(msgArgs.SpaceID, l.GetUserInfo().User, msgArgs.Message, resource)
		if len(usages) > 0 {
			for _, v := range usages {
				process.NewRecordChatUsageRequest(v.Usage.Model, v.Subject, msgArgs.ID, v.Usage.Usage)
			}
		}
		if err != nil {
			err = errors.Trace("ChatLogic.getRelevanceKnowledges", err)
		} else {
			result.Refs = docs.Refs

			if err = RAGHandle(l.core, receiver, msgArgs, docs, types.GEN_MODE_NORMAL); err != nil {
				slog.Error("Failed to handle rag message", slog.String("msg_id", msgArgs.ID), slog.String("error", err.Error()))
			}
		}
	default:
		err = ChatHandle(l.core, receiver, msgArgs, types.GEN_MODE_NORMAL)
		if err != nil {
			slog.Error("Failed to handle message", slog.String("msg_id", msgArgs.ID), slog.String("error", err.Error()))
		}
	}

	if err != nil {
		return nil, err
	}

	queryResult := <-responseChan
	if queryResult != nil {
		result.Message = string(queryResult.Bytes())
	}

	return result, nil
}

type BlockFile struct {
	File struct {
		Url string `json:"url"`
	} `json:"file"`
}

func filterKnowledgeFiles(content types.KnowledgeContent) ([]string, error) {
	var parsedData types.BlockContent
	if err := json.Unmarshal(content, &parsedData); err != nil {
		// TODO: support markdown
		return nil, errors.New("updateFilesUploaded.ParseContentBlocks", i18n.ERROR_INTERNAL, err)
	}

	var files []string
	for _, v := range parsedData.Blocks {
		if v.Type != "image" && v.Type != "video" {
			continue
		}

		var img BlockFile
		if err := json.Unmarshal(v.Data, &img); err != nil {
			return nil, errors.New("updateFilesUploaded.ParseImageBlock", i18n.ERROR_INTERNAL, err)
		}

		if img.File.Url != "" {
			files = append(files, img.File.Url)
		}
	}

	return files, nil
}

func UpdateFilesUploaded(ctx context.Context, core *core.Core, spaceID string, content types.KnowledgeContent) error {
	paths, err := parseEditorJsonToFilesPath(core, content)
	if err != nil {
		return errors.Trace("UpdateFilesUploaded", err)
	}

	if err := core.Store().FileManagementStore().UpdateStatus(ctx, spaceID, paths, types.FILE_UPLOAD_STATUS_UPLOADED); err != nil {
		return errors.New("UpdateFilesUploaded.FileManagementStore.UpdateStatus", i18n.ERROR_INTERNAL, err)
	}
	return nil
}

func parseEditorJsonToFilesPath(core *core.Core, content types.KnowledgeContent) ([]string, error) {
	files, err := filterKnowledgeFiles(content)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, nil
	}

	paths := lo.Map(files, func(item string, _ int) string {
		parsed, err := url.Parse(item)
		if err != nil {
			return item
		}
		return parsed.RequestURI()
	})

	return paths, nil
}

func UpdateFilesToDelete(ctx context.Context, core *core.Core, spaceID string, content types.KnowledgeContent) error {
	paths, err := parseEditorJsonToFilesPath(core, content)
	if err != nil {
		return errors.Trace("UpdateFilesToDelete", err)
	}
	if err := core.Store().FileManagementStore().UpdateStatus(ctx, spaceID, paths, types.FILE_UPLOAD_STATUS_NEED_TO_DELETE); err != nil {
		return errors.New("UpdateFilesToDelete.FileManagementStore.UpdateStatus", i18n.ERROR_INTERNAL, err)
	}
	return nil
}

func (l *KnowledgeLogic) insertContent(isSync bool, spaceID, resource string, kind types.KnowledgeKind, content types.KnowledgeContent, contentType types.KnowledgeContentType) (string, error) {
	if resource == "" {
		resource = types.DEFAULT_RESOURCE
	}

	var (
		err         error
		encryptData []byte
	)

	if encryptData, err = l.core.EncryptData([]byte(content.String())); err != nil {
		return "", errors.New("KnowledgeLogic.InsertContent.EncryptDatae", i18n.ERROR_INTERNAL, err)
	}

	knowledgeID := utils.GenRandomID()
	user := l.GetUserInfo()
	knowledge := types.Knowledge{
		ID:          knowledgeID,
		SpaceID:     spaceID,
		UserID:      user.User,
		Resource:    resource,
		Content:     encryptData,
		ContentType: contentType,
		Kind:        kind,
		Stage:       types.KNOWLEDGE_STAGE_SUMMARIZE,
		MaybeDate:   time.Now().Local().Format("2006-01-02 15:04"),
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}
	err = l.core.Store().KnowledgeStore().Create(l.ctx, knowledge)
	if err != nil {
		return "", errors.New("KnowledgeLogic.InsertContent.Store.KnowledgeStore.Create", i18n.ERROR_INTERNAL, err)
	}

	knowledge.Content = content
	if isSync {
		if err = l.processKnowledgeAsync(knowledge); err != nil {
			return knowledgeID, errors.Trace("KnowledgeLogic.InsertContent", err)
		}
	} else {
		go safe.Run(func() {
			if err = l.processKnowledgeAsync(knowledge); err != nil {
				slog.Error("Process knowledge async failed",
					slog.String("space_id", knowledge.SpaceID),
					slog.String("knowledge_id", knowledge.ID),
					slog.Any("error", err))
			}
		})
	}

	if contentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS {
		go safe.Run(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			if err := UpdateFilesUploaded(ctx, l.core, spaceID, content); err != nil {
				slog.Error("Failed to update files uploaded status", slog.String("space_id", spaceID), slog.Any("error", err))
			}
		})
	}

	return knowledgeID, nil
}

const (
	InserTypeSync  = true
	InserTypeAsync = false
)

func (l *KnowledgeLogic) InsertContentAsync(spaceID, resource string, kind types.KnowledgeKind, content types.KnowledgeContent, contentType types.KnowledgeContentType) (string, error) {
	return l.insertContent(InserTypeAsync, spaceID, resource, kind, content, contentType)
}

func (l *KnowledgeLogic) InsertContent(spaceID, resource string, kind types.KnowledgeKind, content types.KnowledgeContent, contentType types.KnowledgeContentType) (string, error) {
	return l.insertContent(InserTypeSync, spaceID, resource, kind, content, contentType)
	// sw := mark.NewSensitiveWork()
	// content = sw.Do(content)

	// // flow start
	// summary, err := l.core.Srv().AI().Summarize(l.ctx, &content)
	// if err != nil {
	// 	return knowledgeID, errors.New("KnowledgeLogic.AI.Summarize", i18n.ERROR_INTERNAL, err)
	// }

	// slog.Debug("knowledge summary result", slog.Any("result", summary))

	// if summary.DateTime == "" {
	// 	summary.DateTime = knowledge.MaybeDate
	// }

	// if summary.Summary == "" {
	// 	summary.Summary = content
	// }

	// embeddingContent := summary.Summary
	// summary.Summary = sw.Undo(summary.Summary)
	// summary.Summary = summary.Title + "\n" + summary.Summary

	// if err = l.core.Store().KnowledgeStore().FinishedStageSummarize(l.ctx, spaceID, knowledgeID, summary); err != nil {
	// 	return knowledgeID, errors.New("KnowledgeLogic.KnowledgeStore.FinishedStageSummarize", i18n.ERROR_INTERNAL, err)
	// }

	// vector, err := l.core.Srv().AI().EmbeddingForDocument(l.ctx, "", embeddingContent)
	// if err != nil {
	// 	return knowledgeID, errors.New("KnowledgeLogic.AI.EmbeddingForDocument", i18n.ERROR_INTERNAL, err)
	// }

	// err = l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
	// 	err := l.core.Store().VectorStore().Create(ctx, types.Vector{
	// 		ID:        knowledgeID,
	// 		SpaceID:   spaceID,
	// 		UserID:    user.User,
	// 		Embedding: pgvector.NewVector(vector),
	// 		Resource:  resource,
	// 	})
	// 	if err != nil {
	// 		return errors.New("KnowledgeLogic.VectorStore.Create", i18n.ERROR_INTERNAL, err)
	// 	}

	// 	if err = l.core.Store().KnowledgeStore().FinishedStageEmbedding(ctx, spaceID, knowledgeID); err != nil {
	// 		return errors.New("KnowledgeLogic.KnowledgeStore.FinishedStageEmbedding", i18n.ERROR_INTERNAL, err)
	// 	}

	// 	return nil
	// })
	// return knowledgeID, err
}

func (l *KnowledgeLogic) processKnowledgeAsync(knowledge types.Knowledge) error {
	ctx, cancel := context.WithTimeout(l.ctx, time.Minute*2)
	defer cancel()
	respChan := process.NewSummaryRequest(knowledge)
	if respChan == nil {
		return errors.New("KnowledgeLogic.processKnowledgeAsync.NewSummaryRequest", i18n.ERROR_INTERNAL, fmt.Errorf("unexpected, process wrong"))
	}
	select {
	case <-ctx.Done():
		return errors.New("KnowledgeLogic.processKnowledgeAsync.Summary.ctx", i18n.ERROR_INTERNAL, ctx.Err())
	case req := <-respChan:
		if req.Err != nil {
			return errors.New("KnowledgeLogic.processKnowledgeAsync.Summary.Result", i18n.ERROR_INTERNAL, req.Err)
		}
	}

	{
		knowledge, err := l.core.Store().KnowledgeStore().GetKnowledge(ctx, knowledge.SpaceID, knowledge.ID)
		if err != nil {
			return errors.New("KnowledgeLogic.processKnowledgeAsync.KnowledgeStore.GetKnowledge", i18n.ERROR_INTERNAL, err)
		}

		ctx, cancel := context.WithTimeout(l.ctx, time.Minute*2)
		defer cancel()
		respChan := process.NewEmbeddingRequest(*knowledge)
		if respChan == nil {
			return errors.New("KnowledgeLogic.processKnowledgeAsync.NewEmbeddingRequest", i18n.ERROR_INTERNAL, fmt.Errorf("unexpected, process wrong"))
		}
		select {
		case <-ctx.Done():
			return errors.New("KnowledgeLogic.processKnowledgeAsync.Embedding.ctx", i18n.ERROR_INTERNAL, ctx.Err())
		case req := <-respChan:
			if req.Err != nil {
				return errors.New("KnowledgeLogic.processKnowledgeAsync.Embedding.Result", i18n.ERROR_INTERNAL, req.Err)
			}
		}
	}

	return nil
}
