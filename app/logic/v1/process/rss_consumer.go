package process

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/lib/pq"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/queue"
	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/rss"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

func init() {
	register.RegisterFunc[*Process](ProcessKey{}, func(p *Process) {
		// 启动 RSS 消费者
		go startRSSConsumer(p)

		// 每5分钟检查一次需要更新的订阅
		p.Cron().AddFunc("*/5 * * * *", func() {
			enqueueSubscriptionsNeedingUpdate(p)
		})

		slog.Info("RSS task consumers started")
	})
}

// startRSSConsumer 启动 Asynq worker
func startRSSConsumer(p *Process) {
	core := p.Core()

	// 获取或创建 asynq Client 和 Server
	client := p.AsynqClient()
	server := p.AsynqServerMux()
	if client == nil || server == nil {
		slog.Error("Asynq client or server not initialized")
		return
	}

	// 使用共享的 Client 和 Server 创建 RSSQueue，并发数为 3
	rssQueue := queue.NewRSSQueueWithClientServer(core.Cfg().Redis.KeyPrefix, client)

	// 保存 RSSQueue 实例到 Process，以便在 Stop 时关闭
	p.SetRSSQueue(rssQueue)

	mux := p.AsynqServerMux()
	mux.HandleFunc(queue.TaskTypeRSSFetch, func(ctx context.Context, task *asynq.Task) error {
		// 从任务中解析订阅 ID
		var payload queue.RSSFetchTask
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			slog.Error("Failed to unmarshal task payload", slog.String("error", err.Error()))
			return err
		}

		slog.Info("Processing RSS task",
			slog.String("subscription_id", payload.SubscriptionID))

		// 获取订阅信息
		subscription, err := core.Store().RSSSubscriptionStore().Get(ctx, payload.SubscriptionID)
		if err != nil {
			slog.Error("Failed to get subscription",
				slog.String("subscription_id", payload.SubscriptionID),
				slog.String("error", err.Error()))
			return err
		}

		// 处理订阅
		if err := processSubscription(ctx, core, rss.NewFetcher(), subscription); err != nil {
			slog.Error("Failed to process subscription",
				slog.String("subscription_id", payload.SubscriptionID),
				slog.String("error", err.Error()))
			return err
		}

		slog.Info("RSS task completed",
			slog.String("subscription_id", payload.SubscriptionID))

		return nil
	})
}

// enqueueSubscriptionsNeedingUpdate 将需要更新的订阅推送到队列
func enqueueSubscriptionsNeedingUpdate(p *Process) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	slog.Debug("Checking subscriptions needing update")

	// 获取需要更新的订阅（最多100个）
	subscriptions, err := p.core.Store().RSSSubscriptionStore().GetSubscriptionsNeedingUpdate(ctx, 100)
	if err != nil {
		slog.Error("Failed to get subscriptions needing update", slog.String("error", err.Error()))
		return
	}

	if len(subscriptions) == 0 {
		slog.Debug("No subscriptions need update")
		return
	}

	slog.Info("Found subscriptions needing update, enqueuing tasks",
		slog.Int("count", len(subscriptions)))

	// 只需要入队任务，不需要 Server，并发数传 0
	successCount := 0
	failedCount := 0

	for _, subscription := range subscriptions {
		// 推送到队列
		if err := p.rssQueue.EnqueueTask(ctx, subscription.ID); err != nil {
			failedCount++
			slog.Error("Failed to enqueue task",
				slog.String("subscription_id", subscription.ID),
				slog.String("error", err.Error()))
		} else {
			successCount++
		}
	}

	slog.Info("Finished enqueuing tasks",
		slog.Int("total", len(subscriptions)),
		slog.Int("success", successCount),
		slog.Int("failed", failedCount))
}

// processSubscription 处理单个订阅
func processSubscription(ctx context.Context, core *core.Core, fetcher *rss.Fetcher, subscription *types.RSSSubscription) error {
	// 检查是否启用
	if !subscription.Enabled {
		return nil
	}

	// 抓取 RSS Feed（带重试）
	feed, err := fetcher.FetchWithRetry(ctx, subscription.URL, 3)
	if err != nil {
		return fmt.Errorf("failed to fetch RSS feed: %w", err)
	}

	// 处理 Feed 中的文章
	successCount := 0
	failedCount := 0

	for _, item := range feed.Items {
		// 确保所有字段都有初始值，避免数据库 NOT NULL 约束错误
		article := &types.RSSArticle{
			ID:                 utils.GenUniqIDStr(),
			GUID:               item.GUID,
			Title:              item.Title,
			Link:               item.Link,
			Description:        item.Description,
			Content:            item.Content,
			Author:             item.Author,
			PublishedAt:        item.PublishedAt,
			UserID:             subscription.UserID, // 初始为空，创建时由上层设置
			Summary:            "",                  // 初始为空，等待AI生成
			Keywords:           pq.StringArray{},    // 初始为空数组
			SummaryGeneratedAt: 0,                   // 初始为0
			AIModel:            "",                  // 初始为空
			SummaryRetryTimes:  0,                   // 初始为0
			LastSummaryError:   "",                  // 初始为空
		}

		if err := processArticle(ctx, core, subscription, article); err != nil {
			failedCount++
			slog.Error("Failed to process article",
				slog.String("article_title", article.Title),
				slog.String("error", err.Error()))
		} else {
			successCount++
		}
	}

	// 更新订阅的最后抓取时间
	if err := core.Store().RSSSubscriptionStore().Update(ctx, subscription.ID, map[string]interface{}{
		"last_fetched_at": time.Now().Unix(),
	}); err != nil {
		slog.Error("Failed to update subscription last_fetched_at",
			slog.String("subscription_id", subscription.ID),
			slog.String("error", err.Error()))
	}

	slog.Info("RSS feed processing completed",
		slog.String("subscription_id", subscription.ID),
		slog.String("subscription_title", subscription.Title),
		slog.Int("total", len(feed.Items)),
		slog.Int("success", successCount),
		slog.Int("failed", failedCount))

	return nil
}

// processArticle 处理单篇 RSS 文章，创建 Knowledge 记录
func processArticle(ctx context.Context, core *core.Core, subscription *types.RSSSubscription, article *types.RSSArticle) error {
	// 1. 检查文章是否已存在（去重）
	existingArticle, err := core.Store().RSSArticleStore().GetByGUID(ctx, article.GUID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check article existence: %w", err)
	}

	var articleID string
	isNewArticle := false

	if existingArticle == nil {
		// 2. 文章不存在，创建新记录
		article.SubscriptionID = subscription.ID
		article.UserID = subscription.UserID
		article.FetchedAt = time.Now().Unix()
		if err := core.Store().RSSArticleStore().Create(ctx, article); err != nil {
			return fmt.Errorf("failed to create article record: %w", err)
		}
		articleID = article.ID
		isNewArticle = true

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
			defer cancel()
			NewRSSSummarizerLogic(ctx, core).GenerateArticleSummaryWithTokenTracking(article.ID, subscription.SpaceID)
		}()
	} else {
		// 文章已存在，使用已有记录
		articleID = existingArticle.ID
		article = existingArticle
	}

	// 3. 检查用户是否已有此文章的 Knowledge（避免重复创建）
	existingKnowledge, err := core.Store().KnowledgeStore().GetByRelDocID(ctx, subscription.UserID, articleID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing knowledge: %w", err)
	}

	if existingKnowledge != nil {
		// 用户已有这篇文章的 Knowledge
		return nil
	}

	// 4. 获取 Resource 配置以确定过期时间
	resource, err := core.Store().ResourceStore().GetResource(ctx, subscription.SpaceID, subscription.ResourceID)
	if err != nil {
		return fmt.Errorf("failed to get resource config: %w", err)
	}

	// 5. 准备 Knowledge 内容
	content := buildKnowledgeContent(article)

	// 6. 加密内容
	encryptedContent, err := core.EncryptData(types.KnowledgeContent(content))
	if err != nil {
		return fmt.Errorf("failed to encrypt content: %w", err)
	}

	// 7. 计算过期时间
	var expiredAt int64
	if resource.Cycle > 0 {
		expiredAt = time.Now().Add(time.Duration(resource.Cycle) * time.Hour * 24).Unix()
	}

	// 8. 创建用户的 Knowledge 记录（每个用户都有自己的副本）
	knowledge := types.Knowledge{
		ID:          utils.GenUniqIDStr(),
		SpaceID:     subscription.SpaceID,
		UserID:      subscription.UserID,
		Resource:    subscription.ResourceID,
		Kind:        types.KNOWLEDGE_KIND_RSS,
		ContentType: types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN,
		Content:     encryptedContent,
		RelDocID:    articleID,
		Stage:       types.KNOWLEDGE_STAGE_SUMMARIZE,
		Title:       article.Title,
		Source:      types.KNOWLEDGE_SOURCE_RSS.String(),
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
		ExpiredAt:   expiredAt,
		RetryTimes:  0,
	}

	if err := core.Store().KnowledgeStore().Create(ctx, knowledge); err != nil {
		return fmt.Errorf("failed to create knowledge: %w", err)
	}

	slog.Info("RSS article processed successfully",
		slog.String("article_title", article.Title),
		slog.String("article_id", articleID),
		slog.String("knowledge_id", knowledge.ID),
		slog.String("user_id", subscription.UserID),
		slog.Bool("new_article", isNewArticle))

	return nil
}

// buildKnowledgeContent 构建 Knowledge 的 Markdown 内容
func buildKnowledgeContent(article *types.RSSArticle) string {
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
