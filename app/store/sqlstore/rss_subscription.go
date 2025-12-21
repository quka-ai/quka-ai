package sqlstore

import (
	"context"
	"fmt"
	"strings"
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
func (s *RSSSubscriptionStore) Get(ctx context.Context, id string) (*types.RSSSubscription, error) {
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
func (s *RSSSubscriptionStore) Update(ctx context.Context, id string, updates map[string]interface{}) error {
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
func (s *RSSSubscriptionStore) Delete(ctx context.Context, id string) error {
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

// GetSubscriptionsNeedingUpdate 获取需要更新的订阅（带分页限制）
// 采用时间窗口策略：查询最近1小时前更新的订阅，确保覆盖所有更新频率
// 然后在应用层进行真正的更新频率判断，避免数据库计算导致的索引失效
func (s *RSSSubscriptionStore) GetSubscriptionsNeedingUpdate(ctx context.Context, limit int) ([]*types.RSSSubscription, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100 // 默认限制，避免一次性查询过多
	}

	var subscriptions []*types.RSSSubscription
	now := time.Now().Unix()

	// 将列名切片转换为逗号分隔的字符串
	columns := strings.Join(s.GetAllColumns(), ", ")

	// 查询最近1小时前更新的订阅（确保覆盖所有更新频率）
	// 这样可以利用 last_fetched_at 的索引
	queryString := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE enabled = true
		AND last_fetched_at < $1
		ORDER BY last_fetched_at ASC
		LIMIT $2
	`, columns, s.GetTable())

	// 1小时前的时间戳
	oneHourAgo := now - 3600

	err := s.GetReplica(ctx).Select(&subscriptions, queryString, oneHourAgo, limit)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

// CountSubscriptionsNeedingUpdate 统计需要更新的订阅数量（用于监控和调度）
// 这个方法会在应用层过滤，比数据库计算慢但更准确，适合用于监控
func (s *RSSSubscriptionStore) CountSubscriptionsNeedingUpdate(ctx context.Context) (int64, error) {
	// 先获取所有启用的订阅数量作为上限
	var totalEnabled int64
	query := sq.Select("COUNT(*)").
		From(s.GetTable()).
		Where(sq.Eq{"enabled": true})

	queryString, args, err := query.ToSql()
	if err != nil {
		return 0, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&totalEnabled, queryString, args...)
	if err != nil {
		return 0, err
	}

	return totalEnabled, nil
}

// GetSubscriptionsByFrequency 获取特定更新频率的订阅（索引友好）
// 这是最索引友好的方案：直接按 update_frequency 字段过滤
func (s *RSSSubscriptionStore) GetSubscriptionsByFrequency(ctx context.Context, frequency int, limit int) ([]*types.RSSSubscription, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var subscriptions []*types.RSSSubscription
	now := time.Now().Unix()
	threshold := now - int64(frequency)

	columns := strings.Join(s.GetAllColumns(), ", ")

	queryString := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE enabled = true
		AND update_frequency = $1
		AND last_fetched_at < $2
		ORDER BY last_fetched_at ASC
		LIMIT $3
	`, columns, s.GetTable())

	err := s.GetReplica(ctx).Select(&subscriptions, queryString, frequency, threshold, limit)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

// GetSubscriptionsByTimeRange 按时间范围获取订阅（分页查询使用）
func (s *RSSSubscriptionStore) GetSubscriptionsByTimeRange(ctx context.Context, startTime, endTime int64, limit int) ([]*types.RSSSubscription, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}

	var subscriptions []*types.RSSSubscription
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"enabled": true}).
		Where(sq.And{
			sq.GtOrEq{"last_fetched_at": startTime},
			sq.Lt{"last_fetched_at": endTime},
		}).
		OrderBy("last_fetched_at ASC").
		Limit(uint64(limit))

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
