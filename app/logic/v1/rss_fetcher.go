package v1

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/rss"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type RSSFetcherLogic struct {
	ctx       context.Context
	core      *core.Core
	fetcher   *rss.Fetcher
	processor *rss.Processor
}

func NewRSSFetcherLogic(ctx context.Context, core *core.Core) *RSSFetcherLogic {
	return &RSSFetcherLogic{
		ctx:       ctx,
		core:      core,
		fetcher:   rss.NewFetcher(),
		processor: rss.NewProcessor(core),
	}
}

// FetchSubscription 抓取单个订阅的内容
func (l *RSSFetcherLogic) FetchSubscription(subscriptionID int64) error {
	// 获取订阅信息
	subscription, err := l.core.Store().RSSSubscriptionStore().Get(l.ctx, subscriptionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("RSSFetcherLogic.FetchSubscription.NotFound", i18n.ERROR_NOT_FOUND, err)
		}
		return errors.New("RSSFetcherLogic.FetchSubscription.RSSSubscriptionStore.Get", i18n.ERROR_INTERNAL, err)
	}

	// 检查是否启用
	if !subscription.Enabled {
		slog.Debug("Subscription is disabled, skipping",
			slog.Int64("subscription_id", subscriptionID))
		return nil
	}

	// 抓取 RSS Feed（带重试）
	feed, err := l.fetcher.FetchWithRetry(l.ctx, subscription.URL, 3)
	if err != nil {
		slog.Error("Failed to fetch RSS feed",
			slog.Int64("subscription_id", subscriptionID),
			slog.String("url", subscription.URL),
			slog.String("error", err.Error()))
		return errors.New("RSSFetcherLogic.FetchSubscription.Fetcher.FetchWithRetry", i18n.ERROR_INTERNAL, err)
	}

	// 处理 Feed 中的文章
	if err := l.processor.ProcessFeed(l.ctx, subscription, feed); err != nil {
		slog.Error("Failed to process RSS feed",
			slog.Int64("subscription_id", subscriptionID),
			slog.String("error", err.Error()))
		return errors.New("RSSFetcherLogic.FetchSubscription.Processor.ProcessFeed", i18n.ERROR_INTERNAL, err)
	}

	return nil
}

// FetchAllEnabledSubscriptions 抓取所有启用的订阅
func (l *RSSFetcherLogic) FetchAllEnabledSubscriptions() error {
	subscriptions, err := l.core.Store().RSSSubscriptionStore().GetEnabledSubscriptions(l.ctx)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("RSSFetcherLogic.FetchAllEnabledSubscriptions.RSSSubscriptionStore.GetEnabledSubscriptions", i18n.ERROR_INTERNAL, err)
	}

	if len(subscriptions) == 0 {
		slog.Debug("No enabled subscriptions found")
		return nil
	}

	slog.Info("Starting to fetch all enabled RSS subscriptions",
		slog.Int("count", len(subscriptions)))

	successCount := 0
	failedCount := 0

	for _, subscription := range subscriptions {
		if err := l.FetchSubscription(subscription.ID); err != nil {
			slog.Error("Failed to fetch subscription",
				slog.Int64("subscription_id", subscription.ID),
				slog.String("title", subscription.Title),
				slog.String("error", err.Error()))
			failedCount++
			continue
		}
		successCount++
	}

	slog.Info("Finished fetching all enabled RSS subscriptions",
		slog.Int("total", len(subscriptions)),
		slog.Int("success", successCount),
		slog.Int("failed", failedCount))

	return nil
}

// GetArticlesBySubscription 获取订阅的文章列表
func (l *RSSFetcherLogic) GetArticlesBySubscription(subscriptionID int64, limit int) ([]*types.RSSArticle, error) {
	// 检查订阅是否存在
	subscription, err := l.core.Store().RSSSubscriptionStore().Get(l.ctx, subscriptionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("RSSFetcherLogic.GetArticlesBySubscription.NotFound", i18n.ERROR_NOT_FOUND, err)
		}
		return nil, errors.New("RSSFetcherLogic.GetArticlesBySubscription.RSSSubscriptionStore.Get", i18n.ERROR_INTERNAL, err)
	}

	// 获取文章列表
	articles, err := l.core.Store().RSSArticleStore().ListBySubscription(l.ctx, subscription.ID, limit)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("RSSFetcherLogic.GetArticlesBySubscription.RSSArticleStore.ListBySubscription", i18n.ERROR_INTERNAL, err)
	}

	if articles == nil {
		articles = []*types.RSSArticle{}
	}

	return articles, nil
}

// CleanupOldArticles 清理订阅的旧文章（保留最新的N篇）
func (l *RSSFetcherLogic) CleanupOldArticles(subscriptionID int64, keepCount int) error {
	if keepCount <= 0 {
		keepCount = 100 // 默认保留最新100篇
	}

	if err := l.core.Store().RSSArticleStore().DeleteOld(l.ctx, subscriptionID, keepCount); err != nil {
		return errors.New("RSSFetcherLogic.CleanupOldArticles.RSSArticleStore.DeleteOld", i18n.ERROR_INTERNAL, err)
	}

	slog.Info("Cleaned up old articles",
		slog.Int64("subscription_id", subscriptionID),
		slog.Int("keep_count", keepCount))

	return nil
}

// CleanupAllOldArticles 清理所有订阅的旧文章
func (l *RSSFetcherLogic) CleanupAllOldArticles(keepCount int) error {
	subscriptions, err := l.core.Store().RSSSubscriptionStore().GetEnabledSubscriptions(l.ctx)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("RSSFetcherLogic.CleanupAllOldArticles.RSSSubscriptionStore.GetEnabledSubscriptions", i18n.ERROR_INTERNAL, err)
	}

	successCount := 0
	failedCount := 0

	for _, subscription := range subscriptions {
		if err := l.CleanupOldArticles(subscription.ID, keepCount); err != nil {
			slog.Error("Failed to cleanup old articles",
				slog.Int64("subscription_id", subscription.ID),
				slog.String("error", err.Error()))
			failedCount++
			continue
		}
		successCount++
	}

	slog.Info("Finished cleaning up old articles for all subscriptions",
		slog.Int("total", len(subscriptions)),
		slog.Int("success", successCount),
		slog.Int("failed", failedCount))

	return nil
}
