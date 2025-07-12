package sqlstore

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/samber/lo"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.ModelConfigStore = NewModelConfigStore(provider)
	})
}

// ModelConfigStore 处理模型配置表的操作
type ModelConfigStore struct {
	CommonFields
}

// NewModelConfigStore 创建新的 ModelConfigStore 实例
func NewModelConfigStore(provider SqlProviderAchieve) *ModelConfigStore {
	repo := &ModelConfigStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_MODEL_CONFIG)
	repo.SetAllColumns("id", "provider_id", "model_name", "display_name", "model_type", "is_multi_modal", "status", "config", "created_at", "updated_at")
	return repo
}

// Create 创建新的模型配置
func (s *ModelConfigStore) Create(ctx context.Context, data types.ModelConfig) error {
	now := time.Now().Unix()
	if data.CreatedAt == 0 {
		data.CreatedAt = now
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = now
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "provider_id", "model_name", "display_name", "model_type", "is_multi_modal", "status", "config", "created_at", "updated_at").
		Values(data.ID, data.ProviderID, data.ModelName, data.DisplayName, data.ModelType, data.IsMultiModal, data.Status, data.Config, data.CreatedAt, data.UpdatedAt)

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

// Get 根据ID获取模型配置
func (s *ModelConfigStore) Get(ctx context.Context, id string) (*types.ModelConfig, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.ModelConfig
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新模型配置
func (s *ModelConfigStore) Update(ctx context.Context, id string, data types.ModelConfig) error {
	data.UpdatedAt = time.Now().Unix()

	query := sq.Update(s.GetTable()).
		SetMap(map[string]interface{}{
			"provider_id":    data.ProviderID,
			"model_name":     data.ModelName,
			"display_name":   data.DisplayName,
			"model_type":     data.ModelType,
			"is_multi_modal": data.IsMultiModal,
			"status":         data.Status,
			"config":         data.Config,
			"updated_at":     data.UpdatedAt,
		}).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除模型配置
func (s *ModelConfigStore) Delete(ctx context.Context, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// DeleteByProviderID 根据提供商ID批量删除模型配置
func (s *ModelConfigStore) DeleteByProviderID(ctx context.Context, providerID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"provider_id": providerID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// List 分页获取模型配置列表
func (s *ModelConfigStore) List(ctx context.Context, opts types.ListModelConfigOptions) ([]types.ModelConfig, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).OrderBy("provider_id ASC, model_name ASC")
	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.ModelConfig
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

// ListWithProvider 分页获取模型配置列表，包含提供商信息
func (s *ModelConfigStore) ListWithProvider(ctx context.Context, opts types.ListModelConfigOptions) ([]*types.ModelConfig, error) {
	// 构建联合查询
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).OrderBy("provider_id ASC, model_name ASC")
	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []*types.ModelConfig
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}

	providers, err := s.getProvider(ctx, lo.Map(res, func(item *types.ModelConfig, _ int) string {
		return item.ProviderID
	}))
	if err != nil {
		return nil, err
	}

	pmap := lo.SliceToMap(providers, func(item *types.ModelProvider) (string, *types.ModelProvider) {
		return item.ID, item
	})

	fmt.Println(pmap)

	for _, item := range res {
		data, exist := pmap[item.ProviderID]
		if exist {
			item.Provider = data
		}
	}

	return res, nil
}

// getProvider 获取提供商信息（内部方法）
func (s *ModelConfigStore) getProvider(ctx context.Context, providerIDs []string) ([]*types.ModelProvider, error) {
	query := sq.Select("id", "name", "description", "api_url", "status", "config", "created_at", "updated_at").
		From(types.TABLE_MODEL_PROVIDER.Name()).
		Where(sq.Eq{"id": providerIDs})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var provider []*types.ModelProvider
	if err = s.GetReplica(ctx).Select(&provider, queryString, args...); err != nil {
		return nil, err
	}
	return provider, nil
}

// Total 获取模型配置总数
func (s *ModelConfigStore) Total(ctx context.Context, opts types.ListModelConfigOptions) (int64, error) {
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
