package sqlstore

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.ResourceStore = NewResourceStore(provider)
	})
}

// ResourceStore 处理 bw_resource 表的操作
type ResourceStore struct {
	CommonFields
}

// NewResourceStore 创建新的 ResourceStore 实例
func NewResourceStore(provider SqlProviderAchieve) *ResourceStore {
	repo := &ResourceStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_RESOURCE) // 表名
	repo.SetAllColumns("id", "title", "user_id", "space_id", "description", "tag", "cycle", "created_at")
	return repo
}

// Create 创建新的资源记录
func (s *ResourceStore) Create(ctx context.Context, data types.Resource) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("id", "title", "user_id", "space_id", "description", "tag", "cycle", "created_at").
		Values(data.ID, data.Title, data.UserID, data.SpaceID, data.Description, data.Tag, data.Cycle, data.CreatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	if err != nil {
		return err
	}
	return nil
}

// GetResource 根据ID获取资源记录
func (s *ResourceStore) GetResource(ctx context.Context, spaceID, id string) (*types.Resource, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.Resource
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新资源记录
func (s *ResourceStore) Update(ctx context.Context, spaceID, id, title, desc, tag string, cycle int) error {
	query := sq.Update(s.GetTable()).
		Set("title", title).
		Set("description", desc).
		Set("tag", tag).
		Set("cycle", cycle).
		Where(sq.Eq{"space_id": spaceID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除资源记录
func (s *ResourceStore) Delete(ctx context.Context, spaceID, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// ListResources 分页获取资源记录列表
func (s *ResourceStore) ListResources(ctx context.Context, spaceID string, page, pageSize uint64) ([]types.Resource, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID}).OrderBy("created_at")
	if page != 0 || pageSize != 0 {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.Resource
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

// ListResources 分页获取资源记录列表
func (s *ResourceStore) ListUserResources(ctx context.Context, userID string, page, pageSize uint64) ([]types.Resource, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"user_id": userID}).OrderBy("created_at")
	if page != 0 || pageSize != 0 {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.Resource
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
