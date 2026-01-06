package v1

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type ResourceLogic struct {
	ctx  context.Context
	core *core.Core
}

func NewResourceLogic(ctx context.Context, core *core.Core) *ResourceLogic {
	l := &ResourceLogic{
		ctx:  ctx,
		core: core,
	}

	return l
}

func (l *ResourceLogic) CreateResource(spaceID, userID, id, title, desc, tag string, cycle int) error {
	if title == "" {
		title = id
	}

	if id == "knowledge" || title == "knowledge" {
		return errors.New("ResourceLogic.CreateResource.InvalidWord", i18n.ERROR_EXIST, nil).Code(http.StatusForbidden)
	}

	exist, err := l.core.Store().ResourceStore().GetResource(l.ctx, spaceID, id)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("ResourceLogic.CreateResource.ResourceStore.GetResource", i18n.ERROR_INTERNAL, err)
	}

	if exist != nil {
		return errors.New("ResourceLogic.CreateResource.exist", i18n.ERROR_EXIST, nil).Code(http.StatusBadRequest)
	}

	err = l.core.Store().ResourceStore().Create(l.ctx, types.Resource{
		ID:          id,
		UserID:      userID,
		SpaceID:     spaceID,
		Title:       title,
		Description: desc,
		Tag:         tag,
		Cycle:       cycle,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		return errors.New("ResourceLogic.CreateResource.ResourceStore.Create", i18n.ERROR_INTERNAL, err)
	}

	return nil
}

func (l *ResourceLogic) Delete(spaceID, id string) error {
	return l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		knowledgeIDs, err := l.core.Store().KnowledgeStore().ListKnowledgeIDs(ctx, types.GetKnowledgeOptions{
			Resource: &types.ResourceQuery{
				Include: []string{id},
			},
			SpaceID: spaceID,
		}, types.NO_PAGINATION, types.NO_PAGINATION)

		if err != nil {
			return errors.New("ResourceLogic.Delete.KnowledgeStore.ListKnowledges", i18n.ERROR_INTERNAL, err)
		}

		if err = l.core.Store().KnowledgeStore().BatchDelete(ctx, knowledgeIDs); err != nil {
			return errors.New("ResourceLogic.Delete.KnowledgeStore.BatchDelete", i18n.ERROR_INTERNAL, err)
		}

		if err = l.core.Store().KnowledgeChunkStore().BatchDeleteByIDs(ctx, knowledgeIDs); err != nil {
			return errors.New("ResourceLogic.Delete.KnowledgeChunkStore.BatchDeleteByIDs", i18n.ERROR_INTERNAL, err)
		}

		if err = l.core.Store().VectorStore().DeleteByResource(ctx, spaceID, id); err != nil {
			return errors.New("ResourceLogic.Delete.VectorStore.DeleteByResource", i18n.ERROR_INTERNAL, err)
		}

		if err = l.core.Store().ResourceStore().Delete(ctx, spaceID, id); err != nil {
			return errors.New("ResourceLogic.Delete.ResourceStore.Delete", i18n.ERROR_INTERNAL, err)
		}
		return nil
	})

}

func (l *ResourceLogic) Update(spaceID, id, title, desc, tag string, cycle int) error {
	// 先获取当前资源信息，检查cycle是否变化
	currentResource, err := l.core.Store().ResourceStore().GetResource(l.ctx, spaceID, id)
	if err != nil {
		return errors.New("ResourceLogic.Update.ResourceStore.GetResource", i18n.ERROR_INTERNAL, err)
	}

	resources, err := l.core.Store().ResourceStore().ListResources(l.ctx, spaceID, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		return errors.New("ResourceLogic.Update.ResourceStore.ListResources", i18n.ERROR_INTERNAL, err)
	}

	for _, v := range resources {
		if v.ID != id && v.Title == title {
			return errors.New("ResourceLogic.Update.ResourceStore.ListResources", i18n.ERROR_TITLE_EXIST, nil).Code(http.StatusForbidden)
		}
	}

	return l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		// 更新resource
		err = l.core.Store().ResourceStore().Update(ctx, spaceID, id, title, desc, tag, cycle)
		if err != nil {
			return errors.New("ResourceLogic.Update.ResourceStore.Update", i18n.ERROR_INTERNAL, err)
		}

		// 如果cycle发生变化，批量更新相关knowledge的过期时间
		if currentResource != nil && currentResource.Cycle != cycle {
			err = l.updateKnowledgeExpiredAtByResource(ctx, id, cycle)
			if err != nil {
				return errors.New("ResourceLogic.Update.updateKnowledgeExpiredAtByResource", i18n.ERROR_INTERNAL, err)
			}
		}

		return nil
	})
}

func (l *ResourceLogic) ListSpaceResources(spaceID string) ([]types.Resource, error) {
	list, err := l.core.Store().ResourceStore().ListResources(l.ctx, spaceID, 0, 0)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ResourceLogic.ListSpaceResources.ResourceStore.ListResources", i18n.ERROR_INTERNAL, err)
	}

	defaultKnowledgeResource := types.Resource{
		ID:      "knowledge",
		Title:   "resourceKnowledge",
		SpaceID: spaceID,
		Tag:     "resources",
	}

	if len(list) == 0 {
		list = append(list, defaultKnowledgeResource)
	} else {
		list = append([]types.Resource{defaultKnowledgeResource}, list...)
	}

	return list, nil
}

func (l *ResourceLogic) GetResource(spaceID, id string) (*types.Resource, error) {
	data, err := l.core.Store().ResourceStore().GetResource(l.ctx, spaceID, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ResourceLogic.GetResource.ResourceStore.GetResource", i18n.ERROR_INTERNAL, err)
	}
	return data, nil
}

func (l *ResourceLogic) ListUserResources(userID string, page, pagesize uint64) ([]types.Resource, error) {
	list, err := l.core.Store().ResourceStore().ListUserResources(l.ctx, userID, page, pagesize)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ResourceLogic.ListUserResources.ResourceStore.ListUserResources", i18n.ERROR_INTERNAL, err)
	}

	return list, nil
}

// updateKnowledgeExpiredAtByResource 批量更新指定resource的所有knowledge的过期时间
func (l *ResourceLogic) updateKnowledgeExpiredAtByResource(ctx context.Context, resourceID string, cycle int) error {
	// 获取所有相关的knowledge信息
	knowledges, err := l.core.Store().KnowledgeStore().ListKnowledges(ctx, types.GetKnowledgeOptions{
		Resource: &types.ResourceQuery{
			Include: []string{resourceID},
		},
	}, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		return err
	}

	// 如果没有相关知识，直接返回
	if len(knowledges) == 0 {
		return nil
	}

	// 在逻辑层为每个knowledge计算新的过期时间
	for _, knowledge := range knowledges {
		// 根据knowledge的创建时间计算新的过期时间
		newExpiredAt := types.CalculateExpiredAt(knowledge.CreatedAt, cycle)
		// 更新单个knowledge的过期时间
		knowledge.ExpiredAt = newExpiredAt
		err = l.core.Store().KnowledgeStore().UpdateExpiredAt(ctx, knowledge.ID, newExpiredAt)
		if err != nil {
			return err
		}
	}

	return nil
}
