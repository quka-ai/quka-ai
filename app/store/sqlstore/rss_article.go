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
		provider.stores.RSSArticleStore = NewRSSArticleStore(provider)
	})
}

// RSSArticleStore 处理RSS文章表的操作
type RSSArticleStore struct {
	CommonFields
}

// NewRSSArticleStore 创建新的 RSSArticleStore 实例
func NewRSSArticleStore(provider SqlProviderAchieve) *RSSArticleStore {
	store := &RSSArticleStore{}
	store.SetProvider(provider)
	store.SetTable(types.TABLE_RSS_ARTICLES)
	store.SetAllColumns("id", "subscription_id", "user_id", "guid", "title", "link", "description", "content", "author", "summary", "keywords", "summary_generated_at", "ai_model", "summary_retry_times", "last_summary_error", "published_at", "fetched_at", "created_at")
	return store
}

// Create 创建新的文章
func (s *RSSArticleStore) Create(ctx context.Context, data *types.RSSArticle) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.FetchedAt == 0 {
		data.FetchedAt = time.Now().Unix()
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "subscription_id", "user_id", "guid", "title", "link", "description", "content", "author", "summary", "keywords", "summary_generated_at", "ai_model", "summary_retry_times", "last_summary_error", "published_at", "fetched_at", "created_at").
		Values(data.ID, data.SubscriptionID, data.UserID, data.GUID, data.Title, data.Link, data.Description, data.Content, data.Author, data.Summary, data.Keywords, data.SummaryGeneratedAt, data.AIModel, data.SummaryRetryTimes, data.LastSummaryError, data.PublishedAt, data.FetchedAt, data.CreatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Get 根据ID获取文章
func (s *RSSArticleStore) Get(ctx context.Context, id string) (*types.RSSArticle, error) {
	var article types.RSSArticle
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&article, queryString, args...)
	if err != nil {
		return nil, err
	}

	return &article, nil
}

// GetByGUID 根据订阅ID和GUID获取文章（用于去重）
func (s *RSSArticleStore) GetByGUID(ctx context.Context, guid string) (*types.RSSArticle, error) {
	var article types.RSSArticle
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"guid": guid})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&article, queryString, args...)
	if err != nil {
		return nil, err
	}

	return &article, nil
}

// Exists 检查文章是否已存在
func (s *RSSArticleStore) Exists(ctx context.Context, subscriptionID string, guid string) (bool, error) {
	var count int
	query := sq.Select("COUNT(*)").
		From(s.GetTable()).
		Where(sq.Eq{"subscription_id": subscriptionID, "guid": guid})

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

// ListBySubscription 获取订阅的文章列表
func (s *RSSArticleStore) ListBySubscription(ctx context.Context, subscriptionID string, limit int) ([]*types.RSSArticle, error) {
	var articles []*types.RSSArticle
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"subscription_id": subscriptionID}).
		OrderBy("published_at DESC").
		Limit(uint64(limit))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Select(&articles, queryString, args...)
	if err != nil {
		return nil, err
	}

	return articles, nil
}

// DeleteBySubscription 删除特定订阅的所有文章
func (s *RSSArticleStore) DeleteBySubscription(ctx context.Context, subscriptionID string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{"subscription_id": subscriptionID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// DeleteOld 删除旧文章（用于清理）
func (s *RSSArticleStore) DeleteOld(ctx context.Context, subscriptionID string, keepCount int) error {
	// 保留最新的 keepCount 篇文章，删除其他的
	subQuery := sq.Select("id").
		From(s.GetTable()).
		Where(sq.Eq{"subscription_id": subscriptionID}).
		OrderBy("published_at DESC").
		Limit(uint64(keepCount))

	subQueryString, subArgs, err := subQuery.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{"subscription_id": subscriptionID}).
		Where(sq.Expr("id NOT IN ("+subQueryString+")", subArgs...))

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// UpdateSummary 更新文章摘要（所有用户共享）
func (s *RSSArticleStore) UpdateSummary(ctx context.Context, articleID string, summary *types.RSSArticleSummary) error {
	query := sq.Update(s.GetTable()).
		Set("summary", summary.Summary).
		Set("keywords", pq.Array(summary.Keywords)).
		Set("summary_generated_at", summary.SummaryGeneratedAt).
		Set("ai_model", summary.AIModel).
		Set("last_summary_error", ""). // 清除错误信息
		Where(sq.Eq{"id": articleID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// IncrementSummaryRetry 增加摘要生成重试次数并记录错误
func (s *RSSArticleStore) IncrementSummaryRetry(ctx context.Context, articleID string, errorMsg string) error {
	query := sq.Update(s.GetTable()).
		Set("summary_retry_times", sq.Expr("summary_retry_times + 1")).
		Set("last_summary_error", errorMsg).
		Where(sq.Eq{"id": articleID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// ListWithoutSummary 获取没有摘要的文章列表（排除重试次数过多的文章）
func (s *RSSArticleStore) ListWithoutSummary(ctx context.Context, subscriptionID string, limit int) ([]*types.RSSArticle, error) {
	const maxRetryTimes = 3 // 最大重试次数

	var articles []*types.RSSArticle
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"subscription_id": subscriptionID}).
		Where(sq.Or{
			sq.Eq{"summary": ""},
			sq.Eq{"summary": nil},
		}).
		Where(sq.Lt{"summary_retry_times": maxRetryTimes}). // 只查询重试次数小于最大值的文章
		OrderBy("published_at DESC").
		Limit(uint64(limit))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Select(&articles, queryString, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*types.RSSArticle{}, nil
		}
		return nil, err
	}

	return articles, nil
}

// ListByDateRange 获取指定日期范围内的文章
func (s *RSSArticleStore) ListByDateRange(ctx context.Context, subscriptionID string, startTime, endTime int64, limit int) ([]*types.RSSArticle, error) {
	var articles []*types.RSSArticle
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"subscription_id": subscriptionID}).
		Where(sq.GtOrEq{"published_at": startTime}).
		Where(sq.Lt{"published_at": endTime}).
		OrderBy("published_at DESC").
		Limit(uint64(limit))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Select(&articles, queryString, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*types.RSSArticle{}, nil
		}
		return nil, err
	}

	return articles, nil
}
