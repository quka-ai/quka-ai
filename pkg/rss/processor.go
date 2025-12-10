package rss

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// Processor RSS 内容处理器
type Processor struct {
	core *core.Core
}

// NewProcessor 创建新的 RSS 内容处理器实例
func NewProcessor(core *core.Core) *Processor {
	return &Processor{
		core: core,
	}
}

// ProcessArticle 处理单篇RSS文章，创建 Knowledge 记录
func (p *Processor) ProcessArticle(ctx context.Context, subscription *types.RSSSubscription, article *types.RSSArticle) error {
	// 1. 检查文章是否已存在（去重）
	exists, err := p.core.Store().RSSArticleStore().Exists(ctx, subscription.ID, article.GUID)
	if err != nil {
		return fmt.Errorf("failed to check article existence: %w", err)
	}
	if exists {
		slog.Debug("Article already exists, skipping",
			slog.String("guid", article.GUID),
			slog.Int64("subscription_id", subscription.ID))
		return nil
	}

	// 2. 保存文章记录（用于去重）
	article.SubscriptionID = subscription.ID
	article.FetchedAt = time.Now().Unix()
	if err := p.core.Store().RSSArticleStore().Create(ctx, article); err != nil {
		return fmt.Errorf("failed to create article record: %w", err)
	}

	// 3. 获取 Resource 配置以确定过期时间
	resource, err := p.core.Store().ResourceStore().GetResource(ctx, subscription.SpaceID, subscription.ResourceID)
	if err != nil {
		return fmt.Errorf("failed to get resource config: %w", err)
	}

	// 4. 准备 Knowledge 内容
	content := p.buildKnowledgeContent(article)

	// 5. 加密内容
	encryptedContent, err := p.core.EncryptData(types.KnowledgeContent(content))
	if err != nil {
		return fmt.Errorf("failed to encrypt content: %w", err)
	}

	// 6. 计算过期时间
	var expiredAt int64
	if resource.Cycle > 0 {
		expiredAt = time.Now().Add(time.Duration(resource.Cycle) * time.Hour * 24).Unix()
	}

	// 7. 创建 Knowledge 记录
	knowledge := types.Knowledge{
		ID:          utils.GenUniqIDStr(),
		SpaceID:     subscription.SpaceID,
		UserID:      subscription.UserID,
		Resource:    subscription.ResourceID,
		Kind:        "rss",
		ContentType: types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN,
		Content:     encryptedContent,
		RelDocID:    fmt.Sprintf("%d", article.ID),
		Stage:       types.KNOWLEDGE_STAGE_SUMMARIZE, // 需要进行摘要处理
		Title:       article.Title,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
		ExpiredAt:   expiredAt,
		RetryTimes:  0,
	}

	if err := p.core.Store().KnowledgeStore().Create(ctx, knowledge); err != nil {
		return fmt.Errorf("failed to create knowledge: %w", err)
	}

	slog.Info("RSS article processed successfully",
		slog.String("article_title", article.Title),
		slog.String("knowledge_id", knowledge.ID),
		slog.String("subscription_id", fmt.Sprintf("%d", subscription.ID)))

	return nil
}

// ProcessFeed 处理整个 RSS Feed
func (p *Processor) ProcessFeed(ctx context.Context, subscription *types.RSSSubscription, feed *types.RSSFeed) error {
	successCount := 0
	failedCount := 0

	for _, item := range feed.Items {
		article := &types.RSSArticle{
			ID:          utils.GenUniqID(),
			GUID:        item.GUID,
			Title:       item.Title,
			Link:        item.Link,
			Description: item.Description,
			Content:     item.Content,
			Author:      item.Author,
			PublishedAt: item.PublishedAt,
		}

		if err := p.ProcessArticle(ctx, subscription, article); err != nil {
			slog.Error("Failed to process article",
				slog.String("article_title", article.Title),
				slog.String("error", err.Error()))
			failedCount++
			continue
		}

		successCount++
	}

	// 更新订阅的最后抓取时间
	if err := p.core.Store().RSSSubscriptionStore().Update(ctx, subscription.ID, map[string]interface{}{
		"last_fetched_at": time.Now().Unix(),
	}); err != nil {
		slog.Error("Failed to update subscription last_fetched_at",
			slog.Int64("subscription_id", subscription.ID),
			slog.String("error", err.Error()))
	}

	slog.Info("RSS feed processing completed",
		slog.Int64("subscription_id", subscription.ID),
		slog.String("subscription_title", subscription.Title),
		slog.Int("total", len(feed.Items)),
		slog.Int("success", successCount),
		slog.Int("failed", failedCount))

	return nil
}

// buildKnowledgeContent 构建 Knowledge 的 Markdown 内容
func (p *Processor) buildKnowledgeContent(article *types.RSSArticle) string {
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

// ExtractKeywords 从文章中提取关键词（简单实现，可以后续使用AI优化）
func (p *Processor) ExtractKeywords(article *types.RSSArticle) []string {
	// 这里可以实现更复杂的关键词提取逻辑
	// 简单版本：从标题中提取
	keywords := []string{}

	// 可以添加更复杂的NLP处理
	// 目前返回空，等待后续AI增强

	return keywords
}

// UpdateUserInterests 根据文章内容更新用户兴趣模型
func (p *Processor) UpdateUserInterests(ctx context.Context, userID string, topics []string, weight float64) error {
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
		if err := p.core.Store().RSSUserInterestStore().BatchUpsert(ctx, interests); err != nil {
			return fmt.Errorf("failed to update user interests: %w", err)
		}
	}

	return nil
}
