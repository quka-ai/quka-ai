package sqlstore

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/pgvector/pgvector-go"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.VectorStore = NewVectorStore(provider)
	})
}

type VectorStore struct {
	CommonFields
}

// NewBwVectorStore 创建新的 BwVectorStore 实例
func NewVectorStore(provider SqlProviderAchieve) *VectorStore {
	repo := &VectorStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_VECTORS)
	repo.SetAllColumns("id", "knowledge_id", "space_id", "user_id", "embedding", "original_length", "created_at", "updated_at")
	return repo
}

// Create 创建新的文本向量记录
func (s *VectorStore) Create(ctx context.Context, data types.Vector) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("id", "knowledge_id", "space_id", "user_id", "resource", "embedding", "original_length", "created_at", "updated_at").
		Values(data.ID, data.KnowledgeID, data.SpaceID, data.UserID, data.Resource, data.Embedding, data.OriginalLength, data.CreatedAt, data.UpdatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	// fmt.Println(queryString, args)

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	if err != nil {
		return err
	}
	return nil
}

// BatchCreate 批量创建新的文本向量记录
func (s *VectorStore) BatchCreate(ctx context.Context, datas []types.Vector) error {
	query := sq.Insert(s.GetTable()).
		Columns("id", "knowledge_id", "space_id", "user_id", "resource", "embedding", "original_length", "created_at", "updated_at")

	for _, data := range datas {
		if data.CreatedAt == 0 {
			data.CreatedAt = time.Now().Unix()
		}
		if data.UpdatedAt == 0 {
			data.UpdatedAt = time.Now().Unix()
		}
		query = query.Values(data.ID, data.KnowledgeID, data.SpaceID, data.UserID, data.Resource, data.Embedding, data.OriginalLength, data.CreatedAt, data.UpdatedAt)
	}

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

// GetBwVector 根据ID获取文本向量记录
func (s *VectorStore) GetVector(ctx context.Context, spaceID, knowledgeID string) (*types.Vector, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "knowledge_id": knowledgeID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.Vector
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新文本向量记录
func (s *VectorStore) Update(ctx context.Context, spaceID, knowledgeID, id string, vector pgvector.Vector) error {
	query := sq.Update(s.GetTable()).
		Set("embedding", vector).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"space_id": spaceID, "knowledge_id": knowledgeID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除文本向量记录
func (s *VectorStore) Delete(ctx context.Context, spaceID, knowledgeID, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "knowledge_id": knowledgeID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *VectorStore) DeleteByResource(ctx context.Context, spaceID, resource string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "resource": resource})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *VectorStore) BatchDelete(ctx context.Context, spaceID, knowledgeID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "knowledge_id": knowledgeID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *VectorStore) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// ListBwVectors 分页获取文本向量记录列表
func (s *VectorStore) ListVectors(ctx context.Context, opts types.GetVectorsOptions, page, pageSize uint64) ([]types.Vector, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Limit(pageSize).Offset((page - 1) * pageSize).OrderBy("created_at DESC")
	opts.Apply(&query)
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.Vector
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

// ListBwVectors 分页获取文本向量记录列表
func (s *VectorStore) Query(ctx context.Context, opts types.GetVectorsOptions, vectors pgvector.Vector, limit uint64) ([]types.QueryResult, error) {
	// pgvector supported distance functions are:
	// <-> - L2 distance
	// <#> - (negative) inner product
	// <=> - cosine distance
	// <+> - L1 distance (added in 0.7.0)
	cosColum, vectorArgs, _ := sq.Expr("1 - (embedding <=> ?) as cos", vectors).ToSql()
	query := sq.Select("id", "knowledge_id", "original_length", cosColum).From(s.GetTable()).Limit(limit).OrderBy("cos DESC")
	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	args = append(vectorArgs, args...)

	// fmt.Println(queryString, args)

	var res []types.QueryResult
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
