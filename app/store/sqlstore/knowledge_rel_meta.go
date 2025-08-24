package sqlstore

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/quka-ai/quka-ai/app/store"
	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.KnowledgeRelMetaStore = NewKnowledgeRelMetaStore(provider)
	})
}

// KnowledgeRelMetaImpl 处理 knowledge_rel_meta 表的操作
type KnowledgeRelMetaImpl struct {
	CommonFields
}

// NewKnowledgeRelMetaStore 创建新的 KnowledgeRelMetaStore 实例
func NewKnowledgeRelMetaStore(provider SqlProviderAchieve) store.KnowledgeRelMetaStore {
	repo := &KnowledgeRelMetaImpl{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_KNOWLEDGE_REL_META)
	repo.SetAllColumns("knowledge_id", "space_id", "meta_id", "chunk_index", "created_at")
	return repo
}

// Create 创建新的 knowledge_rel_meta 记录
func (s *KnowledgeRelMetaImpl) Create(ctx context.Context, data types.KnowledgeRelMeta) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("knowledge_id", "space_id", "meta_id", "chunk_index", "created_at").
		Values(data.KnowledgeID, data.SpaceID, data.MetaID, data.ChunkIndex, data.CreatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *KnowledgeRelMetaImpl) BatchCreate(ctx context.Context, datas []types.KnowledgeRelMeta) error {
	query := sq.Insert(s.GetTable()).
		Columns("knowledge_id", "space_id", "meta_id", "chunk_index", "created_at")
	for _, data := range datas {
		if data.CreatedAt == 0 {
			data.CreatedAt = time.Now().Unix()
		}
		query = query.Values(data.KnowledgeID, data.SpaceID, data.MetaID, data.ChunkIndex, data.CreatedAt)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	if _, err = s.GetMaster(ctx).Exec(queryString, args...); err != nil {
		return err
	}
	return nil
}

// Get 根据 ID 获取 knowledge_rel_meta 记录
func (s *KnowledgeRelMetaImpl) Get(ctx context.Context, id string) (*types.KnowledgeRelMeta, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"knowledge_id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.KnowledgeRelMeta
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 根据 ID 更新 knowledge_rel_meta 记录
func (s *KnowledgeRelMetaImpl) Update(ctx context.Context, id string, data types.KnowledgeRelMeta) error {
	query := sq.Update(s.GetTable()).
		Set("knowledge_id", data.KnowledgeID).
		Set("meta_id", data.MetaID).
		Set("chunk_index", data.ChunkIndex).
		Set("created_at", data.CreatedAt).
		Where(sq.Eq{"knowledge_id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 根据 ID 删除 knowledge_rel_meta 记录
func (s *KnowledgeRelMetaImpl) Delete(ctx context.Context, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"knowledge_id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *KnowledgeRelMetaImpl) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// ListKnowledgesMeta 分页获取 knowledge_rel_meta 记录
func (s *KnowledgeRelMetaImpl) ListKnowledgesMeta(ctx context.Context, knowledgeIDs []string) ([]*types.KnowledgeRelMeta, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"knowledge_id": knowledgeIDs}).
		OrderBy("chunk_index ASC") // 按 chunk_index 排序

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []*types.KnowledgeRelMeta
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *KnowledgeRelMetaImpl) ListRelMetaWithKnowledgeContent(ctx context.Context, opts []types.MergeDataQuery) ([]*types.RelMetaWithKnowledge, error) {
	if len(opts) == 0 {
		return nil, nil
	}
	query := sq.Select("r.meta_id", "r.chunk_index", "k.content", "k.content_type").From(types.TABLE_KNOWLEDGE.Name() + " k").
		Join(s.GetTable() + " r ON k.id = r.knowledge_id")

	or := sq.Or{}
	for _, v := range opts {
		or = append(or, sq.Eq{"r.meta_id": v.MetaID, "r.chunk_index": v.ChunkIDs})
	}

	queryString, args, err := query.Where(or).ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []*types.RelMetaWithKnowledge
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
