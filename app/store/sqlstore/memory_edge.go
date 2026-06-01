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
		provider.stores.MemoryEdgeStore = NewMemoryEdgeStore(provider)
	})
}

type MemoryEdgeStore struct {
	CommonFields
}

func NewMemoryEdgeStore(provider SqlProviderAchieve) *MemoryEdgeStore {
	store := &MemoryEdgeStore{}
	store.SetProvider(provider)
	store.SetTable(types.TABLE_MEMORY_EDGE)
	store.SetAllColumns("id", "space_id", "from_memory_id", "to_memory_id", "relation", "weight", "created_at")
	return store
}

func (s *MemoryEdgeStore) Create(ctx context.Context, data types.MemoryEdge) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}

	query := sq.Insert(s.GetTable()).
		Columns(s.GetAllColumns()...).
		Values(data.ID, data.SpaceID, data.FromMemoryID, data.ToMemoryID, data.Relation, data.Weight, data.CreatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *MemoryEdgeStore) Delete(ctx context.Context, spaceID, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *MemoryEdgeStore) DeleteByMemoryIDs(ctx context.Context, spaceID string, memoryIDs []string) error {
	if len(memoryIDs) == 0 {
		return nil
	}

	query := deleteMemoryEdgeByMemoryIDsQuery(s.GetTable(), spaceID, memoryIDs)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func deleteMemoryEdgeByMemoryIDsQuery(tableName, spaceID string, memoryIDs []string) sq.DeleteBuilder {
	query := sq.Delete(tableName).Where(sq.Or{
		sq.Eq{"from_memory_id": memoryIDs},
		sq.Eq{"to_memory_id": memoryIDs},
	})
	if spaceID != types.GLOBAL_MEMORY_SPACE_ID {
		query = query.Where(sq.Eq{"space_id": spaceID})
	}
	return query
}

func (s *MemoryEdgeStore) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *MemoryEdgeStore) List(ctx context.Context, opts types.GetMemoryEdgeOptions, page, pageSize uint64) ([]types.MemoryEdge, error) {
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

	var res []types.MemoryEdge
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
