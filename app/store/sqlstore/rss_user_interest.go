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
		provider.stores.RSSUserInterestStore = NewRSSUserInterestStore(provider)
	})
}

// RSSUserInterestStore 处理用户兴趣模型表的操作
type RSSUserInterestStore struct {
	CommonFields
}

// NewRSSUserInterestStore 创建新的 RSSUserInterestStore 实例
func NewRSSUserInterestStore(provider SqlProviderAchieve) *RSSUserInterestStore {
	store := &RSSUserInterestStore{}
	store.SetProvider(provider)
	store.SetTable(types.TABLE_RSS_USER_INTERESTS)
	store.SetAllColumns("id", "user_id", "topic", "weight", "source", "last_updated_at", "created_at")
	return store
}

// Create 创建新的用户兴趣记录
func (s *RSSUserInterestStore) Create(ctx context.Context, data *types.RSSUserInterest) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.LastUpdatedAt == 0 {
		data.LastUpdatedAt = time.Now().Unix()
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "user_id", "topic", "weight", "source", "last_updated_at", "created_at").
		Values(data.ID, data.UserID, data.Topic, data.Weight, data.Source, data.LastUpdatedAt, data.CreatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Get 根据ID获取用户兴趣记录
func (s *RSSUserInterestStore) Get(ctx context.Context, id string) (*types.RSSUserInterest, error) {
	var interest types.RSSUserInterest
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&interest, queryString, args...)
	if err != nil {
		return nil, err
	}

	return &interest, nil
}

// GetByUserAndTopic 根据用户ID和主题获取兴趣记录
func (s *RSSUserInterestStore) GetByUserAndTopic(ctx context.Context, userID, topic string) (*types.RSSUserInterest, error) {
	var interest types.RSSUserInterest
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"user_id": userID, "topic": topic})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&interest, queryString, args...)
	if err != nil {
		return nil, err
	}

	return &interest, nil
}

// ListByUser 获取用户的所有兴趣记录
func (s *RSSUserInterestStore) ListByUser(ctx context.Context, userID string, minWeight float64) ([]*types.RSSUserInterest, error) {
	var interests []*types.RSSUserInterest
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"user_id": userID}).
		Where(sq.GtOrEq{"weight": minWeight}).
		OrderBy("weight DESC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Select(&interests, queryString, args...)
	if err != nil {
		return nil, err
	}

	return interests, nil
}

// Upsert 插入或更新用户兴趣记录（基于 user_id + topic 唯一约束）
func (s *RSSUserInterestStore) Upsert(ctx context.Context, data *types.RSSUserInterest) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	data.LastUpdatedAt = time.Now().Unix()

	query := sq.Insert(s.GetTable()).
		Columns("user_id", "topic", "weight", "source", "last_updated_at", "created_at").
		Values(data.UserID, data.Topic, data.Weight, data.Source, data.LastUpdatedAt, data.CreatedAt).
		Suffix("ON CONFLICT (user_id, topic) DO UPDATE SET weight = EXCLUDED.weight, source = EXCLUDED.source, last_updated_at = EXCLUDED.last_updated_at RETURNING id")

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).QueryRowx(queryString, args...).Scan(&data.ID)
	return err
}

// UpdateWeight 更新兴趣权重
func (s *RSSUserInterestStore) UpdateWeight(ctx context.Context, id string, weight float64) error {
	query := sq.Update(s.GetTable()).
		Set("weight", weight).
		Set("last_updated_at", time.Now().Unix()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// BatchUpsert 批量插入或更新用户兴趣记录
func (s *RSSUserInterestStore) BatchUpsert(ctx context.Context, interests []*types.RSSUserInterest) error {
	if len(interests) == 0 {
		return nil
	}

	now := time.Now().Unix()
	query := sq.Insert(s.GetTable()).
		Columns("user_id", "topic", "weight", "source", "last_updated_at", "created_at")

	for _, interest := range interests {
		if interest.CreatedAt == 0 {
			interest.CreatedAt = now
		}
		interest.LastUpdatedAt = now
		query = query.Values(interest.UserID, interest.Topic, interest.Weight, interest.Source, interest.LastUpdatedAt, interest.CreatedAt)
	}

	query = query.Suffix("ON CONFLICT (user_id, topic) DO UPDATE SET weight = EXCLUDED.weight, source = EXCLUDED.source, last_updated_at = EXCLUDED.last_updated_at")

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除用户兴趣记录
func (s *RSSUserInterestStore) Delete(ctx context.Context, id string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// DeleteByUserAndTopic 根据用户ID和主题删除兴趣记录
func (s *RSSUserInterestStore) DeleteByUserAndTopic(ctx context.Context, userID, topic string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{"user_id": userID, "topic": topic})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// GetTopTopics 获取用户权重最高的N个主题
func (s *RSSUserInterestStore) GetTopTopics(ctx context.Context, userID string, limit int) ([]*types.RSSUserInterest, error) {
	var interests []*types.RSSUserInterest
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"user_id": userID}).
		OrderBy("weight DESC").
		Limit(uint64(limit))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Select(&interests, queryString, args...)
	if err != nil {
		return nil, err
	}

	return interests, nil
}
