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
	store.SetAllColumns("id", "subscription_id", "guid", "title", "link", "description", "content", "author", "published_at", "fetched_at", "created_at")
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
		Columns("id", "subscription_id", "guid", "title", "link", "description", "content", "author", "published_at", "fetched_at", "created_at").
		Values(data.ID, data.SubscriptionID, data.GUID, data.Title, data.Link, data.Description, data.Content, data.Author, data.PublishedAt, data.FetchedAt, data.CreatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Get 根据ID获取文章
func (s *RSSArticleStore) Get(ctx context.Context, id int64) (*types.RSSArticle, error) {
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
func (s *RSSArticleStore) GetByGUID(ctx context.Context, subscriptionID int64, guid string) (*types.RSSArticle, error) {
	var article types.RSSArticle
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"subscription_id": subscriptionID, "guid": guid})

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
func (s *RSSArticleStore) Exists(ctx context.Context, subscriptionID int64, guid string) (bool, error) {
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
func (s *RSSArticleStore) ListBySubscription(ctx context.Context, subscriptionID int64, limit int) ([]*types.RSSArticle, error) {
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

// DeleteOld 删除旧文章（用于清理）
func (s *RSSArticleStore) DeleteOld(ctx context.Context, subscriptionID int64, keepCount int) error {
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
