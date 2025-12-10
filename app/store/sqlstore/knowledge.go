package sqlstore

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.KnowledgeStore = NewKnowledgeStore(provider)
	})
}

// KnowledgeStore 处理活动表的操作
type KnowledgeStore struct {
	CommonFields
	schema types.Knowledge
}

// NewJhEventsStore 创建新的 JhEventsStore 实例
func NewKnowledgeStore(provider SqlProviderAchieve) *KnowledgeStore {
	store := &KnowledgeStore{}
	store.SetProvider(provider)
	store.SetTable(types.TABLE_KNOWLEDGE)
	store.SetAllColumns("id", "title", "user_id", "space_id", "tags", "content", "content_type", "resource", "kind", "summary", "maybe_date", "stage", "retry_times", "created_at", "updated_at", "expired_at", "rel_doc_id")
	return store
}

// Create 创建新的知识记录
func (s *KnowledgeStore) Create(ctx context.Context, data types.Knowledge) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = time.Now().Unix()
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "title", "user_id", "space_id", "tags", "content", "content_type", "resource", "kind", "summary", "maybe_date", "stage", "retry_times", "created_at", "updated_at", "expired_at", "rel_doc_id").
		Values(data.ID, data.Title, data.UserID, data.SpaceID, pq.Array(data.Tags), data.Content.String(), data.ContentType, data.Resource, data.Kind, data.Summary, data.MaybeDate, data.Stage, data.RetryTimes, data.CreatedAt, data.UpdatedAt, data.ExpiredAt, data.RelDocID)

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

func (s *KnowledgeStore) BatchCreate(ctx context.Context, datas []*types.Knowledge) error {
	query := sq.Insert(s.GetTable()).
		Columns("id", "title", "user_id", "space_id", "tags", "content", "content_type", "resource", "kind", "summary", "maybe_date", "stage", "retry_times", "created_at", "updated_at", "expired_at", "rel_doc_id")
	for _, data := range datas {
		if data.CreatedAt == 0 {
			data.CreatedAt = time.Now().Unix()
		}
		if data.UpdatedAt == 0 {
			data.UpdatedAt = time.Now().Unix()
		}

		query = query.Values(data.ID, data.Title, data.UserID, data.SpaceID, pq.Array(data.Tags), data.Content.String(), data.ContentType, data.Resource, data.Kind, data.Summary, data.MaybeDate, data.Stage, data.RetryTimes, data.CreatedAt, data.UpdatedAt, data.ExpiredAt, data.RelDocID)
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

// GetKnowledge 根据ID获取知识记录
func (s *KnowledgeStore) GetKnowledge(ctx context.Context, spaceID string, id string) (*types.Knowledge, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.Knowledge
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新知识记录
func (s *KnowledgeStore) FinishedStageSummarize(ctx context.Context, spaceID, id string, summary ai.ChunkResult) error {
	query := sq.Update(s.GetTable()).
		Set("stage", types.KNOWLEDGE_STAGE_EMBEDDING).
		// Set("summary", summary.Summary).
		Set("maybe_date", summary.DateTime).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"space_id": spaceID, "id": id})

	if summary.Title != "" {
		query = query.Set("title", summary.Title)
	}

	if len(summary.Tags) != 0 {
		query = query.Set("tags", pq.Array(summary.Tags))
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Update 更新知识记录
func (s *KnowledgeStore) FinishedStageEmbedding(ctx context.Context, spaceID, id string) error {
	query := sq.Update(s.GetTable()).
		Set("stage", types.KNOWLEDGE_STAGE_DONE).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"space_id": spaceID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Update 更新知识记录
func (s *KnowledgeStore) Update(ctx context.Context, spaceID, id string, data types.UpdateKnowledgeArgs) error {
	query := sq.Update(s.GetTable()).
		Set("updated_at", time.Now().Unix()).
		Set("retry_times", 0).
		Where(sq.Eq{"space_id": spaceID, "id": id})

	if data.Title != "" {
		query = query.Set("title", data.Title)
	}

	if len(data.Content) > 0 {
		query = query.Set("content", data.Content.String())
	}

	if data.ContentType != "" {
		query = query.Set("content_type", data.ContentType)
	}

	if data.Stage != 0 {
		query = query.Set("stage", data.Stage)
	}

	if data.Kind != "" {
		query = query.Set("kind", data.Kind)
	}

	if len(data.Tags) != 0 {
		query = query.Set("tags", pq.Array(data.Tags))
	}

	if data.Resource != "" {
		query = query.Set("resource", data.Resource)
	}

	if data.Summary != "" {
		query = query.Set("summary", data.Summary)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *KnowledgeStore) SetRetryTimes(ctx context.Context, spaceID, id string, retryTimes int) error {
	query := sq.Update(s.GetTable()).
		Set("retry_times", retryTimes).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"space_id": spaceID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除知识记录
func (s *KnowledgeStore) Delete(ctx context.Context, spaceID, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除知识记录
func (s *KnowledgeStore) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *KnowledgeStore) BatchDelete(ctx context.Context, ids []string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"id": ids})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *KnowledgeStore) ListFailedKnowledges(ctx context.Context, stage types.KnowledgeStage, retryTimes int, page, pageSize uint64) ([]types.Knowledge, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"stage": stage}, sq.Eq{"retry_times": retryTimes})
	if page != 0 || pageSize != 0 {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.Knowledge
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *KnowledgeStore) ListProcessingKnowledges(ctx context.Context, retryTimes int, page, pageSize uint64) ([]types.Knowledge, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.And{sq.NotEq{"stage": types.KNOWLEDGE_STAGE_DONE}, sq.Lt{"retry_times": retryTimes}})
	if page != 0 || pageSize != 0 {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.Knowledge
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *KnowledgeStore) ListLiteKnowledges(ctx context.Context, opts types.GetKnowledgeOptions, page, pageSize uint64) ([]*types.KnowledgeLite, error) {
	query := sq.Select("id", "title", "space_id", "user_id", "resource", "tags").From(s.GetTable())
	if page != 0 || pageSize != 0 {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}
	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []*types.KnowledgeLite
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

// ListKnowledges 分页获取知识记录列表
func (s *KnowledgeStore) ListKnowledges(ctx context.Context, opts types.GetKnowledgeOptions, page, pageSize uint64) ([]*types.Knowledge, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).OrderBy("created_at DESC")
	if page != 0 || pageSize != 0 {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}
	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	fmt.Println(queryString, args)

	var res []*types.Knowledge
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *KnowledgeStore) ListKnowledgeIDs(ctx context.Context, opts types.GetKnowledgeOptions, page, pageSize uint64) ([]string, error) {
	query := sq.Select("id").From(s.GetTable())
	if page != 0 || pageSize != 0 {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}
	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []string
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *KnowledgeStore) Total(ctx context.Context, opts types.GetKnowledgeOptions) (uint64, error) {
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

// UpdateExpiredAt 更新单个knowledge的过期时间
func (s *KnowledgeStore) UpdateExpiredAt(ctx context.Context, knowledgeID string, expiredAt int64) error {
	query := sq.Update(s.GetTable()).
		Set("expired_at", expiredAt).
		Where(sq.Eq{"id": knowledgeID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}
