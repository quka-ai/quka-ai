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
		provider.stores.ModelProviderStore = NewModelProviderStore(provider)
	})
}

// ModelProviderStore 处理模型提供商表的操作
type ModelProviderStore struct {
	CommonFields
}

// NewModelProviderStore 创建新的 ModelProviderStore 实例
func NewModelProviderStore(provider SqlProviderAchieve) *ModelProviderStore {
	repo := &ModelProviderStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_MODEL_PROVIDER)
	repo.SetAllColumns("id", "name", "description", "api_url", "api_key", "status", "config", "created_at", "updated_at")
	return repo
}

// Create 创建新的模型提供商
func (s *ModelProviderStore) Create(ctx context.Context, data types.ModelProvider) error {
	now := time.Now().Unix()
	if data.CreatedAt == 0 {
		data.CreatedAt = now
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = now
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "name", "description", "api_url", "api_key", "status", "config", "created_at", "updated_at").
		Values(data.ID, data.Name, data.Description, data.ApiUrl, data.ApiKey, data.Status, data.Config, data.CreatedAt, data.UpdatedAt)

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

// Get 根据ID获取模型提供商
func (s *ModelProviderStore) Get(ctx context.Context, id string) (*types.ModelProvider, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.ModelProvider
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新模型提供商
func (s *ModelProviderStore) Update(ctx context.Context, id string, data types.ModelProvider) error {
	data.UpdatedAt = time.Now().Unix()

	query := sq.Update(s.GetTable()).
		SetMap(map[string]interface{}{
			"name":        data.Name,
			"description": data.Description,
			"api_url":     data.ApiUrl,
			"api_key":     data.ApiKey,
			"status":      data.Status,
			"config":      data.Config,
			"updated_at":  data.UpdatedAt,
		}).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除模型提供商
func (s *ModelProviderStore) Delete(ctx context.Context, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// List 分页获取模型提供商列表
func (s *ModelProviderStore) List(ctx context.Context, opts types.ListModelProviderOptions, page, pageSize uint64) ([]types.ModelProvider, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).OrderBy("created_at DESC")

	if page > 0 && pageSize > 0 {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}

	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.ModelProvider
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

// Total 获取模型提供商总数
func (s *ModelProviderStore) Total(ctx context.Context, opts types.ListModelProviderOptions) (int64, error) {
	query := sq.Select("COUNT(*)").From(s.GetTable())
	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return 0, ErrorSqlBuild(err)
	}

	var total int64
	if err = s.GetReplica(ctx).Get(&total, queryString, args...); err != nil {
		return 0, err
	}
	return total, nil
}
