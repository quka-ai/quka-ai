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
		provider.stores.MemoryBindingStore = NewMemoryBindingStore(provider)
	})
}

type MemoryBindingStore struct {
	CommonFields
}

func NewMemoryBindingStore(provider SqlProviderAchieve) *MemoryBindingStore {
	store := &MemoryBindingStore{}
	store.SetProvider(provider)
	store.SetTable(types.TABLE_MEMORY_BINDING)
	store.SetAllColumns("id", "space_id", "user_id", "memory_id", "context_type", "context_id", "binding_type", "pinned_by", "created_at", "updated_at")
	return store
}

func (s *MemoryBindingStore) Create(ctx context.Context, data types.MemoryBinding) error {
	now := time.Now().Unix()
	if data.CreatedAt == 0 {
		data.CreatedAt = now
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = now
	}

	query := sq.Insert(s.GetTable()).
		Columns(s.GetAllColumns()...).
		Values(data.ID, data.SpaceID, data.UserID, data.MemoryID, data.ContextType, data.ContextID, data.BindingType, data.PinnedBy, data.CreatedAt, data.UpdatedAt).
		Suffix("ON CONFLICT (space_id, user_id, memory_id, context_type, context_id, binding_type) DO NOTHING")

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *MemoryBindingStore) Update(ctx context.Context, spaceID, id string, data types.MemoryBinding) error {
	query := sq.Update(s.GetTable()).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"space_id": spaceID, "id": id})

	if data.MemoryID != "" {
		query = query.Set("memory_id", data.MemoryID)
	}
	if data.UserID != "" {
		query = query.Set("user_id", data.UserID)
	}
	if data.ContextType != "" {
		query = query.Set("context_type", data.ContextType)
	}
	if data.ContextID != "" {
		query = query.Set("context_id", data.ContextID)
	}
	if data.BindingType != "" {
		query = query.Set("binding_type", data.BindingType)
	}
	if data.PinnedBy != "" {
		query = query.Set("pinned_by", data.PinnedBy)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *MemoryBindingStore) Delete(ctx context.Context, spaceID, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *MemoryBindingStore) DeleteByMemoryIDs(ctx context.Context, spaceID string, memoryIDs []string) error {
	if len(memoryIDs) == 0 {
		return nil
	}

	query := deleteMemoryBindingByMemoryIDsQuery(s.GetTable(), spaceID, memoryIDs)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func deleteMemoryBindingByMemoryIDsQuery(tableName, spaceID string, memoryIDs []string) sq.DeleteBuilder {
	query := sq.Delete(tableName).Where(sq.Eq{"memory_id": memoryIDs})
	if spaceID != types.GLOBAL_MEMORY_SPACE_ID {
		query = query.Where(sq.Eq{"space_id": spaceID})
	}
	return query
}

func (s *MemoryBindingStore) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *MemoryBindingStore) List(ctx context.Context, opts types.GetMemoryBindingOptions, page, pageSize uint64) ([]types.MemoryBinding, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable())
	opts.Apply(&query)
	query = query.OrderBy("created_at DESC")
	if page != types.NO_PAGINATION || pageSize != types.NO_PAGINATION {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.MemoryBinding
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
