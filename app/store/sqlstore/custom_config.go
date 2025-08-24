package sqlstore

import (
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.CustomConfigStore = NewCustomConfigStore(provider)
	})
}

// CustomConfigStore 处理自定义配置表的操作
type CustomConfigStore struct {
	CommonFields
}

// NewCustomConfigStore 创建新的 CustomConfigStore 实例
func NewCustomConfigStore(provider SqlProviderAchieve) *CustomConfigStore {
	repo := &CustomConfigStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_CUSTOM_CONFIG)
	repo.SetAllColumns("name", "description", "value", "category", "status", "created_at", "updated_at")
	return repo
}

// Upsert 插入或更新自定义配置
func (s *CustomConfigStore) Upsert(ctx context.Context, data types.CustomConfig) error {
	now := time.Now().Unix()

	// 检查是否存在
	existing, err := s.Get(ctx, data.Name)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if existing != nil {
		// 更新现有记录
		data.UpdatedAt = now
		data.CreatedAt = existing.CreatedAt // 保持原有的创建时间

		query := sq.Update(s.GetTable()).
			SetMap(map[string]interface{}{
				"description": data.Description,
				"value":       data.Value,
				"category":    data.Category,
				"status":      data.Status,
				"updated_at":  data.UpdatedAt,
			}).
			Where(sq.Eq{"name": data.Name})

		queryString, args, err := query.ToSql()
		if err != nil {
			return ErrorSqlBuild(err)
		}

		_, err = s.GetMaster(ctx).Exec(queryString, args...)
		return err
	} else {
		// 插入新记录
		if data.CreatedAt == 0 {
			data.CreatedAt = now
		}
		if data.UpdatedAt == 0 {
			data.UpdatedAt = now
		}

		query := sq.Insert(s.GetTable()).
			Columns("name", "description", "value", "category", "status", "created_at", "updated_at").
			Values(data.Name, data.Description, data.Value, data.Category, data.Status, data.CreatedAt, data.UpdatedAt)

		queryString, args, err := query.ToSql()
		if err != nil {
			return ErrorSqlBuild(err)
		}

		_, err = s.GetMaster(ctx).Exec(queryString, args...)
		return err
	}
}

// Get 根据名称获取自定义配置
func (s *CustomConfigStore) Get(ctx context.Context, name string) (*types.CustomConfig, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"name": name})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var config types.CustomConfig
	if err = s.GetReplica(ctx).Get(&config, queryString, args...); err != nil {
		return nil, err
	}

	return &config, nil
}

// Delete 删除自定义配置
func (s *CustomConfigStore) Delete(ctx context.Context, name string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"name": name})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// List 分页获取自定义配置列表
func (s *CustomConfigStore) List(ctx context.Context, opts types.ListCustomConfigOptions, page, pageSize uint64) ([]types.CustomConfig, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).OrderBy("created_at DESC")

	// 应用过滤条件
	opts.Apply(&query)

	// 分页
	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		query = query.Limit(pageSize).Offset(offset)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var configs []types.CustomConfig
	if err = s.GetReplica(ctx).Select(&configs, queryString, args...); err != nil {
		return nil, err
	}

	return configs, nil
}

// BatchUpsert 批量插入或更新自定义配置（事务安全）
func (s *CustomConfigStore) BatchUpsert(ctx context.Context, configs []types.CustomConfig) error {
	if len(configs) == 0 {
		return nil
	}

	// 开始事务
	tx, err := s.provider.GetMaster().BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 使用事务上下文
	txCtx := context.WithValue(ctx, "tx", tx)

	// 批量执行upsert操作
	for _, config := range configs {
		if err := s.upsertWithTx(txCtx, config); err != nil {
			return err
		}
	}

	// 提交事务
	return tx.Commit()
}

// upsertWithTx 在事务中执行upsert操作
func (s *CustomConfigStore) upsertWithTx(ctx context.Context, data types.CustomConfig) error {
	now := time.Now().Unix()

	// 检查是否存在
	existing, err := s.Get(ctx, data.Name)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if existing != nil {
		// 更新现有记录
		data.UpdatedAt = now
		data.CreatedAt = existing.CreatedAt // 保持原有的创建时间

		query := sq.Update(s.GetTable()).
			SetMap(map[string]interface{}{
				"description": data.Description,
				"value":       data.Value,
				"category":    data.Category,
				"status":      data.Status,
				"updated_at":  data.UpdatedAt,
			}).
			Where(sq.Eq{"name": data.Name})

		queryString, args, err := query.ToSql()
		if err != nil {
			return ErrorSqlBuild(err)
		}

		_, err = s.GetMaster(ctx).Exec(queryString, args...)
		return err
	} else {
		// 插入新记录
		if data.CreatedAt == 0 {
			data.CreatedAt = now
		}
		if data.UpdatedAt == 0 {
			data.UpdatedAt = now
		}

		query := sq.Insert(s.GetTable()).
			Columns("name", "description", "value", "category", "status", "created_at", "updated_at").
			Values(data.Name, data.Description, data.Value, data.Category, data.Status, data.CreatedAt, data.UpdatedAt)

		queryString, args, err := query.ToSql()
		if err != nil {
			return ErrorSqlBuild(err)
		}

		_, err = s.GetMaster(ctx).Exec(queryString, args...)
		return err
	}
}

// Total 获取自定义配置总数
func (s *CustomConfigStore) Total(ctx context.Context, opts types.ListCustomConfigOptions) (int64, error) {
	query := sq.Select("COUNT(*)").From(s.GetTable())

	// 应用过滤条件
	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return 0, ErrorSqlBuild(err)
	}

	var count int64
	if err = s.GetReplica(ctx).Get(&count, queryString, args...); err != nil {
		return 0, err
	}

	return count, nil
}
