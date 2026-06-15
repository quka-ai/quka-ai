package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type RememberMemoryRequest struct {
	KnowledgeID     string                      `json:"knowledge_id"`
	Resource        string                      `json:"resource"`
	Title           string                      `json:"title"`
	Content         types.KnowledgeContent      `json:"content"`
	ContentType     types.KnowledgeContentType  `json:"content_type"`
	Kind            types.KnowledgeKind         `json:"kind"`
	Layer           types.MemoryLayer           `json:"layer"`
	MemoryType      types.MemoryType            `json:"memory_type"`
	Scope           types.MemoryScope           `json:"scope"`
	AuthorType      types.MemoryAuthorType      `json:"author_type"`
	EpistemicStatus types.MemoryEpistemicStatus `json:"epistemic_status"`
	EntityKey       string                      `json:"entity_key"`
	Importance      int16                       `json:"importance"`
	Confidence      float64                     `json:"confidence"`
	SourceKind      types.MemorySourceKind      `json:"source_kind"`
	SourceRef       string                      `json:"source_ref"`
}

type RememberMemoryResponse struct {
	MemoryID    string `json:"memory_id"`
	KnowledgeID string `json:"knowledge_id"`
}

func (s *HttpSrv) RememberMemory(c *gin.Context) {
	var req RememberMemoryRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}
	if req.KnowledgeID == "" && len(req.Content) == 0 {
		response.APIError(c, errors.New("RememberMemory.InvalidArgument", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest))
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	targetSpaceID, scope := v1.ResolveMemoryLayer(spaceID, req.Layer, req.Scope)
	memoryID, knowledgeID, err := v1.NewMemoryLogic(c, s.Core).Remember(targetSpaceID, v1.RememberMemoryArgs{
		KnowledgeID:     req.KnowledgeID,
		Resource:        req.Resource,
		Title:           req.Title,
		Content:         req.Content,
		ContentType:     req.ContentType,
		Kind:            req.Kind,
		MemoryType:      req.MemoryType,
		Scope:           scope,
		AuthorType:      req.AuthorType,
		EpistemicStatus: req.EpistemicStatus,
		EntityKey:       req.EntityKey,
		Importance:      req.Importance,
		Confidence:      req.Confidence,
		SourceKind:      req.SourceKind,
		SourceRef:       req.SourceRef,
	})
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, RememberMemoryResponse{MemoryID: memoryID, KnowledgeID: knowledgeID})
}

type RecallMemoryRequest struct {
	Query           string              `json:"query"`
	Limit           int                 `json:"limit"`
	Scopes          []types.MemoryScope `json:"scopes"`
	MemoryTypes     []types.MemoryType  `json:"memory_types"`
	EntityKeys      []string            `json:"entity_keys"`
	IncludeEvidence bool                `json:"include_evidence"`
}

type RecallMemoryItem struct {
	MemoryID        string   `json:"memory_id"`
	KnowledgeID     string   `json:"knowledge_id"`
	SpaceID         string   `json:"space_id"`
	Layer           string   `json:"layer"`
	Scope           string   `json:"scope"`
	MemoryType      string   `json:"memory_type"`
	Title           string   `json:"title"`
	Content         string   `json:"content"`
	EntityKey       string   `json:"entity_key,omitempty"`
	Confidence      float64  `json:"confidence"`
	Importance      int16    `json:"importance"`
	AuthorType      string   `json:"author_type"`
	EpistemicStatus string   `json:"epistemic_status"`
	SourceKind      string   `json:"source_kind"`
	SourceRef       string   `json:"source_ref,omitempty"`
	Evidence        []string `json:"evidence,omitempty"`
}

type RecallMemoryResponse struct {
	Items []RecallMemoryItem `json:"items"`
	Count int                `json:"count"`
}

func recallMemoryItemToResponse(item v1.MemoryRecallItem) RecallMemoryItem {
	return RecallMemoryItem{
		MemoryID:        item.Memory.ID,
		KnowledgeID:     item.Knowledge.ID,
		SpaceID:         item.Memory.SpaceID,
		Layer:           v1.MemoryLayerOf(item.Memory).String(),
		Scope:           item.Memory.Scope.String(),
		MemoryType:      item.Memory.MemoryType.String(),
		Title:           item.Knowledge.Title,
		Content:         item.Knowledge.Content.String(),
		EntityKey:       item.Memory.EntityKey,
		Confidence:      item.Memory.Confidence,
		Importance:      item.Memory.Importance,
		AuthorType:      item.Memory.AuthorType.String(),
		EpistemicStatus: item.Memory.EpistemicStatus.String(),
		SourceKind:      item.Memory.SourceKind.String(),
		SourceRef:       item.Memory.SourceRef,
		Evidence:        item.Evidence,
	}
}

func (s *HttpSrv) RecallMemory(c *gin.Context) {
	var req RecallMemoryRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	items, err := v1.NewMemoryLogic(c, s.Core).Recall(spaceID, v1.RecallMemoryArgs{
		Query:           req.Query,
		Limit:           req.Limit,
		Scopes:          req.Scopes,
		MemoryTypes:     req.MemoryTypes,
		EntityKeys:      req.EntityKeys,
		IncludeEvidence: req.IncludeEvidence,
	})
	if err != nil {
		response.APIError(c, err)
		return
	}

	respItems := make([]RecallMemoryItem, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, recallMemoryItemToResponse(item))
	}
	response.APISuccess(c, RecallMemoryResponse{Items: respItems, Count: len(respItems)})
}

type GetMemoryRequest struct {
	ID       string `json:"id" form:"id"`
	MemoryID string `json:"memory_id" form:"memory_id"`
}

func (s *HttpSrv) GetMemory(c *gin.Context) {
	var req GetMemoryRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}
	if req.MemoryID == "" {
		req.MemoryID = req.ID
	}
	if req.MemoryID == "" {
		response.APIError(c, errors.New("GetMemory.InvalidArgument", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest))
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	item, err := v1.NewMemoryLogic(c, s.Core).Get(spaceID, req.MemoryID)
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, recallMemoryItemToResponse(*item))
}

type HydrateMemoryRequest struct {
	RuntimeContext *types.RuntimeContext `json:"runtime_context"`
	Query          string                `json:"query"`
	TokenBudget    int                   `json:"token_budget"`
}

func (s *HttpSrv) HydrateMemory(c *gin.Context) {
	var req HydrateMemoryRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	result, err := v1.NewMemoryLogic(c, s.Core).Hydrate(spaceID, v1.HydrateMemoryArgs{
		RuntimeContext: req.RuntimeContext,
		Query:          req.Query,
		TokenBudget:    req.TokenBudget,
	})
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, result)
}

type PinMemoryRequest struct {
	RuntimeContext types.RuntimeContext    `json:"runtime_context" binding:"required"`
	BindingType    types.MemoryBindingType `json:"binding_type"`
	MemoryIDs      []string                `json:"memory_ids" binding:"required"`
	PinnedBy       types.MemoryPinnedBy    `json:"pinned_by"`
}

func (s *HttpSrv) PinMemory(c *gin.Context) {
	var req PinMemoryRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}
	if req.BindingType == "" {
		req.BindingType = types.MEMORY_BINDING_PIN
	}

	spaceID, _ := v1.InjectSpaceID(c)
	if err := v1.NewMemoryLogic(c, s.Core).Pin(spaceID, v1.PinMemoryArgs{
		RuntimeContext: req.RuntimeContext,
		BindingType:    req.BindingType,
		MemoryIDs:      req.MemoryIDs,
		PinnedBy:       req.PinnedBy,
	}); err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type UpdateMemoryRequest struct {
	ID              string                      `json:"id" binding:"required"`
	Status          types.MemoryStatus          `json:"status"`
	Title           *string                     `json:"title"`
	Content         *types.KnowledgeContent     `json:"content"`
	ContentType     types.KnowledgeContentType  `json:"content_type"`
	Importance      *int16                      `json:"importance"`
	Confidence      *float64                    `json:"confidence"`
	AuthorType      types.MemoryAuthorType      `json:"author_type"`
	EpistemicStatus types.MemoryEpistemicStatus `json:"epistemic_status"`
	EntityKey       *string                     `json:"entity_key"`
	DedupeKey       *string                     `json:"dedupe_key"`
	ConflictState   types.MemoryConflictState   `json:"conflict_state"`
	ValidFrom       *int64                      `json:"valid_from"`
	ValidTo         *int64                      `json:"valid_to"`
}

func (s *HttpSrv) UpdateMemory(c *gin.Context) {
	var req UpdateMemoryRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}
	spaceID, _ := v1.InjectSpaceID(c)
	if err := v1.NewMemoryLogic(c, s.Core).Update(spaceID, req.ID, types.UpdateMemoryArgs{
		Status:          req.Status,
		Title:           req.Title,
		Content:         req.Content,
		ContentType:     req.ContentType,
		Importance:      req.Importance,
		Confidence:      req.Confidence,
		AuthorType:      req.AuthorType,
		EpistemicStatus: req.EpistemicStatus,
		EntityKey:       req.EntityKey,
		DedupeKey:       req.DedupeKey,
		ConflictState:   req.ConflictState,
		ValidFrom:       req.ValidFrom,
		ValidTo:         req.ValidTo,
	}); err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type DeleteMemoryRequest struct {
	ID              string `json:"id" binding:"required"`
	Hard            bool   `json:"hard"`
	DeleteKnowledge bool   `json:"delete_knowledge"`
}

func (s *HttpSrv) DeleteMemory(c *gin.Context) {
	var req DeleteMemoryRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}
	spaceID, _ := v1.InjectSpaceID(c)
	if req.DeleteKnowledge {
		if err := v1.NewMemoryLogic(c, s.Core).Forget(spaceID, req.ID, true); err != nil {
			response.APIError(c, err)
			return
		}
		response.APISuccess(c, nil)
		return
	}
	if err := v1.NewMemoryLogic(c, s.Core).Delete(spaceID, req.ID, req.Hard); err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type ReflectMemoryRequest struct {
	RuntimeContext types.RuntimeContext `json:"runtime_context" binding:"required"`
	FromSequence   int64                `json:"from_sequence"`
	ToSequence     int64                `json:"to_sequence"`
	Mode           string               `json:"mode"`
}

type ReflectMemoryResponse struct {
	MemoryID string `json:"memory_id"`
}

func (s *HttpSrv) ReflectMemory(c *gin.Context) {
	var req ReflectMemoryRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}
	spaceID, _ := v1.InjectSpaceID(c)
	memoryID, err := v1.NewMemoryLogic(c, s.Core).Reflect(spaceID, v1.ReflectMemoryArgs{
		RuntimeContext: req.RuntimeContext,
		FromSequence:   req.FromSequence,
		ToSequence:     req.ToSequence,
		Mode:           req.Mode,
	})
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, ReflectMemoryResponse{MemoryID: memoryID})
}
