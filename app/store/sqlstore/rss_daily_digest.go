package sqlstore

import (
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.RSSDailyDigestStore = NewRSSDailyDigestStore(provider)
	})
}

// RSSDailyDigestStore 处理RSS每日摘要表的操作
type RSSDailyDigestStore struct {
	CommonFields
}

// NewRSSDailyDigestStore 创建新的 RSSDailyDigestStore 实例
func NewRSSDailyDigestStore(provider SqlProviderAchieve) *RSSDailyDigestStore {
	store := &RSSDailyDigestStore{}
	store.SetProvider(provider)
	store.SetTable(types.TABLE_RSS_DAILY_DIGESTS)
	store.SetAllColumns("id", "user_id", "space_id", "date", "content", "article_ids", "article_count", "ai_model", "generated_at", "created_at")
	return store
}

// Create 创建新的每日摘要
func (s *RSSDailyDigestStore) Create(ctx context.Context, data *types.RSSDailyDigest) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.GeneratedAt == 0 {
		data.GeneratedAt = time.Now().Unix()
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "user_id", "space_id", "date", "content", "article_ids", "article_count", "ai_model", "generated_at", "created_at").
		Values(data.ID, data.UserID, data.SpaceID, data.Date, data.Content, pq.Array(data.ArticleIDs), data.ArticleCount, data.AIModel, data.GeneratedAt, data.CreatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Get 根据ID获取每日摘要
func (s *RSSDailyDigestStore) Get(ctx context.Context, id string) (*types.RSSDailyDigest, error) {
	var digest types.RSSDailyDigest
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&digest, queryString, args...)
	if err != nil {
		return nil, err
	}

	return &digest, nil
}

// GetByUserAndDate 根据用户ID和日期获取每日摘要
func (s *RSSDailyDigestStore) GetByUserAndDate(ctx context.Context, userID, spaceID, date string) (*types.RSSDailyDigest, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"user_id": userID, "space_id": spaceID, "date": date})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var digest types.RSSDailyDigest
	err = s.GetReplica(ctx).Get(&digest, queryString, args...)
	if err != nil {
		return nil, err
	}

	return &digest, nil
}

// ListByUser 获取用户的每日摘要列表（按日期倒序）
func (s *RSSDailyDigestStore) ListByUser(ctx context.Context, userID, spaceID string, limit int) ([]*types.RSSDailyDigest, error) {
	var digests []*types.RSSDailyDigest
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"user_id": userID, "space_id": spaceID}).
		OrderBy("date DESC").
		Limit(uint64(limit))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Select(&digests, queryString, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*types.RSSDailyDigest{}, nil
		}
		return nil, err
	}

	return digests, nil
}

// ListByDateRange 获取指定日期范围内的每日摘要
func (s *RSSDailyDigestStore) ListByDateRange(ctx context.Context, userID, spaceID, startDate, endDate string, limit int) ([]*types.RSSDailyDigest, error) {
	var digests []*types.RSSDailyDigest
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"user_id": userID, "space_id": spaceID}).
		Where(sq.GtOrEq{"date": startDate}).
		Where(sq.LtOrEq{"date": endDate}).
		OrderBy("date DESC").
		Limit(uint64(limit))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Select(&digests, queryString, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*types.RSSDailyDigest{}, nil
		}
		return nil, err
	}

	return digests, nil
}

// Update 更新每日摘要
func (s *RSSDailyDigestStore) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	query := sq.Update(s.GetTable()).
		Where(sq.Eq{"id": id})

	for key, value := range updates {
		query = query.Set(key, value)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除每日摘要
func (s *RSSDailyDigestStore) Delete(ctx context.Context, id string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Exists 检查指定日期的摘要是否已存在
func (s *RSSDailyDigestStore) Exists(ctx context.Context, userID, spaceID, date string) (bool, error) {
	var count int
	query := sq.Select("COUNT(*)").
		From(s.GetTable()).
		Where(sq.Eq{"user_id": userID, "space_id": spaceID, "date": date})

	queryString, args, err := query.ToSql()
	if err != nil {
		return false, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&count, queryString, args...)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// DeleteOld 删除旧的每日摘要（用于清理）
func (s *RSSDailyDigestStore) DeleteOld(ctx context.Context, userID, spaceID string, beforeDate string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{"user_id": userID, "space_id": spaceID}).
		Where(sq.Lt{"date": beforeDate})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}
