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
		provider.stores.KnowledgeMetaStore = NewKnowledgeMetaStore(provider)
	})
}

// KnowledgeMetaImpl 处理知识元信息表的操作
type KnowledgeMetaImpl struct {
	CommonFields
}

// NewKnowledgeMetaStore 创建新的 KnowledgeMetaStore 实例
func NewKnowledgeMetaStore(provider SqlProviderAchieve) store.KnowledgeMetaStore {
	repo := &KnowledgeMetaImpl{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_KNOWLEDGE_META)
	repo.SetAllColumns("id", "meta_info", "space_id", "created_at")
	return repo
}

// Create 创建新的知识元信息
func (s *KnowledgeMetaImpl) Create(ctx context.Context, data types.KnowledgeMeta) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("id", "meta_info", "space_id", "created_at").
		Values(data.ID, data.MetaInfo, data.SpaceID, data.CreatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// GetKnowledgeMeta 根据ID获取知识元信息
func (s *KnowledgeMetaImpl) GetKnowledgeMeta(ctx context.Context, id string) (*types.KnowledgeMeta, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.KnowledgeMeta
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新知识元信息
func (s *KnowledgeMetaImpl) Update(ctx context.Context, id string, data types.KnowledgeMeta) error {
	query := sq.Update(s.GetTable()).
		Set("meta_info", data.MetaInfo).
		Set("created_at", time.Now().Unix()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除知识元信息
func (s *KnowledgeMetaImpl) Delete(ctx context.Context, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *KnowledgeMetaImpl) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// ListKnowledgeMetas 分页获取知识元信息列表
func (s *KnowledgeMetaImpl) ListKnowledgeMetas(ctx context.Context, ids []string) ([]*types.KnowledgeMeta, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"id": ids})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []*types.KnowledgeMeta
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
