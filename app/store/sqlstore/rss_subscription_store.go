package sqlstore

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.RSSSubscriptionStore = NewRSSSubscriptionStore(provider)
	})
}

// RSSSubscriptionStore 处理RSS订阅表的操作
type RSSSubscriptionStore struct {
	CommonFields
}

// NewRSSSubscriptionStore 创建新的 RSSSubscriptionStore 实例
func NewRSSSubscriptionStore(provider SqlProviderAchieve) *RSSSubscriptionStore {
	store := &RSSSubscriptionStore{}
	store.SetProvider(provider)
	store.SetTable(types.TABLE_RSS_SUBSCRIPTIONS)
	store.SetAllColumns("id", "user_id", "space_id", "resource_id", "url", "title", "description", "category", "update_frequency", "last_fetched_at", "enabled", "created_at", "updated_at")
	return store
}

// Create 创建新的订阅
func (s *RSSSubscriptionStore) Create(ctx context.Context, data *types.RSSSubscription) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = time.Now().Unix()
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "user_id", "space_id", "resource_id", "url", "title", "description", "category", "update_frequency", "last_fetched_at", "enabled", "created_at", "updated_at").
		Values(data.ID, data.UserID, data.SpaceID, data.ResourceID, data.URL, data.Title, data.Description, data.Category, data.UpdateFrequency, data.LastFetchedAt, data.Enabled, data.CreatedAt, data.UpdatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	fmt.Println(queryString, args)

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Get 根据ID获取订阅
func (s *RSSSubscriptionStore) Get(ctx context.Context, id int64) (*types.RSSSubscription, error) {
	var subscription types.RSSSubscription
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&subscription, queryString, args...)
	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

// GetByUserAndURL 根据用户ID和URL获取订阅（用于检查重复）
func (s *RSSSubscriptionStore) GetByUserAndURL(ctx context.Context, userID, url string) (*types.RSSSubscription, error) {
	var subscription types.RSSSubscription
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"user_id": userID, "url": url})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&subscription, queryString, args...)
	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

// List 获取用户的订阅列表
func (s *RSSSubscriptionStore) List(ctx context.Context, userID string, spaceID string) ([]*types.RSSSubscription, error) {
	var subscriptions []*types.RSSSubscription
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"user_id": userID})

	if spaceID != "" {
		query = query.Where(sq.Eq{"space_id": spaceID})
	}

	query = query.OrderBy("created_at DESC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Select(&subscriptions, queryString, args...)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

// Update 更新订阅
func (s *RSSSubscriptionStore) Update(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().Unix()

	query := sq.Update(s.GetTable()).
		SetMap(updates).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除订阅
func (s *RSSSubscriptionStore) Delete(ctx context.Context, id int64) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// GetEnabledSubscriptions 获取所有启用的订阅（用于定时抓取）
func (s *RSSSubscriptionStore) GetEnabledSubscriptions(ctx context.Context) ([]*types.RSSSubscription, error) {
	var subscriptions []*types.RSSSubscription
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"enabled": true}).
		OrderBy("last_fetched_at ASC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Select(&subscriptions, queryString, args...)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}
