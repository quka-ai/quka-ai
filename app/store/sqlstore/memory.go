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
		provider.stores.MemoryStore = NewMemoryStore(provider)
	})
}

type MemoryStore struct {
	CommonFields
}

func NewMemoryStore(provider SqlProviderAchieve) *MemoryStore {
	store := &MemoryStore{}
	store.SetProvider(provider)
	store.SetTable(types.TABLE_MEMORY)
	store.SetAllColumns(
		"id", "space_id", "user_id", "knowledge_id", "memory_type", "scope", "status",
		"importance", "confidence", "author_type", "epistemic_status", "source_kind",
		"source_ref", "entity_key", "dedupe_key", "conflict_state", "valid_from", "valid_to",
		"last_accessed_at", "access_count", "created_at", "updated_at",
	)
	return store
}

func (s *MemoryStore) Create(ctx context.Context, data types.Memory) error {
	now := time.Now().Unix()
	if data.CreatedAt == 0 {
		data.CreatedAt = now
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = now
	}

	query := sq.Insert(s.GetTable()).
		Columns(s.GetAllColumns()...).
		Values(
			data.ID, data.SpaceID, data.UserID, data.KnowledgeID, data.MemoryType, data.Scope, data.Status,
			data.Importance, data.Confidence, data.AuthorType, data.EpistemicStatus, data.SourceKind,
			data.SourceRef, data.EntityKey, data.DedupeKey, data.ConflictState, data.ValidFrom, data.ValidTo,
			data.LastAccessedAt, data.AccessCount, data.CreatedAt, data.UpdatedAt,
		)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *MemoryStore) Get(ctx context.Context, spaceID, id string) (*types.Memory, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{
		"space_id": spaceID,
		"id":       id,
	})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.Memory
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *MemoryStore) GetByKnowledgeID(ctx context.Context, spaceID, knowledgeID string) (*types.Memory, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{
		"space_id":     spaceID,
		"knowledge_id": knowledgeID,
	})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.Memory
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *MemoryStore) Update(ctx context.Context, spaceID, id string, data types.UpdateMemoryArgs) error {
	query := sq.Update(s.GetTable()).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"space_id": spaceID, "id": id})

	if data.Status != "" {
		query = query.Set("status", data.Status)
	}
	if data.Importance != nil {
		query = query.Set("importance", *data.Importance)
	}
	if data.Confidence != nil {
		query = query.Set("confidence", *data.Confidence)
	}
	if data.AuthorType != "" {
		query = query.Set("author_type", data.AuthorType)
	}
	if data.EpistemicStatus != "" {
		query = query.Set("epistemic_status", data.EpistemicStatus)
	}
	if data.EntityKey != nil {
		query = query.Set("entity_key", *data.EntityKey)
	}
	if data.DedupeKey != nil {
		query = query.Set("dedupe_key", *data.DedupeKey)
	}
	if data.ConflictState != "" {
		query = query.Set("conflict_state", data.ConflictState)
	}
	if data.ValidFrom != nil {
		query = query.Set("valid_from", *data.ValidFrom)
	}
	if data.ValidTo != nil {
		query = query.Set("valid_to", *data.ValidTo)
	}
	if data.LastAccessedAt != nil {
		query = query.Set("last_accessed_at", *data.LastAccessedAt)
	}
	if data.AccessCount != nil {
		query = query.Set("access_count", *data.AccessCount)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *MemoryStore) Delete(ctx context.Context, spaceID, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *MemoryStore) DeleteByIDs(ctx context.Context, spaceID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "id": ids})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *MemoryStore) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *MemoryStore) List(ctx context.Context, opts types.GetMemoryOptions, page, pageSize uint64) ([]types.Memory, error) {
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

	var res []types.Memory
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *MemoryStore) Total(ctx context.Context, opts types.GetMemoryOptions) (uint64, error) {
	query := sq.Select("COUNT(*)").From(s.GetTable())
	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return 0, ErrorSqlBuild(err)
	}

	var total uint64
	if err = s.GetReplica(ctx).Get(&total, queryString, args...); err != nil {
		return 0, err
	}
	return total, nil
}
