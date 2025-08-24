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
		provider.stores.KnowledgeChunkStore = NewKnowledgeChunkStore(provider)
	})
}

type KnowledgeChunkStore struct {
	CommonFields // 嵌入通用操作字段
}

// NewKnowledgeChunkStore 创建一个新的 KnowledgeChunkStore 实例
func NewKnowledgeChunkStore(provider SqlProviderAchieve) *KnowledgeChunkStore {
	repo := &KnowledgeChunkStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_KNOWLEDGE_CHUNK)
	repo.SetAllColumns("id", "knowledge_id", "space_id", "user_id", "chunk", "original_length", "updated_at", "created_at")
	return repo
}

// Create 创建新的知识片段记录
func (s *KnowledgeChunkStore) Create(ctx context.Context, data types.KnowledgeChunk) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("id", "knowledge_id", "space_id", "user_id", "chunk", "original_length", "updated_at", "created_at").
		Values(data.ID, data.KnowledgeID, data.SpaceID, data.UserID, data.Chunk, data.OriginalLength, data.UpdatedAt, data.CreatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	if _, err = s.GetMaster(ctx).Exec(queryString, args...); err != nil {
		return err
	}
	return nil
}

// BatchCreate 批量创建知识片段记录
func (s *KnowledgeChunkStore) BatchCreate(ctx context.Context, data []*types.KnowledgeChunk) error {
	if len(data) == 0 {
		return nil
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "knowledge_id", "space_id", "user_id", "chunk", "original_length", "updated_at", "created_at")

	// 遍历数据，构建批量插入的 values
	for _, item := range data {
		if item.CreatedAt == 0 {
			item.CreatedAt = time.Now().Unix()
		}
		if item.UpdatedAt == 0 {
			item.UpdatedAt = time.Now().Unix()
		}
		query = query.Values(item.ID, item.KnowledgeID, item.SpaceID, item.UserID, item.Chunk, item.OriginalLength, item.UpdatedAt, item.CreatedAt)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Get 根据ID获取知识片段记录
func (s *KnowledgeChunkStore) Get(ctx context.Context, spaceID, knowledgeID, id string) (*types.KnowledgeChunk, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"space_id": spaceID, "knowledge_id": knowledgeID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.KnowledgeChunk
	if err := s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新知识片段记录
func (s *KnowledgeChunkStore) Update(ctx context.Context, spaceID, knowledgeID, id, chunk string) error {
	query := sq.Update(s.GetTable()).
		Set("chunk", chunk).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"space_id": spaceID, "knowledge_id": knowledgeID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 根据ID删除知识片段记录
func (s *KnowledgeChunkStore) BatchDelete(ctx context.Context, spaceID, knowledgeID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "knowledge_id": knowledgeID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *KnowledgeChunkStore) BatchDeleteByIDs(ctx context.Context, knowledgeIDs []string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"knowledge_id": knowledgeIDs})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 根据ID删除知识片段记录
func (s *KnowledgeChunkStore) Delete(ctx context.Context, spaceID, knowledgeID, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "knowledge_id": knowledgeID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *KnowledgeChunkStore) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// List 获取知识片段列表，支持分页
func (s *KnowledgeChunkStore) List(ctx context.Context, spaceID, knowledgeID string) ([]types.KnowledgeChunk, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "knowledge_id": knowledgeID}).OrderBy("id")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.KnowledgeChunk
	if err := s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
