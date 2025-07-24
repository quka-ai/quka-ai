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
	UserInfo
}

func NewResourceLogic(ctx context.Context, core *core.Core) *ResourceLogic {
	l := &ResourceLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}

	return l
}

func (l *ResourceLogic) CreateResource(spaceID, id, title, desc, tag string, cycle int) error {
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
		UserID:      l.GetUserInfo().User,
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
	resources, err := l.core.Store().ResourceStore().ListResources(l.ctx, spaceID, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		return errors.New("ResourceLogic.Update.ResourceStore.ListResources", i18n.ERROR_INTERNAL, err)
	}

	for _, v := range resources {
		if v.ID != id && v.Title == title {
			return errors.New("ResourceLogic.Update.ResourceStore.ListResources", i18n.ERROR_TITLE_EXIST, nil).Code(http.StatusForbidden)
		}
	}

	err = l.core.Store().ResourceStore().Update(l.ctx, spaceID, id, title, desc, tag, cycle)
	if err != nil {
		return errors.New("ResourceLogic.Update.ResourceStore.Update", i18n.ERROR_INTERNAL, err)
	}
	return nil
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

func (l *ResourceLogic) ListUserResources(page, pagesize uint64) ([]types.Resource, error) {
	list, err := l.core.Store().ResourceStore().ListUserResources(l.ctx, l.GetUserInfo().User, page, pagesize)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ResourceLogic.ListUserResources.ResourceStore.ListUserResources", i18n.ERROR_INTERNAL, err)
	}

	return list, nil
}
