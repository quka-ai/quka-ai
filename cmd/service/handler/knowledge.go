package handler

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
)


type UpdateKnowledgeRequest struct {
	ID          string                     `json:"id" binding:"required"`
	Title       string                     `json:"title"`
	Resource    string                     `json:"resource"`
	Content     types.KnowledgeContent     `json:"content"`
	ContentType types.KnowledgeContentType `json:"content_type"`
	Tags        []string                   `json:"tags"`
	Kind        types.KnowledgeKind        `json:"kind"`
}

func (s *HttpSrv) UpdateKnowledge(c *gin.Context) {
	var (
		err error
		req UpdateKnowledgeRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	err = v1.NewKnowledgeLogic(c, s.Core).Update(spaceID, req.ID, types.UpdateKnowledgeArgs{
		Title:       req.Title,
		Content:     req.Content,
		ContentType: req.ContentType,
		Resource:    req.Resource,
		Tags:        req.Tags,
		Kind:        req.Kind,
	})
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type CreateKnowledgeRequest struct {
	Resource    string                     `json:"resource"`
	Content     types.KnowledgeContent     `json:"content" binding:"required"`
	ContentType types.KnowledgeContentType `json:"content_type" binding:"required"`
	Kind        string                     `json:"kind"`
	Async       bool                       `json:"async"`
}

type CreateKnowledgeResponse struct {
	ID string `json:"id"`
}

func (s *HttpSrv) CreateKnowledge(c *gin.Context) {
	var req CreateKnowledgeRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	var handler func(spaceID, resource string, kind types.KnowledgeKind, content types.KnowledgeContent, contentType types.KnowledgeContentType) (string, error)
	logic := v1.NewKnowledgeLogic(c, s.Core)
	if req.Async {
		handler = logic.InsertContentAsync
	} else {
		handler = logic.InsertContent
	}

	id, err := handler(spaceID, req.Resource, types.KindNewFromString(req.Kind), req.Content, req.ContentType)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, CreateKnowledgeResponse{
		ID: id,
	})
}

type GetKnowledgeRequest struct {
	ID          string `json:"id" form:"id" binding:"required"`
	OnlyPreview bool   `json:"only_preview" form:"only_preview"`
}

func (s *HttpSrv) GetKnowledge(c *gin.Context) {
	var (
		err error
		req GetKnowledgeRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	knowledge, err := v1.NewKnowledgeLogic(c, s.Core).GetKnowledge(spaceID, req.ID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	if req.OnlyPreview {
		response.APISuccess(c, KnowledgeToKnowledgeResponseLite(knowledge))
		return
	}

	response.APISuccess(c, KnowledgeToKnowledgeResponse(knowledge))
}

type ListKnowledgeRequest struct {
	Resource string `json:"resource" form:"resource"`
	Keywords string `json:"keywords" form:"keywords"`
	Page     uint64 `json:"page" form:"page" binding:"required"`
	PageSize uint64 `json:"pagesize" form:"pagesize" binding:"required,lte=50"`
}

type ListKnowledgeResponse struct {
	List  []*types.KnowledgeResponse `json:"list"`
	Total uint64                     `json:"total"`
}

func (s *HttpSrv) ListKnowledge(c *gin.Context) {
	var req ListKnowledgeRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	var resource *types.ResourceQuery
	if req.Resource != "" {
		resource = &types.ResourceQuery{
			Include: []string{req.Resource},
		}
	}

	spaceID, _ := v1.InjectSpaceID(c)
	list, total, err := v1.NewKnowledgeLogic(c, s.Core).ListUserKnowledges(spaceID, req.Keywords, resource, req.Page, req.PageSize)
	if err != nil {
		response.APIError(c, err)
		return
	}

	knowledgeList := lo.Map(list, func(item *types.Knowledge, index int) *types.KnowledgeResponse {
		liteContent := KnowledgeToKnowledgeResponseLite(item)
		liteContent.Content = editorjs.ReplaceMarkdownStaticResourcesWithPresignedURL(liteContent.Content, s.Core.Plugins.FileStorage())
		return liteContent
	})

	response.APISuccess(c, ListKnowledgeResponse{
		List:  knowledgeList,
		Total: total,
	})
}

type ListContentTaskRequest struct {
	Page     uint64 `json:"page" form:"page" binding:"required"`
	PageSize uint64 `json:"pagesize" form:"pagesize" binding:"required,lte=50"`
}

type ListContentTaskResponse struct {
	List  []v1.ChunkTaskDetail `json:"list"`
	Total uint64               `json:"total"`
}

func (s *HttpSrv) ListContentTask(c *gin.Context) {
	var req ListContentTaskRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	list, total, err := v1.NewAIFileDisposeLogic(c, s.Core).GetLongContentTaskList(spaceID, req.Page, req.PageSize)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, ListContentTaskResponse{
		List:  list,
		Total: uint64(total),
	})
}

type GetTaskKnowledgeRequest struct {
	TaskID   string `json:"task_id" form:"task_id" binding:"required"`
	Page     uint64 `json:"page" form:"page" binding:"required"`
	PageSize uint64 `json:"pagesize" form:"pagesize" binding:"required,lte=50"`
}

type GetTaskKnowledgeResponse struct {
	List  []*types.KnowledgeResponse `json:"list"`
	Total uint64                     `json:"total"`
}

func (s *HttpSrv) GetTaskKnowledge(c *gin.Context) {
	var req GetTaskKnowledgeRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	list, total, err := v1.NewKnowledgeLogic(c, s.Core).GetTaskKnowledges(spaceID, req.TaskID, req.Page, req.PageSize)
	if err != nil {
		response.APIError(c, err)
		return
	}

	knowledgeList := lo.Map(list, func(item *types.Knowledge, index int) *types.KnowledgeResponse {
		liteContent := KnowledgeToKnowledgeResponseLite(item)
		liteContent.Content = editorjs.ReplaceMarkdownStaticResourcesWithPresignedURL(liteContent.Content, s.Core.Plugins.FileStorage())
		return liteContent
	})

	response.APISuccess(c, GetTaskKnowledgeResponse{
		List:  knowledgeList,
		Total: total,
	})
}

func KnowledgeToKnowledgeResponse(item *types.Knowledge) *types.KnowledgeResponse {
	result := &types.KnowledgeResponse{
		ID:          item.ID,
		SpaceID:     item.SpaceID,
		Title:       item.Title,
		ContentType: item.ContentType,
		Tags:        item.Tags,
		Kind:        item.Kind,
		Resource:    item.Resource,
		UserID:      item.UserID,
		Stage:       item.Stage,
		UpdatedAt:   item.UpdatedAt,
		CreatedAt:   item.CreatedAt,
	}

	if result.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS {
		result.Blocks = json.RawMessage(item.Content)
	} else {
		result.Content = string(item.Content)
	}
	return result
}

func KnowledgeToKnowledgeResponseLite(item *types.Knowledge) *types.KnowledgeResponse {
	result := &types.KnowledgeResponse{
		ID:          item.ID,
		SpaceID:     item.SpaceID,
		Title:       item.Title,
		ContentType: item.ContentType,
		Tags:        item.Tags,
		Kind:        item.Kind,
		Resource:    item.Resource,
		UserID:      item.UserID,
		Stage:       item.Stage,
		UpdatedAt:   item.UpdatedAt,
		CreatedAt:   item.CreatedAt,
	}

	if result.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS {
		blocks, err := editorjs.ParseRawToBlocks(json.RawMessage(item.Content))
		if err != nil {
			slog.Error("Failed to parse editor blocks", slog.String("knowledge_id", item.ID), slog.String("error", err.Error()))
		}

		if len(blocks.Blocks) > 6 {
			blocks.Blocks = blocks.Blocks[:6]
		}

		result.ContentType = types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN
		result.Content, err = editorjs.ConvertEditorJSBlocksToMarkdown(blocks.Blocks)
		if err != nil {
			slog.Error("Failed to convert editor blocks to markdown", slog.String("knowledge_id", item.ID), slog.String("error", err.Error()))
		}
	} else {
		result.Content = string(item.Content)
	}
	return result
}

type DeleteKnowledgeRequest struct {
	ID string `json:"id" binding:"required"`
}

func (s *HttpSrv) DeleteKnowledge(c *gin.Context) {
	var req DeleteKnowledgeRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	if err := v1.NewKnowledgeLogic(c, s.Core).Delete(spaceID, req.ID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type QueryRequest struct {
	Query    string               `json:"query" binding:"required"`
	Agent    string               `json:"agent"`
	Resource *types.ResourceQuery `json:"resource"`
}

func (s *HttpSrv) Query(c *gin.Context) {
	var req QueryRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	// v1.KnowledgeQueryResult
	result, err := v1.NewKnowledgeLogic(c, s.Core).Query(spaceID, req.Agent, req.Resource, req.Query)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, result)
}

type GetDateCreatedKnowledgeRequest struct {
	StartTime int64 `json:"start_time" form:"start_time" binding:"required"`
	EndTime   int64 `json:"end_time" form:"end_time" binding:"required"`
}

func (s *HttpSrv) GetDateCreatedKnowledge(c *gin.Context) {
	var (
		err error
		req GetDateCreatedKnowledgeRequest
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	result, err := v1.NewKnowledgeLogic(c, s.Core).GetTimeRangeLiteKnowledges(spaceID, time.Unix(req.StartTime, 0), time.Unix(req.EndTime, 0))
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, result)
}
