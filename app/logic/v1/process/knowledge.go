package process

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/holdno/firetower/protocol"
	"github.com/pgvector/pgvector-go"
	"github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/pkg/mark"
	"github.com/quka-ai/quka-ai/pkg/safe"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

var (
	knowledgeProcess *KnowledgeProcess
)

func StartKnowledgeProcess(core *core.Core, concurrency int) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	knowledgeProcess = &KnowledgeProcess{
		concurrency:              concurrency,
		ctx:                      ctx,
		core:                     core,
		SummaryChan:              make(chan *SummaryRequest, 1000),
		EmbeddingChan:            make(chan *EmbeddingRequest, 1000),
		RecordUsageChan:          make(chan *RecordUsageRequest, 100),
		RecordChatUsageChan:      make(chan *RecordChatUsageRequest, 10000),
		RecordSessionUsageChan:   make(chan *RecordSessionUsageRequest, 100),
		RecordKnowledgeUsageChan: make(chan *RecordKnowledgeUsageRequest, 10000),
		processingMap:            make(map[string]struct{}),
	}

	go safe.Run(knowledgeProcess.Start)
	go safe.Run(func() {
		knowledgeProcess.Flush()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				knowledgeProcess.Flush()
			}
		}
	})
	return cancel
}

func (p *KnowledgeProcess) Flush() {
	ctx, cancel := context.WithTimeout(p.ctx, time.Second*10)
	defer cancel()
	list, err := p.core.Store().KnowledgeStore().ListProcessingKnowledges(ctx, 3, 1, 20)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("Failed to list processing knowledges", slog.String("error", err.Error()))
		return
	}

	if len(list) > 0 {
		slog.Info("KnowledgeProcess flush", slog.Int("length", len(list)))
	}

	for _, v := range list {
		if v.RetryTimes > 3 {
			continue
		}

		if v.Content, err = p.core.DecryptData(v.Content); err != nil {
			slog.Error("Failed to decrypt knowledge content", slog.String("knowledge_id", v.ID), slog.String("error", err.Error()))
			continue
		}

		switch v.Stage {
		case types.KNOWLEDGE_STAGE_SUMMARIZE:
			NewSummaryRequest(v)
		case types.KNOWLEDGE_STAGE_EMBEDDING:
			NewEmbeddingRequest(v)
		}
	}
}

type KnowledgeProcess struct {
	concurrency              int
	ctx                      context.Context
	core                     *core.Core
	SummaryChan              chan *SummaryRequest
	EmbeddingChan            chan *EmbeddingRequest
	RecordUsageChan          chan *RecordUsageRequest
	RecordChatUsageChan      chan *RecordChatUsageRequest
	RecordSessionUsageChan   chan *RecordSessionUsageRequest
	RecordKnowledgeUsageChan chan *RecordKnowledgeUsageRequest
	mu                       sync.Mutex
	processingMap            map[string]struct{}
}

func (p *KnowledgeProcess) Start() {
	for range p.concurrency {
		go safe.Run(func() {
			p.ProcessSummary()
		})
	}
	for range p.concurrency {
		go safe.Run(func() {
			p.ProcessEmbedding()
		})
	}
	for range p.concurrency {
		go safe.Run(func() {
			p.ProcessUsage()
		})
	}
}

type SummaryRequest struct {
	ctx      context.Context
	data     *types.Knowledge
	response chan SummaryResponse
}

type SummaryResponse struct {
	Err error
}

func (p *KnowledgeProcess) CheckProcess(id string, handler func()) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	ok, err := p.core.TryLock(ctx, fmt.Sprintf("knowledge:process:%s", id))
	if err != nil {
		slog.Error("Failed to lock knowledge process", slog.String("key", fmt.Sprintf("knowledge:process:%s", id)))
		return
	}

	if !ok {
		return
	}

	handler()
}

func NewSummaryRequest(data types.Knowledge) chan SummaryResponse {
	if knowledgeProcess == nil || knowledgeProcess.ctx.Err() != nil {
		slog.Error("Knowledge Process not working", slog.String("error", knowledgeProcess.ctx.Err().Error()),
			slog.String("space_id", data.SpaceID), slog.String("knowledge_id", data.ID))
		return nil
	}

	resp := make(chan SummaryResponse, 1)
	knowledgeProcess.SummaryChan <- &SummaryRequest{
		ctx:      context.Background(),
		data:     &data,
		response: resp,
	}
	return resp
}

func NewEmbeddingRequest(data types.Knowledge) chan EmbeddingResponse {
	if knowledgeProcess == nil || knowledgeProcess.ctx.Err() != nil {
		slog.Error("Knowledge Process not working", slog.String("error", knowledgeProcess.ctx.Err().Error()),
			slog.String("space_id", data.SpaceID), slog.String("knowledge_id", data.ID))
		return nil
	}

	resp := make(chan EmbeddingResponse, 1)
	knowledgeProcess.EmbeddingChan <- &EmbeddingRequest{
		ctx:      context.Background(),
		data:     &data,
		response: resp,
	}
	return resp
}

func (p *KnowledgeProcess) ProcessSummary() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case req := <-p.SummaryChan:
			if req == nil {
				continue
			}

			p.CheckProcess(req.data.ID, func() {
				p.processSummary(req)
			})
		}
	}
}

func (p *KnowledgeProcess) ProcessEmbedding() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case req := <-p.EmbeddingChan:
			if req == nil {
				continue
			}

			p.CheckProcess(req.data.ID, func() {
				p.processEmbedding(req)
			})
		}
	}
}

type EmbeddingRequest struct {
	ctx      context.Context
	data     *types.Knowledge
	response chan EmbeddingResponse
}

type EmbeddingResponse struct {
	Err error
}

func (p *KnowledgeProcess) processEmbedding(req *EmbeddingRequest) {
	logAttrs := []any{
		slog.String("space_id", req.data.SpaceID),
		slog.String("knowledge_id", req.data.ID),
		slog.String("component", "KnowledgeProcess.processEmbedding"),
	}

	ctx, cancel := context.WithTimeout(req.ctx, time.Minute*5)
	defer cancel()
	knowledge, err := p.core.Store().KnowledgeStore().GetKnowledge(ctx, req.data.SpaceID, req.data.ID)
	if err != nil {
		slog.Error("Failed to process knowledge summary", append(logAttrs, slog.String("error", err.Error()))...)
		return
	}

	if knowledge.Stage != types.KNOWLEDGE_STAGE_EMBEDDING {
		return
	}

	slog.Info("Receive new embedding request",
		logAttrs...)

	defer func() {
		slog.Info("Embedding finished",
			logAttrs...)
		if req.response != nil {
			req.response <- EmbeddingResponse{
				Err: err,
			}
			close(req.response)
		}
		if err != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			if err = p.core.Store().KnowledgeStore().SetRetryTimes(ctx, req.data.SpaceID, req.data.ID, req.data.RetryTimes+1); err != nil {
				slog.Error("Failed to set knowledge process retry times", slog.String("stage", types.KNOWLEDGE_STAGE_EMBEDDING.String()),
					slog.String("error", err.Error()),
					slog.String("space_id", req.data.SpaceID),
					slog.String("knowledge_id", req.data.ID),
					slog.String("component", "KnowledgeProcess.processEmbedding"))
			}
		}
	}()

	// if req.data.Summary == "" {
	// 	err = errors.New("empty summary")
	// 	return
	// }

	sw := mark.NewSensitiveWork()
	// content := sw.Do(req.data.Summary)

	var chunksData []types.KnowledgeChunk

	if req.data.Kind == types.KNOWLEDGE_KIND_CHUNK {
		// type KnowledgeChunk struct {
		// 	ID             string `json:"id" db:"id"`                           // 主键，字符串类型
		// 	KnowledgeID    string `json:"knowledge_id" db:"knowledge_id"`       // 知识点ID
		// 	SpaceID        string `json:"space_id" db:"space_id"`               // 空间ID
		// 	UserID         string `json:"user_id" db:"user_id"`                 // 用户ID
		// 	Chunk          string `json:"chunk" db:"chunk"`                     // 知识片段
		// 	OriginalLength int    `json:"original_length" db:"original_length"` // 原文长度
		// 	UpdatedAt      int64  `json:"updated_at" db:"updated_at"`           // 更新时间
		// 	CreatedAt      int64  `json:"created_at" db:"created_at"`           // 创建时间
		// }

		markdownContent := string(req.data.Content)
		if req.data.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS {
			markdownContent, err = utils.ConvertEditorJSBlocksToMarkdown(json.RawMessage(req.data.Content))
			if err != nil {
				slog.Error("Failed to convert editor blocks to markdown", append(logAttrs, slog.String("error", err.Error()))...)
				return
			}
		}
		chunksData = append(chunksData, types.KnowledgeChunk{
			ID:             req.data.ID,
			KnowledgeID:    req.data.ID,
			SpaceID:        req.data.SpaceID,
			UserID:         req.data.UserID,
			Chunk:          markdownContent,
			OriginalLength: len([]rune(markdownContent)),
			UpdatedAt:      time.Now().Unix(),
			CreatedAt:      time.Now().Unix(),
		})
	} else {
		chunksData, err = p.core.Store().KnowledgeChunkStore().List(ctx, req.data.SpaceID, req.data.ID)
		if err != nil {
			slog.Error("Failed to list knowledge chuns", append(logAttrs, slog.String("error", err.Error()))...)
			return
		}
	}

	var (
		vectors []types.Vector
		chunks  []string
	)
	for _, v := range chunksData {
		decryptData, err := p.core.DecryptData([]byte(v.Chunk))
		if err != nil {
			slog.Error("Failed to decrypt knowledge chunk", slog.String("error", err.Error()))
			continue
		}
		v.Chunk = string(decryptData)
		chunks = append(chunks, sw.Do(v.Chunk))

		vectors = append(vectors, types.Vector{
			ID:             v.ID,
			KnowledgeID:    v.KnowledgeID,
			SpaceID:        v.SpaceID,
			UserID:         v.UserID,
			Resource:       req.data.Resource,
			OriginalLength: v.OriginalLength,
			// Embedding:   pgvector.NewVector(vector),
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		})
	}

	vectorResults, err := p.core.Srv().AI().EmbeddingForDocument(ctx, "", chunks)
	if err != nil {
		slog.Error("Failed to embedding for document", append(logAttrs, slog.String("error", err.Error()))...)
		return
	}

	NewRecordKnowledgeUsageRequest(vectorResults.Model, types.USAGE_SUB_TYPE_EMBEDDING, req.data, vectorResults.Usage)

	if len(vectorResults.Data) != len(vectors) {
		slog.Error("Embedding results count not matched chunks count", append(logAttrs, slog.String("error", "embedding result length not match"))...)
		return
	}

	for i, v := range vectorResults.Data {
		vectors[i].Embedding = pgvector.NewVector(v)
	}

	err = p.core.Store().Transaction(req.ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(req.ctx, time.Minute)
		defer cancel()

		// exist, err := p.core.Store().VectorStore().GetVector(ctx, req.data.SpaceID, req.data.ID)
		// if err != nil && err != sql.ErrNoRows {
		// 	slog.Error("Failed to check the existence of knowledge", append(logAttrs, slog.String("error", err.Error()))...)
		// 	return err
		// }

		// if exist == nil {
		// 	err = p.core.Store().VectorStore().Create(ctx, types.Vector{
		// 		ID:        req.data.ID,
		// 		SpaceID:   req.data.SpaceID,
		// 		UserID:    req.data.UserID,
		// 		Embedding: pgvector.NewVector(vector),
		// 		Resource:  req.data.Resource,
		// 	})
		// 	if err != nil {
		// 		slog.Error("Failed to insert vector data into vector store", append(logAttrs, slog.String("error", err.Error()))...)
		// 		return err
		// 	}
		// } else {
		// 	err = p.core.Store().VectorStore().Update(ctx, req.data.SpaceID, req.data.ID, pgvector.NewVector(vector))
		// 	if err != nil {
		// 		slog.Error("Failed to update vector data", append(logAttrs, slog.String("error", err.Error()))...)
		// 		return err
		// 	}
		// }

		err := p.core.Store().VectorStore().BatchDelete(ctx, req.data.SpaceID, req.data.ID)
		if err != nil && err != sql.ErrNoRows {
			slog.Error("Failed to check the existence of knowledge", append(logAttrs, slog.String("error", err.Error()))...)
			return err
		}

		err = p.core.Store().VectorStore().BatchCreate(ctx, vectors)
		if err != nil {
			slog.Error("Failed to insert vector data into vector store", append(logAttrs, slog.String("error", err.Error()))...)
			return err
		}

		if err = p.core.Store().KnowledgeStore().FinishedStageEmbedding(ctx, req.data.SpaceID, req.data.ID); err != nil {
			slog.Error("Failed to set knowledge finished embedding stage", append(logAttrs, slog.String("error", err.Error()))...)
			return err
		}

		publishStageChangedMessage(p.core.Srv().Tower(), req.data.SpaceID, req.data.ID, types.KNOWLEDGE_STAGE_DONE)
		return nil
	})
}

func (p *KnowledgeProcess) processSummary(req *SummaryRequest) {
	logAttrs := []any{
		slog.String("space_id", req.data.SpaceID),
		slog.String("knowledge_id", req.data.ID),
		slog.String("component", "KnowledgeProcess.processSummary"),
	}

	ctx, cancel := context.WithTimeout(req.ctx, time.Minute*5)
	defer cancel()
	knowledge, err := p.core.Store().KnowledgeStore().GetKnowledge(ctx, req.data.SpaceID, req.data.ID)
	if err != nil {
		slog.Error("Failed to process knowledge summary", append(logAttrs, slog.String("error", err.Error()))...)
		return
	}

	if knowledge.Stage != types.KNOWLEDGE_STAGE_SUMMARIZE {
		return
	}

	slog.Info("Receive new summary request",
		logAttrs...)

	defer func() {
		slog.Info("Summary finished",
			logAttrs...)
		if req.response != nil {
			req.response <- SummaryResponse{
				Err: err,
			}
			close(req.response)
		}
		if err != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			if err = p.core.Store().KnowledgeStore().SetRetryTimes(ctx, req.data.SpaceID, req.data.ID, req.data.RetryTimes+1); err != nil {
				slog.Error("Failed to set knowledge process retry times",
					append(logAttrs,
						slog.String("stage", types.KNOWLEDGE_STAGE_SUMMARIZE.String()),
						slog.String("error", err.Error()))...)
			}
		}
	}()

	sw := mark.NewSensitiveWork()
	markdownContent := string(req.data.Content)
	if req.data.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS {
		markdownContent, err = utils.ConvertEditorJSBlocksToMarkdown(json.RawMessage(req.data.Content))
		if err != nil {
			slog.Error("Failed to convert editor blocks to markdown", append(logAttrs, slog.String("error", err.Error()))...)
			return
		}
	}

	secretContent := sw.Do(markdownContent)

	summary, err := p.core.Srv().AI().Chunk(ctx, &secretContent)
	if err != nil {
		slog.Error("Failed to summarize knowledge", append(logAttrs, slog.String("error", err.Error()))...)
		return
	}

	NewRecordKnowledgeUsageRequest(summary.Model, types.USAGE_SUB_TYPE_SUMMARY, req.data, summary.Usage)

	slog.Debug("Knowledge summary result", slog.String("knowledge_id", req.data.ID), slog.String("space_id", req.data.SpaceID), slog.Any("result", summary))

	if summary.DateTime == "" {
		summary.DateTime = req.data.MaybeDate
	}

	if len(summary.Chunks) == 0 {
		summary.Chunks = append(summary.Chunks, markdownContent)
	}

	originalLenght := len([]rune(markdownContent))
	var chunks []*types.KnowledgeChunk
	for _, v := range summary.Chunks {
		chunks = append(chunks, &types.KnowledgeChunk{
			ID:             utils.GenRandomID(),
			SpaceID:        req.data.SpaceID,
			KnowledgeID:    req.data.ID,
			UserID:         req.data.UserID,
			Chunk:          sw.Undo(v),
			OriginalLength: originalLenght,
			UpdatedAt:      time.Now().Unix(),
			CreatedAt:      time.Now().Unix(),
		})
	}

	if req.data.Summary != "" {
		needToUpdate := make(map[string]bool)
		for _, v := range strings.Split(req.data.Summary, ",") {
			needToUpdate[v] = true
		}

		if !needToUpdate["title"] {
			summary.Title = ""
		}
		if !needToUpdate["tags"] {
			summary.Tags = nil
		}
		if !needToUpdate["content"] {
			chunks = nil
		}
	}

	// if summary.Title != "" {
	// 	summary.Summary = fmt.Sprintf("%s\n%s\n%s", summary.Title, strings.Join(summary.Tags, ","), summary.Summary)
	// }

	p.core.Store().Transaction(req.ctx, func(ctx context.Context) error {
		if len(chunks) > 0 {
			if err = p.core.Store().KnowledgeChunkStore().BatchDelete(req.ctx, req.data.SpaceID, req.data.ID); err != nil {
				slog.Error("Failed to pre-delete knowledge chunks", append(logAttrs, slog.String("error", err.Error()))...)
				return err
			}

			for _, v := range chunks {
				encryptData, err := p.core.EncryptData([]byte(v.Chunk))
				if err != nil {
					slog.Error("Failed to encrypt knowledge chunk content", slog.String("error", err.Error()), slog.String("id", v.ID))
					return err
				}

				v.Chunk = string(encryptData)
			}

			if err = p.core.Store().KnowledgeChunkStore().BatchCreate(req.ctx, chunks); err != nil {
				slog.Error("Failed to create knowledge chunks", append(logAttrs, slog.String("error", err.Error()))...)
				return err
			}
		}

		if err = p.core.Store().KnowledgeStore().FinishedStageSummarize(req.ctx, req.data.SpaceID, req.data.ID, summary); err != nil {
			slog.Error("Failed to set finished summary stage", append(logAttrs, slog.String("error", err.Error()))...)
			return err
		}

		publishStageChangedMessage(p.core.Srv().Tower(), req.data.SpaceID, req.data.ID, types.KNOWLEDGE_STAGE_EMBEDDING)
		return nil
	})
}

func publishStageChangedMessage(tower *srv.Tower, spaceID, knowledgeID string, stage types.KnowledgeStage) {
	fire := tower.NewFire(protocol.SourceSystem, tower.Pusher())
	fire.Message = protocol.TopicMessage[srv.PublishData]{
		Topic: "/knowledge/list/" + spaceID,
		Type:  protocol.PublishOperation,
		Data: srv.PublishData{
			Version: "v1",
			Subject: "stage_changed",
			Data: map[string]string{
				"knowledge_id": knowledgeID,
				"stage":        stage.String(),
			},
		},
	}

	tower.Publish(fire)
}

type RecordSessionUsageRequest struct {
	ctx       context.Context
	model     string
	spaceID   string
	sessionID string
	subType   string
	usage     *openai.Usage
	response  chan CommonProcessResponse
}

type RecordUsageRequest struct {
	ctx      context.Context
	spaceID  string
	userID   string
	model    string
	subType  string
	_type    string
	usage    *openai.Usage
	response chan CommonProcessResponse
}

type RecordChatUsageRequest struct {
	ctx       context.Context
	model     string
	messageID string
	subType   string
	usage     *openai.Usage
	response  chan CommonProcessResponse
}

type RecordKnowledgeUsageRequest struct {
	ctx       context.Context
	model     string
	knowledge *types.Knowledge
	subType   string
	usage     *openai.Usage
	response  chan CommonProcessResponse
}

type CommonProcessResponse struct {
	Error error
}

func NewRecordUsageRequest(model, _type, subType, spaceID, userID string, usage *openai.Usage) chan CommonProcessResponse {
	if knowledgeProcess == nil || knowledgeProcess.ctx.Err() != nil {
		return nil
	}
	resp := make(chan CommonProcessResponse, 1)
	knowledgeProcess.RecordUsageChan <- &RecordUsageRequest{
		ctx:      context.Background(),
		model:    model,
		spaceID:  spaceID,
		userID:   userID,
		_type:    _type,
		subType:  subType,
		usage:    usage,
		response: resp,
	}
	return resp
}

func NewRecordChatUsageRequest(model, subType, messageID string, usage *openai.Usage) chan CommonProcessResponse {
	if knowledgeProcess == nil || knowledgeProcess.ctx.Err() != nil {
		slog.Error("Knowledge Process not working", slog.String("message", messageID), slog.Any("usage", usage))
		return nil
	}

	resp := make(chan CommonProcessResponse, 1)
	knowledgeProcess.RecordChatUsageChan <- &RecordChatUsageRequest{
		ctx:       context.Background(),
		model:     model,
		messageID: messageID,
		subType:   subType,
		usage:     usage,
		response:  resp,
	}
	return resp
}

func NewRecordSessionUsageRequest(model, subType, spaceID, sessionID string, usage *openai.Usage) chan CommonProcessResponse {
	if knowledgeProcess == nil || knowledgeProcess.ctx.Err() != nil {
		slog.Error("Knowledge Process not working", slog.String("error", knowledgeProcess.ctx.Err().Error()),
			slog.String("session_id", sessionID), slog.Any("usage", usage))
		return nil
	}

	resp := make(chan CommonProcessResponse, 1)
	knowledgeProcess.RecordSessionUsageChan <- &RecordSessionUsageRequest{
		ctx:       context.Background(),
		model:     model,
		spaceID:   spaceID,
		sessionID: sessionID,
		subType:   subType,
		usage:     usage,
		response:  resp,
	}
	return resp
}

func NewRecordKnowledgeUsageRequest(model, subType string, knowledge *types.Knowledge, usage *openai.Usage) chan CommonProcessResponse {
	if knowledgeProcess == nil || knowledgeProcess.ctx.Err() != nil {
		slog.Error("Knowledge Process not working", slog.String("error", knowledgeProcess.ctx.Err().Error()),
			slog.String("space_id", knowledge.SpaceID), slog.String("knowledge_id", knowledge.ID))
		return nil
	}

	resp := make(chan CommonProcessResponse, 1)
	knowledgeProcess.RecordKnowledgeUsageChan <- &RecordKnowledgeUsageRequest{
		ctx:       context.Background(),
		model:     model,
		subType:   subType,
		knowledge: knowledge,
		usage:     usage,
		response:  resp,
	}
	return resp
}

func (p *KnowledgeProcess) ProcessUsage() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case req := <-p.RecordUsageChan:
			if req == nil {
				continue
			}
			req.response <- CommonProcessResponse{
				Error: p.RecordUsage(req),
			}
		case req := <-p.RecordSessionUsageChan:
			if req == nil {
				continue
			}

			p.CheckProcess(fmt.Sprintf("session_%s_%s_usage", req.sessionID, req.subType), func() {
				req.response <- CommonProcessResponse{
					Error: p.RecordSessionUsage(req),
				}
			})
		case req := <-p.RecordChatUsageChan:
			if req == nil {
				continue
			}

			p.CheckProcess(fmt.Sprintf("message_%s_%s_usage", req.messageID, req.subType), func() {
				req.response <- CommonProcessResponse{
					Error: p.RecordChatUsage(req),
				}
			})
		case req := <-p.RecordKnowledgeUsageChan:
			if req == nil {
				continue
			}

			p.CheckProcess(fmt.Sprintf("knowledge_%s_usage_%s", req.knowledge.ID, req.subType), func() {
				req.response <- CommonProcessResponse{
					Error: p.RecordKnowledgeUsage(req),
				}
			})
		}
	}
}

func (p *KnowledgeProcess) RecordUsage(req *RecordUsageRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err := p.core.Store().AITokenUsageStore().Create(ctx, types.AITokenUsage{
		SpaceID:     req.spaceID,
		UserID:      req.userID,
		Type:        req._type,
		SubType:     req.subType,
		ObjectID:    "",
		Model:       req.model,
		UsagePrompt: req.usage.PromptTokens,
		UsageOutput: req.usage.CompletionTokens,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		slog.Error("Process RecordUsage failed", slog.String("error", err.Error()),
			slog.String("space_id", req.spaceID), slog.String("user_id", req.userID), slog.Any("usage", req.usage))
		return err
	}
	return nil
}

func (p *KnowledgeProcess) RecordSessionUsage(req *RecordSessionUsageRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	session, err := p.core.Store().ChatSessionStore().GetChatSession(ctx, req.spaceID, req.sessionID)
	if err != nil {
		return err
	}

	err = p.core.Store().AITokenUsageStore().Create(ctx, types.AITokenUsage{
		SpaceID:     session.SpaceID,
		UserID:      session.UserID,
		Type:        types.USAGE_TYPE_CHAT,
		SubType:     req.subType,
		ObjectID:    session.ID,
		Model:       req.model,
		UsagePrompt: req.usage.PromptTokens,
		UsageOutput: req.usage.CompletionTokens,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		slog.Error("Process RecordSessionUsage failed", slog.String("error", err.Error()),
			slog.String("space_id", session.SpaceID), slog.String("session_id", session.ID), slog.String("user_id", session.UserID), slog.Any("usage", req.usage))
		return err
	}
	return nil
}

func (p *KnowledgeProcess) RecordKnowledgeUsage(req *RecordKnowledgeUsageRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err := p.core.Store().AITokenUsageStore().Create(ctx, types.AITokenUsage{
		SpaceID:     req.knowledge.SpaceID,
		UserID:      req.knowledge.UserID,
		Type:        types.USAGE_TYPE_KNOWLEDGE,
		SubType:     req.subType,
		ObjectID:    req.knowledge.ID,
		Model:       req.model,
		UsagePrompt: req.usage.PromptTokens,
		UsageOutput: req.usage.CompletionTokens,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		slog.Error("Process RecordKnowledgeUsage failed", slog.String("error", err.Error()),
			slog.String("space_id", req.knowledge.SpaceID), slog.String("knowledge_id", req.knowledge.ID), slog.Any("usage", req.usage))
		return err
	}
	return nil
}

func (p *KnowledgeProcess) RecordChatUsage(req *RecordChatUsageRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	relMessage, err := p.core.Store().ChatMessageStore().GetOne(ctx, req.messageID)
	if err != nil {
		return err
	}

	err = p.core.Store().AITokenUsageStore().Create(ctx, types.AITokenUsage{
		SpaceID:     relMessage.SpaceID,
		UserID:      relMessage.UserID,
		Type:        types.USAGE_TYPE_CHAT,
		SubType:     req.subType,
		ObjectID:    req.messageID,
		Model:       req.model,
		UsagePrompt: req.usage.PromptTokens,
		UsageOutput: req.usage.CompletionTokens,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		slog.Error("Process RecordKnowledgeUsage failed", slog.String("error", err.Error()),
			slog.String("space_id", relMessage.SpaceID), slog.String("message_id", relMessage.ID), slog.String("user_id", relMessage.UserID), slog.Any("usage", req.usage))
		return err
	}
	return nil
}
