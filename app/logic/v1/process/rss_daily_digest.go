package process

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Process](ProcessKey{}, func(p *Process) {
		// 每天凌晨 4 点执行每日摘要生成
		p.Cron().AddFunc("0 4 * * *", func() {
			generateDailyDigestForAllUsers(p.Core())
		})

		slog.Info("RSS daily digest scheduler registered: runs at 04:00 AM every day")
	})
}

// generateDailyDigestForAllUsers 为所有用户生成前一天的RSS每日摘要
func generateDailyDigestForAllUsers(core *core.Core) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	slog.Info("Starting daily digest generation for all users")

	// 前一天的日期（使用UTC时间，确保所有时区用户统一）
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	dateStr := yesterday.Format("2006-01-02")

	// 获取所有有RSS订阅的用户
	users, err := getUsersWithRSSSubscriptions(ctx, core)
	if err != nil {
		slog.Error("Failed to get users with RSS subscriptions", slog.String("error", err.Error()))
		return
	}

	if len(users) == 0 {
		slog.Info("No users with RSS subscriptions found")
		return
	}

	slog.Info("Generating daily digests",
		slog.Int("user_count", len(users)),
		slog.String("date", dateStr))

	successCount := 0
	errorCount := 0

	for _, user := range users {
		// 检查是否已生成摘要
		exists, err := core.Store().RSSDailyDigestStore().Exists(ctx, user.UserID, user.SpaceID, dateStr)
		if err != nil {
			slog.Error("Failed to check digest existence",
				slog.String("user_id", user.UserID),
				slog.String("space_id", user.SpaceID),
				slog.String("error", err.Error()))
			errorCount++
			continue
		}

		if exists {
			slog.Debug("Daily digest already exists",
				slog.String("user_id", user.UserID),
				slog.String("space_id", user.SpaceID),
				slog.String("date", dateStr))
			continue
		}

		// 获取用户当天的RSS文章
		articles, err := getUserDailyArticles(ctx, core, user.UserID, user.SpaceID, yesterday)
		if err != nil {
			slog.Error("Failed to get user daily articles",
				slog.String("user_id", user.UserID),
				slog.String("space_id", user.SpaceID),
				slog.String("error", err.Error()))
			errorCount++
			continue
		}

		if len(articles) == 0 {
			slog.Info("No articles found for user, skipping digest generation",
				slog.String("user_id", user.UserID),
				slog.String("space_id", user.SpaceID),
				slog.String("date", dateStr))
			continue
		}

		// 生成AI摘要
		digestContent, model, err := generateIntegratedDigest(ctx, core, articles, yesterday)
		if err != nil {
			slog.Error("Failed to generate digest content",
				slog.String("user_id", user.UserID),
				slog.String("space_id", user.SpaceID),
				slog.String("error", err.Error()))
			errorCount++
			continue
		}

		// 保存到数据库
		digest := &types.RSSDailyDigest{
			UserID:       user.UserID,
			SpaceID:      user.SpaceID,
			Date:         dateStr,
			Content:      digestContent,
			ArticleIDs:   articles,
			ArticleCount: len(articles),
			AIModel:      model,
			GeneratedAt:  time.Now().Unix(),
			CreatedAt:    time.Now().Unix(),
		}

		if err := core.Store().RSSDailyDigestStore().Create(ctx, digest); err != nil {
			slog.Error("Failed to save daily digest",
				slog.String("user_id", user.UserID),
				slog.String("space_id", user.SpaceID),
				slog.String("error", err.Error()))
			errorCount++
			continue
		}

		slog.Info("Daily digest generated successfully",
			slog.String("user_id", user.UserID),
			slog.String("space_id", user.SpaceID),
			slog.String("date", dateStr),
			slog.Int("article_count", len(articles)))

		successCount++

		// 避免过于频繁的 AI 调用
		select {
		case <-ctx.Done():
			slog.Warn("Daily digest generation cancelled due to timeout")
			return
		case <-time.After(2 * time.Second):
		}
	}

	slog.Info("Daily digest generation completed",
		slog.String("date", dateStr),
		slog.Int("success_count", successCount),
		slog.Int("error_count", errorCount),
		slog.Int("skipped_no_articles", len(users)-successCount-errorCount),
		slog.Int("total_users", len(users)))
}

// UserWithRSS 有RSS订阅的用户信息
type UserWithRSS struct {
	UserID  string
	SpaceID string
}

// getUsersWithRSSSubscriptions 获取所有有RSS订阅的用户
func getUsersWithRSSSubscriptions(ctx context.Context, core *core.Core) ([]*UserWithRSS, error) {
	// 获取所有启用的订阅
	subscriptions, err := core.Store().RSSSubscriptionStore().GetEnabledSubscriptions(ctx)
	if err != nil {
		return nil, err
	}

	if len(subscriptions) == 0 {
		return []*UserWithRSS{}, nil
	}

	// 去重，收集唯一的 (user_id, space_id) 组合
	userMap := make(map[string]bool)
	var users []*UserWithRSS

	for _, sub := range subscriptions {
		key := sub.UserID + ":" + sub.SpaceID
		if !userMap[key] {
			userMap[key] = true
			users = append(users, &UserWithRSS{
				UserID:  sub.UserID,
				SpaceID: sub.SpaceID,
			})
		}
	}

	return users, nil
}

// getUserDailyArticles 获取用户在指定日期的RSS文章ID列表
func getUserDailyArticles(ctx context.Context, core *core.Core, userID, spaceID string, date time.Time) ([]string, error) {
	// 获取用户的所有订阅
	subscriptions, err := core.Store().RSSSubscriptionStore().List(ctx, userID, spaceID)
	if err != nil {
		return nil, err
	}

	if len(subscriptions) == 0 {
		return []string{}, nil
	}

	// 获取当天的时间范围
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var articleIDs []string

	for _, subscription := range subscriptions {
		// 获取该订阅在当天发布的文章
		articles, err := core.Store().RSSArticleStore().ListByDateRange(
			ctx,
			subscription.ID,
			startOfDay.Unix(),
			endOfDay.Unix(),
			100,
		)
		if err != nil {
			slog.Error("Failed to list articles for subscription",
				slog.String("subscription_id", subscription.ID),
				slog.String("error", err.Error()))
			continue
		}

		// 收集有摘要的文章ID
		for _, article := range articles {
			if article.Summary != "" {
				articleIDs = append(articleIDs, article.ID)
			}
		}
	}

	return articleIDs, nil
}

// generateIntegratedDigest 调用AI生成整合后的每日摘要
func generateIntegratedDigest(ctx context.Context, core *core.Core, articleIDs []string, date time.Time) (string, string, error) {
	if len(articleIDs) == 0 {
		return generateEmptyDigestContent(core, date), "none", nil
	}

	// 构建提示词
	prompt := buildDailyDigestPrompt(len(articleIDs), date, core)

	// 准备文章内容（简化版，只包含ID列表和摘要）
	articlesContent := buildArticlesContentForDigest(ctx, core, articleIDs)

	// 调用 AI
	aiDriver := core.Srv().AI().GetChatAI(false)
	if aiDriver == nil {
		return "", "", fmt.Errorf("AI driver not available")
	}

	messages := []*types.MessageContext{
		{
			Role:    types.USER_ROLE_SYSTEM,
			Content: prompt,
		},
		{
			Role:    types.USER_ROLE_USER,
			Content: articlesContent,
		},
	}

	// TODO: 记录 Usage
	// 使用 Eino 接口生成
	einoMessages := ai.ConvertMessageContextToEinoMessages(messages)
	response, err := aiDriver.Generate(ctx, einoMessages)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate daily digest: %w", err)
	}

	return response.Content, aiDriver.Config().ModelName, nil
}

// buildDailyDigestPrompt 构建每日摘要的AI提示词
func buildDailyDigestPrompt(articleCount int, date time.Time, core *core.Core) string {

	return fmt.Sprintf(`你是一个专业的内容整合助手，负责将用户订阅的 RSS 内容整合成一份易读的每日报告。

## 任务目标

为用户生成 %s 的 RSS 每日摘要报告。用户今天共收到 %d 篇文章更新，需要你帮助整合、分类和提炼核心信息。

## 报告要求

1. **内容组织**：
   - 按主题将相关文章归类（例如：技术动态、行业新闻、产品更新等）
   - 每个主题下包含 1-N 篇相关文章
   - 如果文章主题差异较大，可以分成多个类别

2. **主题分类标准**：
   - 根据文章的关键词和内容自动识别主题
   - 优先按技术领域、行业类别、内容类型分类
   - 相似主题的文章归为一组
   - 独立主题的文章单独成组

3. **每个主题包含**：
   - 主题名称（简洁、准确，使用 🏷️ emoji）
   - 主题概述（50-100字，综合该主题下所有文章的核心观点）
   - 相关文章列表（每篇文章需包含）：
     * 文章标题作为链接，格式为：[标题](#article-文章ID)
     * 一句话总结（20-30字）
     * 关键词标签（使用反引号包裹）

4. **输出格式**：
   - 使用 Markdown 格式
   - 结构清晰，层次分明
   - 重点突出，易于快速浏览
   - 专业、客观、信息密度高
   - 直接输出内容，不要使用代码块标记（不要用三个反引号）

请根据下面提供的文章信息，生成今日的 RSS 摘要报告。`,
		date.Format("2006年01月02日"),
		articleCount)

	return fmt.Sprintf(`You are a professional content curator responsible for creating a comprehensive daily digest from users' RSS subscriptions.

## Task

Generate a daily RSS digest report for %s. The user received %d article updates today. Help organize, categorize, and extract core information.

## Report Requirements

1. **Content Organization**:
   - Group related articles by topics (e.g., Tech Updates, Industry News, Product Releases)
   - Each topic contains 1-N related articles
   - Create multiple categories if articles cover diverse topics

2. **Output Format**:
   - Use Markdown format
   - Clear structure with distinct hierarchy
   - Highlight key points for easy scanning
   - Professional, objective, information-dense
   - Output content directly, do NOT use code block markers (no triple backticks)

Now, generate today's RSS digest report based on the article information provided below.`,
		date.Format("January 02, 2006"),
		articleCount)
}

// buildArticlesContentForDigest 构建文章内容供AI处理
func buildArticlesContentForDigest(ctx context.Context, core *core.Core, articleIDs []string) string {
	var builder strings.Builder

	builder.WriteString("## 文章列表\n\n")

	for _, articleID := range articleIDs {
		// 获取文章详情
		article, err := core.Store().RSSArticleStore().Get(ctx, articleID)
		if err != nil {
			slog.Warn("Failed to get article", slog.String("article_id", articleID), slog.String("error", err.Error()))
			continue
		}

		builder.WriteString(fmt.Sprintf("### 文章 %s\n\n", articleID))
		builder.WriteString(fmt.Sprintf("- **标题**: %s\n", article.Title))

		if len(article.Keywords) > 0 {
			builder.WriteString(fmt.Sprintf("- **关键词**: %s\n", strings.Join(article.Keywords, ", ")))
		}

		builder.WriteString(fmt.Sprintf("- **摘要**: %s\n\n", article.Summary))
		builder.WriteString("---  ")
	}

	return builder.String()
}

// generateEmptyDigestContent 生成空摘要内容
func generateEmptyDigestContent(core *core.Core, date time.Time) string {
	lang := core.Srv().AI().Lang()

	if lang == ai.MODEL_BASE_LANGUAGE_CN {
		return fmt.Sprintf(`# 📅 %s RSS 每日摘要

> 今日暂无新文章更新

您的 RSS 订阅源今天没有新内容发布。建议：

- 检查订阅源是否正常工作
- 考虑添加更多感兴趣的订阅源
- 查看历史摘要回顾往期内容

---
*下次更新时间：明天*`,
			date.Format("2006年01月02日"))
	}

	return fmt.Sprintf(`# 📅 %s Daily RSS Digest

> No new articles today

Your RSS feeds have no new content published today. Suggestions:

- Check if your feeds are working properly
- Consider adding more feeds you're interested in
- Review historical digests for past content

---
*Next update: Tomorrow*`,
		date.Format("January 02, 2006"))
}
