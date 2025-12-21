package v1

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/rss"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type RSSFetcherLogic struct {
	ctx     context.Context
	core    *core.Core
	fetcher *rss.Fetcher
}

func NewRSSFetcherLogic(ctx context.Context, core *core.Core) *RSSFetcherLogic {
	return &RSSFetcherLogic{
		ctx:     ctx,
		core:    core,
		fetcher: rss.NewFetcher(),
	}
}

// FetchSubscription 抓取单个订阅的内容
// Deprecated: 使用队列机制替代，该方法将在下个版本移除
// 请通过 queue.NewRSSQueue().EnqueueTask() 将订阅推送到队列
func (l *RSSFetcherLogic) FetchSubscription(subscriptionID string) error {
	slog.Warn("Deprecated method FetchSubscription called, please use queue mechanism instead",
		slog.String("subscription_id", subscriptionID))
	return fmt.Errorf("deprecated: use queue mechanism instead")
}

// buildKnowledgeContent 构建 Knowledge 的 Markdown 内容
func (l *RSSFetcherLogic) buildKnowledgeContent(article *types.RSSArticle) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("# %s\n\n", article.Title))

	if article.Author != "" {
		builder.WriteString(fmt.Sprintf("**作者**: %s\n\n", article.Author))
	}

	if article.Link != "" {
		builder.WriteString(fmt.Sprintf("**原文链接**: [%s](%s)\n\n", article.Link, article.Link))
	}

	if article.PublishedAt > 0 {
		publishTime := time.Unix(article.PublishedAt, 0)
		builder.WriteString(fmt.Sprintf("**发布时间**: %s\n\n", publishTime.Format("2006-01-02 15:04:05")))
	}

	builder.WriteString("---\n\n")

	// 优先使用 Content，如果没有则使用 Description
	content := article.Content
	if content == "" {
		content = article.Description
	}

	if content != "" {
		builder.WriteString(content)
	}

	return builder.String()
}

// FetchAllEnabledSubscriptions 抓取所有启用的订阅
// Deprecated: 使用队列机制替代，定时任务会自动将订阅推送到队列
func (l *RSSFetcherLogic) FetchAllEnabledSubscriptions() error {
	slog.Warn("Deprecated method FetchAllEnabledSubscriptions called")
	return fmt.Errorf("deprecated: use queue mechanism instead")
}

// FetchSubscriptionsNeedingUpdate 抓取需要更新的订阅
// Deprecated: 使用队列机制替代，定时任务会自动将需要更新的订阅推送到队列
func (l *RSSFetcherLogic) FetchSubscriptionsNeedingUpdate(limit int) error {
	slog.Warn("Deprecated method FetchSubscriptionsNeedingUpdate called")
	return fmt.Errorf("deprecated: use queue mechanism instead")
}

// GetArticlesBySubscription 获取订阅的文章列表
func (l *RSSFetcherLogic) GetArticlesBySubscription(subscriptionID string, limit int) ([]*types.RSSArticle, error) {
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
func (l *RSSFetcherLogic) CleanupOldArticles(subscriptionID string, keepCount int) error {
	if keepCount <= 0 {
		keepCount = 100 // 默认保留最新100篇
	}

	if err := l.core.Store().RSSArticleStore().DeleteOld(l.ctx, subscriptionID, keepCount); err != nil {
		return errors.New("RSSFetcherLogic.CleanupOldArticles.RSSArticleStore.DeleteOld", i18n.ERROR_INTERNAL, err)
	}

	slog.Info("Cleaned up old articles",
		slog.String("subscription_id", subscriptionID),
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
				slog.String("subscription_id", subscription.ID),
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

// UpdateUserInterests 根据文章内容更新用户兴趣模型
func (l *RSSFetcherLogic) UpdateUserInterests(userID string, topics []string, weight float64) error {
	interests := make([]*types.RSSUserInterest, 0, len(topics))

	for _, topic := range topics {
		interests = append(interests, &types.RSSUserInterest{
			UserID: userID,
			Topic:  topic,
			Weight: weight,
			Source: "implicit", // 隐式学习
		})
	}

	if len(interests) > 0 {
		if err := l.core.Store().RSSUserInterestStore().BatchUpsert(l.ctx, interests); err != nil {
			return fmt.Errorf("failed to update user interests: %w", err)
		}
	}

	return nil
}
