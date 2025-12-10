package v1

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/rss"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type RSSSubscriptionLogic struct {
	ctx  context.Context
	core *core.Core
	UserInfo
}

func NewRSSSubscriptionLogic(ctx context.Context, core *core.Core) *RSSSubscriptionLogic {
	return &RSSSubscriptionLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}
}

// CreateSubscription 创建RSS订阅
func (l *RSSSubscriptionLogic) CreateSubscription(spaceID, resourceID, url, title, description, category string, updateFrequency int) (*types.RSSSubscription, error) {
	user := l.GetUserInfo()
	if !l.core.Srv().RBAC().CheckPermission(user.GetRole(), srv.PermissionEdit) {
		return nil, errors.New("RSSSubscriptionLogic.CreateSubscription.RBAC.CheckPermission", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	// 检查 Resource 是否存在
	_, err := l.core.Store().ResourceStore().GetResource(l.ctx, spaceID, resourceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("RSSSubscriptionLogic.CreateSubscription.ResourceNotFound", i18n.ERROR_NOT_FOUND, err).Code(http.StatusNotFound)
		}
		return nil, errors.New("RSSSubscriptionLogic.CreateSubscription.ResourceStore.GetResource", i18n.ERROR_INTERNAL, err)
	}

	// 检查订阅是否已存在
	existing, err := l.core.Store().RSSSubscriptionStore().GetByUserAndURL(l.ctx, user.User, url)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("RSSSubscriptionLogic.CreateSubscription.RSSSubscriptionStore.GetByUserAndURL", i18n.ERROR_INTERNAL, err)
	}
	if existing != nil {
		return nil, errors.New("RSSSubscriptionLogic.CreateSubscription.SubscriptionExists", i18n.ERROR_EXIST, nil).Code(http.StatusBadRequest)
	}

	// 尝试获取 RSS Feed 信息（验证URL有效性）
	parser := rss.NewParser()
	feed, err := parser.ParseURL(url)
	if err != nil {
		return nil, errors.New("RSSSubscriptionLogic.CreateSubscription.ParseURL", i18n.ERROR_INVALIDARGUMENT, err).Code(http.StatusBadRequest)
	}

	// 如果没有提供标题或描述，使用 Feed 中的信息
	if title == "" {
		title = feed.Title
	}
	if description == "" {
		description = feed.Description
	}

	// 设置默认更新频率（3600秒 = 1小时）
	if updateFrequency <= 0 {
		updateFrequency = 3600
	}

	subscription := &types.RSSSubscription{
		ID:              utils.GenUniqID(),
		UserID:          user.User,
		SpaceID:         spaceID,
		ResourceID:      resourceID,
		URL:             url,
		Title:           title,
		Description:     description,
		Category:        category,
		UpdateFrequency: updateFrequency,
		Enabled:         true,
		CreatedAt:       time.Now().Unix(),
		UpdatedAt:       time.Now().Unix(),
	}

	if err := l.core.Store().RSSSubscriptionStore().Create(l.ctx, subscription); err != nil {
		return nil, errors.New("RSSSubscriptionLogic.CreateSubscription.RSSSubscriptionStore.Create", i18n.ERROR_INTERNAL, err)
	}

	// 立即触发一次抓取
	go func() {
		fetchLogic := NewRSSFetcherLogic(context.Background(), l.core)
		if err := fetchLogic.FetchSubscription(subscription.ID); err != nil {
			// 记录错误但不阻塞创建流程
			fmt.Printf("Failed to fetch new subscription: %v\n", err)
		}
	}()

	return subscription, nil
}

// GetSubscription 获取订阅详情
func (l *RSSSubscriptionLogic) GetSubscription(id int64) (*types.RSSSubscription, error) {
	subscription, err := l.core.Store().RSSSubscriptionStore().Get(l.ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("RSSSubscriptionLogic.GetSubscription.NotFound", i18n.ERROR_NOT_FOUND, err).Code(http.StatusNotFound)
		}
		return nil, errors.New("RSSSubscriptionLogic.GetSubscription.RSSSubscriptionStore.Get", i18n.ERROR_INTERNAL, err)
	}

	// 检查权限
	user := l.GetUserInfo()
	if subscription.UserID != user.User && !l.core.Srv().RBAC().CheckPermission(user.GetRole(), srv.PermissionView) {
		return nil, errors.New("RSSSubscriptionLogic.GetSubscription.PermissionDenied", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	return subscription, nil
}

// ListSubscriptions 获取用户的订阅列表
func (l *RSSSubscriptionLogic) ListSubscriptions(spaceID string) ([]*types.RSSSubscription, error) {
	user := l.GetUserInfo()
	if !l.core.Srv().RBAC().CheckPermission(user.GetRole(), srv.PermissionView) {
		return nil, errors.New("RSSSubscriptionLogic.ListSubscriptions.RBAC.CheckPermission", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	subscriptions, err := l.core.Store().RSSSubscriptionStore().List(l.ctx, user.User, spaceID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("RSSSubscriptionLogic.ListSubscriptions.RSSSubscriptionStore.List", i18n.ERROR_INTERNAL, err)
	}

	if subscriptions == nil {
		subscriptions = []*types.RSSSubscription{}
	}

	return subscriptions, nil
}

// UpdateSubscription 更新订阅配置
func (l *RSSSubscriptionLogic) UpdateSubscription(id int64, title, description, category string, updateFrequency int, enabled *bool) error {
	user := l.GetUserInfo()
	if !l.core.Srv().RBAC().CheckPermission(user.GetRole(), srv.PermissionEdit) {
		return errors.New("RSSSubscriptionLogic.UpdateSubscription.RBAC.CheckPermission", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	// 检查订阅是否存在
	subscription, err := l.core.Store().RSSSubscriptionStore().Get(l.ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("RSSSubscriptionLogic.UpdateSubscription.NotFound", i18n.ERROR_NOT_FOUND, err).Code(http.StatusNotFound)
		}
		return errors.New("RSSSubscriptionLogic.UpdateSubscription.RSSSubscriptionStore.Get", i18n.ERROR_INTERNAL, err)
	}

	// 检查权限
	if subscription.UserID != user.User {
		return errors.New("RSSSubscriptionLogic.UpdateSubscription.PermissionDenied", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	updates := make(map[string]interface{})
	if title != "" {
		updates["title"] = title
	}
	if description != "" {
		updates["description"] = description
	}
	if category != "" {
		updates["category"] = category
	}
	if updateFrequency > 0 {
		updates["update_frequency"] = updateFrequency
	}
	if enabled != nil {
		updates["enabled"] = *enabled
	}

	if len(updates) == 0 {
		return nil
	}

	if err := l.core.Store().RSSSubscriptionStore().Update(l.ctx, id, updates); err != nil {
		return errors.New("RSSSubscriptionLogic.UpdateSubscription.RSSSubscriptionStore.Update", i18n.ERROR_INTERNAL, err)
	}

	return nil
}

// DeleteSubscription 删除订阅
func (l *RSSSubscriptionLogic) DeleteSubscription(id int64) error {
	user := l.GetUserInfo()
	if !l.core.Srv().RBAC().CheckPermission(user.GetRole(), srv.PermissionEdit) {
		return errors.New("RSSSubscriptionLogic.DeleteSubscription.RBAC.CheckPermission", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	// 检查订阅是否存在
	subscription, err := l.core.Store().RSSSubscriptionStore().Get(l.ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("RSSSubscriptionLogic.DeleteSubscription.NotFound", i18n.ERROR_NOT_FOUND, err).Code(http.StatusNotFound)
		}
		return errors.New("RSSSubscriptionLogic.DeleteSubscription.RSSSubscriptionStore.Get", i18n.ERROR_INTERNAL, err)
	}

	// 检查权限
	if subscription.UserID != user.User {
		return errors.New("RSSSubscriptionLogic.DeleteSubscription.PermissionDenied", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	// 删除订阅
	if err := l.core.Store().RSSSubscriptionStore().Delete(l.ctx, id); err != nil {
		return errors.New("RSSSubscriptionLogic.DeleteSubscription.RSSSubscriptionStore.Delete", i18n.ERROR_INTERNAL, err)
	}

	// TODO: 考虑是否要删除相关的 Knowledge 记录
	// 当前设计是不删除，让它们自然过期

	return nil
}

// TriggerFetch 手动触发订阅抓取
func (l *RSSSubscriptionLogic) TriggerFetch(id int64) error {
	user := l.GetUserInfo()
	if !l.core.Srv().RBAC().CheckPermission(user.GetRole(), srv.PermissionEdit) {
		return errors.New("RSSSubscriptionLogic.TriggerFetch.RBAC.CheckPermission", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	// 检查订阅是否存在
	subscription, err := l.core.Store().RSSSubscriptionStore().Get(l.ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("RSSSubscriptionLogic.TriggerFetch.NotFound", i18n.ERROR_NOT_FOUND, err).Code(http.StatusNotFound)
		}
		return errors.New("RSSSubscriptionLogic.TriggerFetch.RSSSubscriptionStore.Get", i18n.ERROR_INTERNAL, err)
	}

	// 检查权限
	if subscription.UserID != user.User {
		return errors.New("RSSSubscriptionLogic.TriggerFetch.PermissionDenied", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	// 异步触发抓取
	go func() {
		fetchLogic := NewRSSFetcherLogic(context.Background(), l.core)
		if err := fetchLogic.FetchSubscription(id); err != nil {
			fmt.Printf("Failed to fetch subscription %d: %v\n", id, err)
		}
	}()

	return nil
}
