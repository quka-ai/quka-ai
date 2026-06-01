package v1

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
	"github.com/samber/lo"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type MemoryLogic struct {
	UserInfo
	ctx  context.Context
	core *core.Core
}

func NewMemoryLogic(ctx context.Context, core *core.Core) *MemoryLogic {
	return &MemoryLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}
}

type RememberMemoryArgs struct {
	KnowledgeID     string
	Resource        string
	Title           string
	Content         types.KnowledgeContent
	ContentType     types.KnowledgeContentType
	Kind            types.KnowledgeKind
	MemoryType      types.MemoryType
	Scope           types.MemoryScope
	AuthorType      types.MemoryAuthorType
	EpistemicStatus types.MemoryEpistemicStatus
	EntityKey       string
	Importance      int16
	Confidence      float64
	SourceKind      types.MemorySourceKind
	SourceRef       string
}

type RecallMemoryArgs struct {
	Query           string
	Limit           int
	Scopes          []types.MemoryScope
	MemoryTypes     []types.MemoryType
	EntityKeys      []string
	IncludeEvidence bool
}

type HydrateMemoryArgs struct {
	RuntimeContext *types.RuntimeContext
	Query          string
	TokenBudget    int
}

type PinMemoryArgs struct {
	RuntimeContext types.RuntimeContext
	BindingType    types.MemoryBindingType
	MemoryIDs      []string
	PinnedBy       types.MemoryPinnedBy
}

type ReflectMemoryArgs struct {
	RuntimeContext types.RuntimeContext
	FromSequence   int64
	ToSequence     int64
	Mode           string
}

type MemoryRecallItem struct {
	Memory    *types.Memory    `json:"memory"`
	Knowledge *types.Knowledge `json:"knowledge"`
	Evidence  []string         `json:"evidence,omitempty"`
}

type HydrateMemoryResult struct {
	CoreMemories           []string                   `json:"core_memories"`
	WorkingMemories        []string                   `json:"working_memories"`
	RecentEpisodicMemories []string                   `json:"recent_episodic_memories"`
	SemanticMemories       []string                   `json:"semantic_memories"`
	AssembledContext       string                     `json:"assembled_context"`
	Items                  []HydrateMemoryContextItem `json:"items"`
}

type HydrateMemoryContextItem struct {
	Role            string  `json:"role"`
	MemoryID        string  `json:"memory_id"`
	KnowledgeID     string  `json:"knowledge_id"`
	SpaceID         string  `json:"space_id"`
	Layer           string  `json:"layer"`
	Scope           string  `json:"scope"`
	MemoryType      string  `json:"memory_type"`
	Title           string  `json:"title"`
	Content         string  `json:"content"`
	EntityKey       string  `json:"entity_key,omitempty"`
	Confidence      float64 `json:"confidence"`
	Importance      int16   `json:"importance"`
	AuthorType      string  `json:"author_type"`
	EpistemicStatus string  `json:"epistemic_status"`
	SourceKind      string  `json:"source_kind"`
	SourceRef       string  `json:"source_ref,omitempty"`
}

func (l *MemoryLogic) RegisterKnowledgeMemory(spaceID string, knowledge *types.Knowledge, authorType types.MemoryAuthorType, sourceKind types.MemorySourceKind, sourceRef string) (string, error) {
	if knowledge == nil {
		return "", nil
	}

	existing, err := l.core.Store().MemoryStore().GetByKnowledgeID(l.ctx, spaceID, knowledge.ID)
	if err != nil && err != sql.ErrNoRows {
		return "", errors.New("MemoryLogic.RegisterKnowledgeMemory.MemoryStore.GetByKnowledgeID", i18n.ERROR_INTERNAL, err)
	}
	if existing != nil {
		return existing.ID, nil
	}

	user := l.GetUserInfo()
	now := time.Now().Unix()
	if authorType == "" {
		authorType = types.MEMORY_AUTHOR_HUMAN
	}
	if sourceKind == "" {
		sourceKind = types.MEMORY_SOURCE_MANUAL
	}
	memory := types.Memory{
		ID:              utils.GenRandomID(),
		SpaceID:         spaceID,
		UserID:          user.User,
		KnowledgeID:     knowledge.ID,
		MemoryType:      types.MEMORY_TYPE_SEMANTIC,
		Scope:           types.MEMORY_SCOPE_USER,
		Status:          types.MEMORY_STATUS_ACTIVE,
		Importance:      50,
		Confidence:      0.7,
		AuthorType:      authorType,
		EpistemicStatus: types.MEMORY_EPISTEMIC_SUMMARIZED,
		SourceKind:      sourceKind,
		SourceRef:       sourceRef,
		EntityKey:       "",
		DedupeKey:       buildMemoryDedupeKey(spaceID, knowledge.ID, ""),
		ConflictState:   types.MEMORY_CONFLICT_NONE,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := l.core.Store().MemoryStore().Create(l.ctx, memory); err != nil {
		return "", errors.New("MemoryLogic.RegisterKnowledgeMemory.MemoryStore.Create", i18n.ERROR_INTERNAL, err)
	}
	return memory.ID, nil
}

func (l *MemoryLogic) Remember(spaceID string, args RememberMemoryArgs) (string, string, error) {
	user := l.GetUserInfo()
	knowledgeID := args.KnowledgeID
	scope := args.Scope
	if scope == "" {
		scope = types.MEMORY_SCOPE_USER
	}

	if knowledgeID == "" {
		if err := l.ensureCanCreateMemory(spaceID, scope); err != nil {
			return "", "", err
		}
		kind := args.Kind
		if kind == "" {
			kind = types.KNOWLEDGE_KIND_TEXT
		}
		ctype := args.ContentType
		if ctype == "" {
			ctype = types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN
		}
		id, err := NewKnowledgeLogic(l.ctx, l.core).InsertContentAsyncWithSourceWithoutMemory(spaceID, types.MEMORY_BACKING_RESOURCE, kind, args.Content, ctype, memorySourceToKnowledgeSource(args.SourceKind), args.SourceRef)
		if err != nil {
			return "", "", err
		}
		knowledgeID = id
	}

	knowledge, err := l.core.Store().KnowledgeStore().GetKnowledge(l.ctx, spaceID, knowledgeID)
	if err != nil {
		return "", "", errors.New("MemoryLogic.Remember.KnowledgeStore.GetKnowledge", i18n.ERROR_INTERNAL, err)
	}

	existing, err := l.core.Store().MemoryStore().GetByKnowledgeID(l.ctx, spaceID, knowledgeID)
	if err != nil && err != sql.ErrNoRows {
		return "", "", errors.New("MemoryLogic.Remember.MemoryStore.GetByKnowledgeID", i18n.ERROR_INTERNAL, err)
	}

	now := time.Now().Unix()
	if existing != nil {
		if err := l.ensureCanMutateMemory(existing); err != nil {
			return "", "", err
		}
		accessCount := existing.AccessCount
		updateArgs := types.UpdateMemoryArgs{
			LastAccessedAt: &now,
			AccessCount:    &accessCount,
		}
		if args.Importance > 0 {
			updateArgs.Importance = &args.Importance
		}
		if args.Confidence > 0 {
			updateArgs.Confidence = &args.Confidence
		}
		if args.AuthorType != "" {
			updateArgs.AuthorType = args.AuthorType
		}
		if args.EpistemicStatus != "" {
			updateArgs.EpistemicStatus = args.EpistemicStatus
		}
		if args.EntityKey != "" {
			updateArgs.EntityKey = &args.EntityKey
		}
		if err = l.core.Store().MemoryStore().Update(l.ctx, spaceID, existing.ID, updateArgs); err != nil {
			return "", "", errors.New("MemoryLogic.Remember.MemoryStore.Update", i18n.ERROR_INTERNAL, err)
		}
		return existing.ID, knowledgeID, nil
	}

	if err := l.ensureCanCreateMemory(spaceID, scope); err != nil {
		return "", "", err
	}

	memoryType := args.MemoryType
	if memoryType == "" {
		memoryType = types.MEMORY_TYPE_SEMANTIC
	}
	authorType := args.AuthorType
	if authorType == "" {
		authorType = types.MEMORY_AUTHOR_HUMAN
	}
	epistemic := args.EpistemicStatus
	if epistemic == "" {
		epistemic = types.MEMORY_EPISTEMIC_STATED
	}
	confidence := args.Confidence
	if confidence == 0 {
		confidence = 0.7
	}
	importance := args.Importance
	if importance == 0 {
		importance = 50
	}
	sourceKind := args.SourceKind
	if sourceKind == "" {
		sourceKind = types.MEMORY_SOURCE_MANUAL
	}

	memory := types.Memory{
		ID:              utils.GenRandomID(),
		SpaceID:         spaceID,
		UserID:          user.User,
		KnowledgeID:     knowledgeID,
		MemoryType:      memoryType,
		Scope:           scope,
		Status:          types.MEMORY_STATUS_ACTIVE,
		Importance:      importance,
		Confidence:      confidence,
		AuthorType:      authorType,
		EpistemicStatus: epistemic,
		SourceKind:      sourceKind,
		SourceRef:       args.SourceRef,
		EntityKey:       args.EntityKey,
		DedupeKey:       buildMemoryDedupeKey(spaceID, knowledge.ID, args.EntityKey),
		ConflictState:   types.MEMORY_CONFLICT_NONE,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err = l.core.Store().MemoryStore().Create(l.ctx, memory); err != nil {
		return "", "", errors.New("MemoryLogic.Remember.MemoryStore.Create", i18n.ERROR_INTERNAL, err)
	}
	return memory.ID, knowledgeID, nil
}

func ResolveMemoryLayer(currentSpaceID string, layer types.MemoryLayer, fallbackScope types.MemoryScope) (string, types.MemoryScope) {
	switch layer {
	case types.MEMORY_LAYER_USER_GLOBAL:
		return types.GLOBAL_MEMORY_SPACE_ID, types.MEMORY_SCOPE_USER
	case types.MEMORY_LAYER_USER_SPACE:
		return currentSpaceID, types.MEMORY_SCOPE_USER
	case types.MEMORY_LAYER_SPACE_SHARED:
		return currentSpaceID, types.MEMORY_SCOPE_SPACE
	default:
		if fallbackScope != "" {
			return currentSpaceID, fallbackScope
		}
		return currentSpaceID, types.MEMORY_SCOPE_USER
	}
}

func MemoryLayerOf(memory *types.Memory) types.MemoryLayer {
	if memory == nil {
		return types.MEMORY_LAYER_USER_SPACE
	}
	if memory.SpaceID == types.GLOBAL_MEMORY_SPACE_ID && memory.Scope == types.MEMORY_SCOPE_USER {
		return types.MEMORY_LAYER_USER_GLOBAL
	}
	if memory.Scope == types.MEMORY_SCOPE_SPACE {
		return types.MEMORY_LAYER_SPACE_SHARED
	}
	return types.MEMORY_LAYER_USER_SPACE
}

func (l *MemoryLogic) Recall(spaceID string, args RecallMemoryArgs) ([]MemoryRecallItem, error) {
	limit := uint64(args.Limit)
	if limit == 0 {
		limit = 8
	}
	user := l.GetUserInfo()
	opts := types.GetMemoryOptions{
		Statuses:                 []types.MemoryStatus{types.MEMORY_STATUS_ACTIVE},
		Scopes:                   args.Scopes,
		MemoryTypes:              args.MemoryTypes,
		EntityKeys:               args.EntityKeys,
		AccessibleUserID:         user.User,
		AccessibleSpaceID:        spaceID,
		IncludeGlobalUserMemory:  true,
		IncludeSpaceUserMemory:   true,
		IncludeSpaceSharedMemory: true,
	}
	memories, err := l.core.Store().MemoryStore().List(l.ctx, opts, 1, 100)
	if err != nil {
		return nil, errors.New("MemoryLogic.Recall.MemoryStore.List", i18n.ERROR_INTERNAL, err)
	}

	vectorHits, err := l.recallByVector(spaceID, args, memories)
	if err != nil {
		return nil, err
	}

	sort.Slice(memories, func(i, j int) bool {
		if memories[i].Importance != memories[j].Importance {
			return memories[i].Importance > memories[j].Importance
		}
		if memories[i].Confidence != memories[j].Confidence {
			return memories[i].Confidence > memories[j].Confidence
		}
		return memories[i].UpdatedAt > memories[j].UpdatedAt
	})
	candidateLimit := int(limit) * 4
	if candidateLimit < 20 {
		candidateLimit = 20
	}
	if candidateLimit > len(memories) {
		candidateLimit = len(memories)
	}
	queryLower := strings.ToLower(strings.TrimSpace(args.Query))
	candidates := make([]types.Memory, 0, candidateLimit)
	candidateByMemoryID := make(map[string]struct{}, candidateLimit)
	appendCandidate := func(memory types.Memory) {
		if len(candidates) >= candidateLimit {
			return
		}
		if memory.ID == "" || memory.KnowledgeID == "" {
			return
		}
		if _, ok := candidateByMemoryID[memory.ID]; ok {
			return
		}
		candidateByMemoryID[memory.ID] = struct{}{}
		candidates = append(candidates, memory)
	}

	for _, memory := range memories {
		if len(candidates) >= candidateLimit {
			break
		}
		if queryLower == "" || strings.Contains(strings.ToLower(memory.EntityKey), queryLower) {
			appendCandidate(memory)
		}
	}

	for _, memory := range memories {
		if len(candidates) >= candidateLimit {
			break
		}
		if _, hit := vectorHits[memory.KnowledgeID]; hit {
			appendCandidate(memory)
		}
	}

	for _, memory := range memories {
		if len(candidates) >= candidateLimit {
			break
		}
		appendCandidate(memory)
	}

	knowledgeIDs := lo.Map(candidates, func(item types.Memory, _ int) string {
		return item.KnowledgeID
	})
	knowledgeIDs = lo.Uniq(knowledgeIDs)
	if len(knowledgeIDs) == 0 {
		return nil, nil
	}

	knowledgeList, err := l.core.Store().KnowledgeStore().ListKnowledges(l.ctx, types.GetKnowledgeOptions{
		SpaceIDs: accessibleMemorySpaceIDs(spaceID),
		IDs:      knowledgeIDs,
	}, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		return nil, errors.New("MemoryLogic.Recall.KnowledgeStore.ListKnowledges", i18n.ERROR_INTERNAL, err)
	}
	knowledgeByID := make(map[string]*types.Knowledge, len(knowledgeList))
	for _, knowledge := range knowledgeList {
		if knowledge == nil {
			continue
		}
		if knowledge.Content, err = l.core.DecryptData(knowledge.Content); err != nil {
			return nil, errors.New("MemoryLogic.Recall.DecryptData", i18n.ERROR_INTERNAL, err)
		}
		knowledgeByID[knowledge.ID] = knowledge
	}

	evidenceByMemoryID := make(map[string][]string)
	if args.IncludeEvidence {
		memoryIDs := lo.Map(candidates, func(item types.Memory, _ int) string {
			return item.ID
		})
		memoryIDs = lo.Uniq(memoryIDs)
		if len(memoryIDs) > 0 {
			edges, err := l.core.Store().MemoryEdgeStore().List(l.ctx, types.GetMemoryEdgeOptions{
				SpaceID:       spaceID,
				FromMemoryIDs: memoryIDs,
			}, 1, uint64(len(memoryIDs)*8))
			if err != nil {
				return nil, errors.New("MemoryLogic.Recall.MemoryEdgeStore.List", i18n.ERROR_INTERNAL, err)
			}
			for _, edge := range edges {
				evidenceByMemoryID[edge.FromMemoryID] = append(evidenceByMemoryID[edge.FromMemoryID], edge.ToMemoryID)
			}
		}
	}

	items := make([]MemoryRecallItem, 0, limit)
	seen := make(map[string]struct{})
	appendItem := func(memory types.Memory) error {
		if len(items) >= int(limit) {
			return nil
		}
		if _, ok := seen[memory.ID]; ok {
			return nil
		}
		knowledge, ok := knowledgeByID[memory.KnowledgeID]
		if !ok || knowledge == nil {
			return nil
		}

		if queryLower != "" {
			contentLower := strings.ToLower(knowledge.Content.String())
			titleLower := strings.ToLower(knowledge.Title)
			entityLower := strings.ToLower(memory.EntityKey)
			if !strings.Contains(contentLower, queryLower) && !strings.Contains(titleLower, queryLower) && !strings.Contains(entityLower, queryLower) {
				if _, hit := vectorHits[memory.KnowledgeID]; !hit {
					return nil
				}
			}
		}

		now := time.Now().Unix()
		access := memory.AccessCount + 1
		_ = l.core.Store().MemoryStore().Update(l.ctx, memory.SpaceID, memory.ID, types.UpdateMemoryArgs{
			LastAccessedAt: &now,
			AccessCount:    &access,
		})
		memory.LastAccessedAt = now
		memory.AccessCount = access

		item := MemoryRecallItem{
			Memory:    &memory,
			Knowledge: knowledge,
		}
		if args.IncludeEvidence {
			item.Evidence = evidenceByMemoryID[memory.ID]
		}
		seen[memory.ID] = struct{}{}
		items = append(items, item)
		return nil
	}

	for _, memory := range candidates {
		if len(items) >= int(limit) {
			break
		}
		if err := appendItem(memory); err != nil {
			return nil, err
		}
	}

	return items, nil
}

func (l *MemoryLogic) Get(spaceID, id string) (*MemoryRecallItem, error) {
	memory, err := l.findAccessibleMemory(spaceID, id)
	if err != nil {
		return nil, err
	}
	if memory == nil {
		return nil, errors.New("MemoryLogic.Get.MemoryNotFound", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNotFound)
	}

	knowledge, err := l.core.Store().KnowledgeStore().GetKnowledge(l.ctx, memory.SpaceID, memory.KnowledgeID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("MemoryLogic.Get.KnowledgeStore.GetKnowledge", i18n.ERROR_INTERNAL, err)
	}
	if knowledge == nil {
		return nil, errors.New("MemoryLogic.Get.KnowledgeNotFound", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNotFound)
	}
	if knowledge.Content, err = l.core.DecryptData(knowledge.Content); err != nil {
		return nil, errors.New("MemoryLogic.Get.DecryptData", i18n.ERROR_INTERNAL, err)
	}

	return &MemoryRecallItem{
		Memory:    memory,
		Knowledge: knowledge,
	}, nil
}

func (l *MemoryLogic) recallByVector(spaceID string, args RecallMemoryArgs, memories []types.Memory) (map[string]struct{}, error) {
	hits := make(map[string]struct{})
	if strings.TrimSpace(args.Query) == "" {
		return hits, nil
	}
	vectorOpts, ok := buildMemoryRecallVectorOptions(spaceID, memories)
	if !ok {
		return hits, nil
	}

	embedding, err := l.core.Srv().AI().EmbeddingForDocument(l.ctx, "", []string{args.Query})
	if err != nil || len(embedding.Data) == 0 {
		return hits, nil
	}

	refs, err := l.core.Store().VectorStore().Query(l.ctx, vectorOpts, pgvector.NewVector(embedding.Data[0]), 50)
	if err != nil {
		return nil, errors.New("MemoryLogic.recallByVector.VectorStore.Query", i18n.ERROR_INTERNAL, err)
	}
	for _, ref := range refs {
		hits[ref.KnowledgeID] = struct{}{}
	}
	return hits, nil
}

func buildMemoryRecallVectorOptions(spaceID string, memories []types.Memory) (types.GetVectorsOptions, bool) {
	knowledgeIDs := lo.Map(memories, func(item types.Memory, _ int) string {
		return item.KnowledgeID
	})
	knowledgeIDs = lo.Filter(knowledgeIDs, func(item string, _ int) bool {
		return item != ""
	})
	knowledgeIDs = lo.Uniq(knowledgeIDs)
	if len(knowledgeIDs) == 0 {
		return types.GetVectorsOptions{}, false
	}
	return types.GetVectorsOptions{
		SpaceIDs:     accessibleMemorySpaceIDs(spaceID),
		KnowledgeIDs: knowledgeIDs,
	}, true
}

func (l *MemoryLogic) Hydrate(spaceID string, args HydrateMemoryArgs) (*HydrateMemoryResult, error) {
	result := &HydrateMemoryResult{}
	user := l.GetUserInfo()
	if err := validateRuntimeContext(args.RuntimeContext, false); err != nil {
		return nil, err
	}
	if args.RuntimeContext != nil {
		bindings, err := l.core.Store().MemoryBindingStore().List(l.ctx, types.GetMemoryBindingOptions{
			SpaceID:     spaceID,
			UserID:      user.User,
			ContextType: args.RuntimeContext.Type,
			ContextID:   args.RuntimeContext.ID,
		}, 1, 100)
		if err != nil {
			return nil, errors.New("MemoryLogic.Hydrate.MemoryBindingStore.List", i18n.ERROR_INTERNAL, err)
		}
		for _, binding := range bindings {
			if binding.BindingType == types.MEMORY_BINDING_WORKING || binding.BindingType == types.MEMORY_BINDING_PIN {
				result.WorkingMemories = append(result.WorkingMemories, binding.MemoryID)
			}
		}
	}
	workingItems, err := l.loadAccessibleMemoryItems(spaceID, result.WorkingMemories)
	if err != nil {
		return nil, err
	}
	result.WorkingMemories = lo.Map(workingItems, func(item MemoryRecallItem, _ int) string {
		return item.Memory.ID
	})

	coreItems, err := l.Recall(spaceID, RecallMemoryArgs{
		Query:       args.Query,
		Limit:       6,
		Scopes:      []types.MemoryScope{types.MEMORY_SCOPE_USER, types.MEMORY_SCOPE_SPACE},
		MemoryTypes: []types.MemoryType{types.MEMORY_TYPE_CORE, types.MEMORY_TYPE_SEMANTIC, types.MEMORY_TYPE_EPISODIC},
	})
	if err != nil {
		return nil, err
	}

	contextParts := make([]string, 0, len(workingItems)+len(coreItems))
	contextMemoryIDs := make(map[string]struct{}, len(workingItems)+len(coreItems))
	for _, item := range workingItems {
		if item.Memory == nil || item.Knowledge == nil {
			continue
		}
		contextMemoryIDs[item.Memory.ID] = struct{}{}
		result.Items = append(result.Items, buildHydrateMemoryContextItem("working", item))
	}
	for _, item := range coreItems {
		if _, exists := contextMemoryIDs[item.Memory.ID]; exists {
			continue
		}
		contextMemoryIDs[item.Memory.ID] = struct{}{}
		result.Items = append(result.Items, buildHydrateMemoryContextItem("recall", item))
	}
	result.Items = applyHydrateTokenBudget(result.Items, args.TokenBudget)
	result.rebuildHydrateViews()
	for _, item := range result.Items {
		contextParts = append(contextParts, formatHydrateContextItem(item))
	}
	result.AssembledContext = strings.Join(contextParts, "\n\n")

	if args.RuntimeContext != nil && args.RuntimeContext.Type == types.MEMORY_CONTEXT_CHAT_SESSION {
		_ = l.syncChatSessionPin(spaceID, args.RuntimeContext.ID, result)
	}
	return result, nil
}

func (r *HydrateMemoryResult) rebuildHydrateViews() {
	r.CoreMemories = nil
	r.WorkingMemories = nil
	r.RecentEpisodicMemories = nil
	r.SemanticMemories = nil

	for _, item := range r.Items {
		switch item.Role {
		case "working":
			r.WorkingMemories = append(r.WorkingMemories, item.MemoryID)
		}
		switch item.MemoryType {
		case types.MEMORY_TYPE_CORE.String():
			r.CoreMemories = append(r.CoreMemories, item.MemoryID)
		case types.MEMORY_TYPE_SEMANTIC.String():
			r.SemanticMemories = append(r.SemanticMemories, item.MemoryID)
		case types.MEMORY_TYPE_EPISODIC.String():
			r.RecentEpisodicMemories = append(r.RecentEpisodicMemories, item.MemoryID)
		}
	}

	r.CoreMemories = lo.Uniq(r.CoreMemories)
	r.WorkingMemories = lo.Uniq(r.WorkingMemories)
	r.RecentEpisodicMemories = lo.Uniq(r.RecentEpisodicMemories)
	r.SemanticMemories = lo.Uniq(r.SemanticMemories)
}

func applyHydrateTokenBudget(items []HydrateMemoryContextItem, tokenBudget int) []HydrateMemoryContextItem {
	if tokenBudget <= 0 || len(items) == 0 {
		return items
	}

	selected := make([]HydrateMemoryContextItem, 0, len(items))
	used := 0
	for _, item := range items {
		cost := estimateTokenCount(formatHydrateContextItem(item))
		if cost <= 0 {
			continue
		}
		if used+cost > tokenBudget {
			continue
		}
		selected = append(selected, item)
		used += cost
	}
	return selected
}

func buildHydrateMemoryContextItem(role string, item MemoryRecallItem) HydrateMemoryContextItem {
	if item.Memory == nil || item.Knowledge == nil {
		return HydrateMemoryContextItem{Role: role}
	}
	return HydrateMemoryContextItem{
		Role:            role,
		MemoryID:        item.Memory.ID,
		KnowledgeID:     item.Knowledge.ID,
		SpaceID:         item.Memory.SpaceID,
		Layer:           MemoryLayerOf(item.Memory).String(),
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
	}
}

func formatHydrateContextItem(item HydrateMemoryContextItem) string {
	label := "Relevant memory"
	if item.Role == "working" {
		label = "Working memory"
	}
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = item.MemoryID
	}
	return fmt.Sprintf("## %s: %s\n%s", label, title, strings.TrimSpace(item.Content))
}

func estimateTokenCount(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	runes := len([]rune(text))
	return (runes + 3) / 4
}

func (l *MemoryLogic) loadAccessibleMemoryItems(spaceID string, memoryIDs []string) ([]MemoryRecallItem, error) {
	memoryIDs = lo.Uniq(lo.Filter(memoryIDs, func(item string, _ int) bool {
		return strings.TrimSpace(item) != ""
	}))
	if len(memoryIDs) == 0 {
		return nil, nil
	}

	user := l.GetUserInfo()
	memories, err := l.core.Store().MemoryStore().List(l.ctx, types.GetMemoryOptions{
		IDs:                      memoryIDs,
		Statuses:                 []types.MemoryStatus{types.MEMORY_STATUS_ACTIVE},
		AccessibleUserID:         user.User,
		AccessibleSpaceID:        spaceID,
		IncludeGlobalUserMemory:  true,
		IncludeSpaceUserMemory:   true,
		IncludeSpaceSharedMemory: true,
	}, 1, uint64(len(memoryIDs)))
	if err != nil {
		return nil, errors.New("MemoryLogic.loadAccessibleMemoryItems.MemoryStore.List", i18n.ERROR_INTERNAL, err)
	}
	if len(memories) == 0 {
		return nil, nil
	}

	memoryByID := lo.SliceToMap(memories, func(item types.Memory) (string, types.Memory) {
		return item.ID, item
	})
	orderedMemories := make([]types.Memory, 0, len(memories))
	for _, id := range memoryIDs {
		if memory, ok := memoryByID[id]; ok && memory.KnowledgeID != "" {
			orderedMemories = append(orderedMemories, memory)
		}
	}
	if len(orderedMemories) == 0 {
		return nil, nil
	}

	knowledgeIDs := lo.Uniq(lo.Map(orderedMemories, func(item types.Memory, _ int) string {
		return item.KnowledgeID
	}))
	knowledgeList, err := l.core.Store().KnowledgeStore().ListKnowledges(l.ctx, types.GetKnowledgeOptions{
		SpaceIDs: accessibleMemorySpaceIDs(spaceID),
		IDs:      knowledgeIDs,
	}, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		return nil, errors.New("MemoryLogic.loadAccessibleMemoryItems.KnowledgeStore.ListKnowledges", i18n.ERROR_INTERNAL, err)
	}

	knowledgeByID := make(map[string]*types.Knowledge, len(knowledgeList))
	for _, knowledge := range knowledgeList {
		if knowledge == nil {
			continue
		}
		if knowledge.Content, err = l.core.DecryptData(knowledge.Content); err != nil {
			return nil, errors.New("MemoryLogic.loadAccessibleMemoryItems.DecryptData", i18n.ERROR_INTERNAL, err)
		}
		knowledgeByID[knowledge.ID] = knowledge
	}

	items := make([]MemoryRecallItem, 0, len(orderedMemories))
	for _, memory := range orderedMemories {
		knowledge := knowledgeByID[memory.KnowledgeID]
		if knowledge == nil {
			continue
		}
		memoryCopy := memory
		items = append(items, MemoryRecallItem{Memory: &memoryCopy, Knowledge: knowledge})
	}
	return items, nil
}

func formatMemoryContextPart(label string, item MemoryRecallItem) string {
	title := strings.TrimSpace(item.Knowledge.Title)
	if title == "" {
		title = item.Memory.ID
	}
	return fmt.Sprintf("## %s: %s\n%s", label, title, strings.TrimSpace(item.Knowledge.Content.String()))
}

func (l *MemoryLogic) Pin(spaceID string, args PinMemoryArgs) error {
	if err := validateRuntimeContext(&args.RuntimeContext, true); err != nil {
		return err
	}
	now := time.Now().Unix()
	user := l.GetUserInfo()
	for _, memoryID := range args.MemoryIDs {
		if memoryID == "" {
			continue
		}
		memory, err := l.findAccessibleMemory(spaceID, memoryID)
		if err != nil {
			return err
		}
		if memory == nil {
			continue
		}
		existing, err := l.core.Store().MemoryBindingStore().List(l.ctx, types.GetMemoryBindingOptions{
			SpaceID:     spaceID,
			UserID:      user.User,
			MemoryID:    memoryID,
			ContextType: args.RuntimeContext.Type,
			ContextID:   args.RuntimeContext.ID,
			BindingType: args.BindingType,
		}, 1, 1)
		if err != nil {
			return errors.New("MemoryLogic.Pin.MemoryBindingStore.List", i18n.ERROR_INTERNAL, err)
		}
		if len(existing) == 0 {
			err = l.core.Store().MemoryBindingStore().Create(l.ctx, types.MemoryBinding{
				ID:          utils.GenRandomID(),
				SpaceID:     spaceID,
				UserID:      user.User,
				MemoryID:    memoryID,
				ContextType: args.RuntimeContext.Type,
				ContextID:   args.RuntimeContext.ID,
				BindingType: args.BindingType,
				PinnedBy:    lo.If(args.PinnedBy != "", args.PinnedBy).Else(types.MEMORY_PINNED_BY_AGENT),
				CreatedAt:   now,
				UpdatedAt:   now,
			})
			if err != nil {
				return errors.New("MemoryLogic.Pin.MemoryBindingStore.Create", i18n.ERROR_INTERNAL, err)
			}
		}

		access := int64(1)
		_ = l.core.Store().MemoryStore().Update(l.ctx, memory.SpaceID, memoryID, types.UpdateMemoryArgs{
			LastAccessedAt: &now,
			AccessCount:    &access,
		})
	}
	if args.RuntimeContext.Type == types.MEMORY_CONTEXT_CHAT_SESSION {
		hydration := &HydrateMemoryResult{WorkingMemories: args.MemoryIDs}
		_ = l.syncChatSessionPin(spaceID, args.RuntimeContext.ID, hydration)
	}
	return nil
}

func (l *MemoryLogic) Reflect(spaceID string, args ReflectMemoryArgs) (string, error) {
	if err := validateRuntimeContext(&args.RuntimeContext, true); err != nil {
		return "", err
	}
	var content string
	switch args.RuntimeContext.Type {
	case types.MEMORY_CONTEXT_CHAT_SESSION:
		summary, err := l.core.Store().ChatSummaryStore().GetChatSessionLatestSummary(l.ctx, args.RuntimeContext.ID)
		if err != nil && err != sql.ErrNoRows {
			return "", errors.New("MemoryLogic.Reflect.ChatSummaryStore.GetChatSessionLatestSummary", i18n.ERROR_INTERNAL, err)
		}
		if summary != nil {
			content = summary.Content
		}
		if strings.TrimSpace(content) == "" {
			return "", errors.New("MemoryLogic.Reflect.EmptyChatSessionSummary", i18n.ERROR_INVALIDARGUMENT, nil)
		}
	case types.MEMORY_CONTEXT_AGENT_RUN, types.MEMORY_CONTEXT_TASK, types.MEMORY_CONTEXT_WORKSPACE:
		content = args.RuntimeContext.Extraction.ReflectionContent()
		if strings.TrimSpace(content) == "" {
			return "", errors.New("MemoryLogic.Reflect.EmptyExtractionContent", i18n.ERROR_INVALIDARGUMENT, nil)
		}
	default:
		return "", errors.New("MemoryLogic.Reflect.UnsupportedRuntimeContextType", i18n.ERROR_INVALIDARGUMENT, nil)
	}

	memoryID, _, err := l.Remember(spaceID, RememberMemoryArgs{
		Title:           "Reflected memory",
		Content:         types.KnowledgeContent(content),
		ContentType:     types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN,
		Kind:            types.KNOWLEDGE_KIND_TEXT,
		MemoryType:      types.MEMORY_TYPE_EPISODIC,
		Scope:           types.MEMORY_SCOPE_USER,
		AuthorType:      types.MEMORY_AUTHOR_AGENT,
		EpistemicStatus: types.MEMORY_EPISTEMIC_SUMMARIZED,
		Importance:      60,
		Confidence:      0.75,
		SourceKind:      types.MEMORY_SOURCE_REFLECTION,
		SourceRef:       args.RuntimeContext.ID,
	})
	if err != nil {
		return "", err
	}

	bindings, err := l.core.Store().MemoryBindingStore().List(l.ctx, types.GetMemoryBindingOptions{
		SpaceID:     spaceID,
		UserID:      l.GetUserInfo().User,
		ContextType: args.RuntimeContext.Type,
		ContextID:   args.RuntimeContext.ID,
	}, 1, 100)
	if err == nil {
		for _, binding := range bindings {
			if binding.MemoryID == memoryID {
				continue
			}
			_ = l.core.Store().MemoryEdgeStore().Create(l.ctx, types.MemoryEdge{
				ID:           utils.GenRandomID(),
				SpaceID:      spaceID,
				FromMemoryID: memoryID,
				ToMemoryID:   binding.MemoryID,
				Relation:     types.MEMORY_RELATION_DERIVED_FROM,
				Weight:       1.0,
				CreatedAt:    time.Now().Unix(),
			})
		}
	}
	return memoryID, nil
}

func (l *MemoryLogic) Update(spaceID, id string, args types.UpdateMemoryArgs) error {
	memory, err := l.findAccessibleMemory(spaceID, id)
	if err != nil {
		return err
	}
	if memory == nil {
		return nil
	}
	if err := l.ensureCanMutateMemory(memory); err != nil {
		return err
	}
	if err := l.core.Store().MemoryStore().Update(l.ctx, memory.SpaceID, id, args); err != nil {
		return errors.New("MemoryLogic.Update.MemoryStore.Update", i18n.ERROR_INTERNAL, err)
	}
	return nil
}

func (l *MemoryLogic) Delete(spaceID, id string, hard bool) error {
	memory, err := l.findAccessibleMemory(spaceID, id)
	if err != nil {
		return err
	}
	if memory == nil {
		return nil
	}
	if err := l.ensureCanMutateMemory(memory); err != nil {
		return err
	}
	return l.delete(memory.SpaceID, id, hard, false)
}

func (l *MemoryLogic) Forget(spaceID, id string, deleteKnowledge bool) error {
	memory, err := l.findAccessibleMemory(spaceID, id)
	if err != nil {
		return err
	}
	if memory == nil {
		return nil
	}
	if err := l.ensureCanMutateMemory(memory); err != nil {
		return err
	}
	return l.delete(memory.SpaceID, id, true, deleteKnowledge)
}

func (l *MemoryLogic) delete(spaceID, id string, hard bool, deleteKnowledge bool) error {
	if hard {
		memory, err := l.core.Store().MemoryStore().Get(l.ctx, spaceID, id)
		if err != nil && err != sql.ErrNoRows {
			return errors.New("MemoryLogic.Delete.MemoryStore.Get", i18n.ERROR_INTERNAL, err)
		}
		if memory == nil {
			return nil
		}

		err = l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
			if err := l.deleteMemoryReferences(ctx, spaceID, id); err != nil {
				return err
			}
			if err := l.core.Store().MemoryStore().Delete(ctx, spaceID, id); err != nil {
				return errors.New("MemoryLogic.Delete.MemoryStore.Delete", i18n.ERROR_INTERNAL, err)
			}
			if deleteKnowledge && memory.KnowledgeID != "" {
				if err := l.core.Store().KnowledgeStore().Delete(ctx, memory.SpaceID, memory.KnowledgeID); err != nil && err != sql.ErrNoRows {
					return errors.New("MemoryLogic.Delete.KnowledgeStore.Delete", i18n.ERROR_INTERNAL, err)
				}
				if err := l.core.Store().KnowledgeChunkStore().BatchDelete(ctx, memory.SpaceID, memory.KnowledgeID); err != nil && err != sql.ErrNoRows {
					return errors.New("MemoryLogic.Delete.KnowledgeChunkStore.BatchDelete", i18n.ERROR_INTERNAL, err)
				}
				if err := l.core.Store().VectorStore().BatchDelete(ctx, memory.SpaceID, []string{memory.KnowledgeID}); err != nil && err != sql.ErrNoRows {
					return errors.New("MemoryLogic.Delete.VectorStore.BatchDelete", i18n.ERROR_INTERNAL, err)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	}
	status := types.MEMORY_STATUS_DELETED
	if err := l.core.Store().MemoryStore().Update(l.ctx, spaceID, id, types.UpdateMemoryArgs{Status: status}); err != nil {
		return errors.New("MemoryLogic.Delete.MemoryStore.Update", i18n.ERROR_INTERNAL, err)
	}
	return nil
}

func (l *MemoryLogic) findAccessibleMemory(spaceID, id string) (*types.Memory, error) {
	user := l.GetUserInfo()
	memories, err := l.core.Store().MemoryStore().List(l.ctx, types.GetMemoryOptions{
		ID:                       id,
		AccessibleUserID:         user.User,
		AccessibleSpaceID:        spaceID,
		IncludeGlobalUserMemory:  true,
		IncludeSpaceUserMemory:   true,
		IncludeSpaceSharedMemory: true,
	}, 1, 1)
	if err != nil {
		return nil, errors.New("MemoryLogic.findAccessibleMemory.MemoryStore.List", i18n.ERROR_INTERNAL, err)
	}
	if len(memories) == 0 {
		return nil, nil
	}
	return &memories[0], nil
}

func (l *MemoryLogic) ensureCanMutateMemory(memory *types.Memory) error {
	user := l.GetUserInfo()
	if canMutateMemory(memory, user.User, false) {
		return nil
	}
	if memory == nil || memory.Scope != types.MEMORY_SCOPE_SPACE || memory.SpaceID == types.GLOBAL_MEMORY_SPACE_ID {
		return errors.New("MemoryLogic.ensureCanMutateMemory.OwnerOnly", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	userSpace, err := l.core.Store().UserSpaceStore().GetUserSpaceRole(l.ctx, user.User, memory.SpaceID)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("MemoryLogic.ensureCanMutateMemory.UserSpaceStore.GetUserSpaceRole", i18n.ERROR_INTERNAL, err)
	}
	if userSpace == nil || !l.core.Srv().RBAC().CheckPermission(userSpace.Role, srv.PermissionEdit) {
		return errors.New("MemoryLogic.ensureCanMutateMemory.RBAC.CheckPermission", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}
	if !canMutateMemory(memory, user.User, true) {
		return errors.New("MemoryLogic.ensureCanMutateMemory.Denied", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}
	return nil
}

func (l *MemoryLogic) ensureCanCreateMemory(spaceID string, scope types.MemoryScope) error {
	user := l.GetUserInfo()
	if canCreateMemory(spaceID, scope, false) {
		return nil
	}
	if scope != types.MEMORY_SCOPE_SPACE || spaceID == types.GLOBAL_MEMORY_SPACE_ID {
		return errors.New("MemoryLogic.ensureCanCreateMemory.InvalidTarget", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	userSpace, err := l.core.Store().UserSpaceStore().GetUserSpaceRole(l.ctx, user.User, spaceID)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("MemoryLogic.ensureCanCreateMemory.UserSpaceStore.GetUserSpaceRole", i18n.ERROR_INTERNAL, err)
	}
	if userSpace == nil || !l.core.Srv().RBAC().CheckPermission(userSpace.Role, srv.PermissionEdit) {
		return errors.New("MemoryLogic.ensureCanCreateMemory.RBAC.CheckPermission", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}
	if !canCreateMemory(spaceID, scope, true) {
		return errors.New("MemoryLogic.ensureCanCreateMemory.Denied", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}
	return nil
}

func canMutateMemory(memory *types.Memory, userID string, hasSpaceEdit bool) bool {
	if memory == nil || userID == "" {
		return false
	}
	if memory.Scope == types.MEMORY_SCOPE_USER {
		return memory.UserID == userID
	}
	if memory.Scope == types.MEMORY_SCOPE_SPACE {
		return memory.SpaceID != types.GLOBAL_MEMORY_SPACE_ID && hasSpaceEdit
	}
	return memory.UserID == userID
}

func canCreateMemory(spaceID string, scope types.MemoryScope, hasSpaceEdit bool) bool {
	switch scope {
	case "", types.MEMORY_SCOPE_USER:
		return true
	case types.MEMORY_SCOPE_SPACE:
		return spaceID != "" && spaceID != types.GLOBAL_MEMORY_SPACE_ID && hasSpaceEdit
	default:
		return false
	}
}

func validateRuntimeContext(runtimeContext *types.RuntimeContext, required bool) error {
	if runtimeContext == nil {
		if required {
			return errors.New("MemoryLogic.validateRuntimeContext.Required", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
		}
		return nil
	}
	runtimeContext.ID = strings.TrimSpace(runtimeContext.ID)
	if runtimeContext.ID == "" {
		return errors.New("MemoryLogic.validateRuntimeContext.IDRequired", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}
	switch runtimeContext.Type {
	case types.MEMORY_CONTEXT_CHAT_SESSION, types.MEMORY_CONTEXT_AGENT_RUN, types.MEMORY_CONTEXT_TASK, types.MEMORY_CONTEXT_WORKSPACE:
		return nil
	default:
		return errors.New("MemoryLogic.validateRuntimeContext.UnsupportedType", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}
}

func (l *MemoryLogic) deleteMemoryReferences(ctx context.Context, spaceID, memoryID string) error {
	memoryIDs := []string{memoryID}
	if err := l.core.Store().MemoryBindingStore().DeleteByMemoryIDs(ctx, spaceID, memoryIDs); err != nil {
		return errors.New("MemoryLogic.deleteMemoryReferences.MemoryBindingStore.DeleteByMemoryIDs", i18n.ERROR_INTERNAL, err)
	}
	if err := l.core.Store().MemoryEdgeStore().DeleteByMemoryIDs(ctx, spaceID, memoryIDs); err != nil {
		return errors.New("MemoryLogic.deleteMemoryReferences.MemoryEdgeStore.DeleteByMemoryIDs", i18n.ERROR_INTERNAL, err)
	}
	return nil
}

func buildMemoryDedupeKey(spaceID, knowledgeID, entityKey string) string {
	if entityKey != "" {
		return fmt.Sprintf("%s:%s:%s", spaceID, knowledgeID, entityKey)
	}
	return fmt.Sprintf("%s:%s", spaceID, knowledgeID)
}

func accessibleMemorySpaceIDs(spaceID string) []string {
	if spaceID == "" || spaceID == types.GLOBAL_MEMORY_SPACE_ID {
		return []string{types.GLOBAL_MEMORY_SPACE_ID}
	}
	return []string{types.GLOBAL_MEMORY_SPACE_ID, spaceID}
}

func memorySourceToKnowledgeSource(source types.MemorySourceKind) types.KnowledgeSource {
	switch source {
	case types.MEMORY_SOURCE_CHAT:
		return types.KNOWLEDGE_SOURCE_CHAT
	case types.MEMORY_SOURCE_MCP:
		return types.KNOWLEDGE_SOURCE_MCP
	case types.MEMORY_SOURCE_RSS:
		return types.KNOWLEDGE_SOURCE_RSS
	default:
		return types.KNOWLEDGE_SOURCE_PLATFORM
	}
}

func knowledgeSourceToMemorySource(source types.KnowledgeSource) types.MemorySourceKind {
	switch source {
	case types.KNOWLEDGE_SOURCE_CHAT:
		return types.MEMORY_SOURCE_CHAT
	case types.KNOWLEDGE_SOURCE_MCP:
		return types.MEMORY_SOURCE_MCP
	case types.KNOWLEDGE_SOURCE_RSS:
		return types.MEMORY_SOURCE_RSS
	default:
		return types.MEMORY_SOURCE_MANUAL
	}
}

func knowledgeSourceToMemoryAuthor(source types.KnowledgeSource) types.MemoryAuthorType {
	switch source {
	case types.KNOWLEDGE_SOURCE_CHAT, types.KNOWLEDGE_SOURCE_MCP:
		return types.MEMORY_AUTHOR_AGENT
	case types.KNOWLEDGE_SOURCE_RSS, types.KNOWLEDGE_SOURCE_PODCAST:
		return types.MEMORY_AUTHOR_SYSTEM
	default:
		return types.MEMORY_AUTHOR_HUMAN
	}
}

func (l *MemoryLogic) syncChatSessionPin(spaceID, sessionID string, hydration *HydrateMemoryResult) error {
	pin, err := l.core.Store().ChatSessionPinStore().GetBySessionID(l.ctx, sessionID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	content := types.ContentPinV2{
		Memories: lo.Uniq(hydration.WorkingMemories),
		Hydration: &types.ChatSessionHydrationV2{
			Core:           lo.Uniq(hydration.CoreMemories),
			Working:        lo.Uniq(hydration.WorkingMemories),
			RecentEpisodic: lo.Uniq(hydration.RecentEpisodicMemories),
			GeneratedAt:    time.Now().Unix(),
			Version:        types.CHAT_SESSION_PIN_VERSION_V2,
		},
	}
	raw, err := json.Marshal(content)
	if err != nil {
		return err
	}

	if pin == nil {
		user := l.GetUserInfo()
		return l.core.Store().ChatSessionPinStore().Create(l.ctx, types.ChatSessionPin{
			SessionID: sessionID,
			SpaceID:   spaceID,
			UserID:    user.User,
			Content:   raw,
			Version:   types.CHAT_SESSION_PIN_VERSION_V2,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		})
	}
	return l.core.Store().ChatSessionPinStore().Update(l.ctx, spaceID, sessionID, raw, types.CHAT_SESSION_PIN_VERSION_V2)
}
