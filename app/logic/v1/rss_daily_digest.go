package v1

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/types"
)

// RSSDailyDigestLogic 每日摘要生成逻辑
type RSSDailyDigestLogic struct {
	ctx  context.Context
	core *core.Core
}

// NewRSSDailyDigestLogic 创建每日摘要生成逻辑实例
func NewRSSDailyDigestLogic(ctx context.Context, core *core.Core) *RSSDailyDigestLogic {
	return &RSSDailyDigestLogic{
		ctx:  ctx,
		core: core,
	}
}

// DailyDigestResult 每日摘要生成结果
type DailyDigestResult struct {
	Content      string   // 整合后的摘要内容（Markdown格式）
	ArticleIDs   []string // 包含的文章ID列表
	ArticleCount int      // 文章总数
	Model        string   // 使用的AI模型
}

// GenerateDailyDigest 为用户生成每日RSS摘要
func (l *RSSDailyDigestLogic) GenerateDailyDigest(userID, spaceID string, date time.Time) (*DailyDigestResult, error) {
	// 1. 获取用户在该日期的所有RSS文章
	articles, err := l.getUserDailyArticles(userID, spaceID, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get user daily articles: %w", err)
	}

	if len(articles) == 0 {
		slog.Info("No articles found for daily digest",
			slog.String("user_id", userID),
			slog.String("date", date.Format("2006-01-02")))
		return &DailyDigestResult{
			Content:      l.generateEmptyDigestContent(date),
			ArticleIDs:   []string{},
			ArticleCount: 0,
		}, nil
	}

	slog.Info("Generating daily digest",
		slog.String("user_id", userID),
		slog.String("date", date.Format("2006-01-02")),
		slog.Int("article_count", len(articles)))

	// 2. 调用AI生成整合摘要
	digestContent, model, err := l.generateIntegratedDigest(articles, date)
	if err != nil {
		return nil, fmt.Errorf("failed to generate integrated digest: %w", err)
	}

	// 3. 收集文章ID列表
	articleIDs := lo.Map(articles, func(article *types.RSSDigestArticle, _ int) string {
		return article.ID
	})

	return &DailyDigestResult{
		Content:      digestContent,
		ArticleIDs:   articleIDs,
		ArticleCount: len(articles),
		Model:        model,
	}, nil
}

// getUserDailyArticles 获取用户在指定日期的所有RSS文章（已生成摘要的）
func (l *RSSDailyDigestLogic) getUserDailyArticles(userID, spaceID string, date time.Time) ([]*types.RSSDigestArticle, error) {
	// 获取用户的所有订阅
	subscriptions, err := l.core.Store().RSSSubscriptionStore().List(l.ctx, userID, spaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user subscriptions: %w", err)
	}

	if len(subscriptions) == 0 {
		return []*types.RSSDigestArticle{}, nil
	}

	// 获取当天的时间范围 (00:00:00 - 23:59:59)
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	// 收集所有订阅的文章
	var allArticles []*types.RSSDigestArticle
	var articleIDs []string

	for _, subscription := range subscriptions {
		// 获取该订阅在当天发布的文章
		articles, err := l.core.Store().RSSArticleStore().ListByDateRange(
			l.ctx,
			subscription.ID,
			startOfDay.Unix(),
			endOfDay.Unix(),
			100, // 限制每个订阅最多100篇
		)
		if err != nil {
			slog.Error("Failed to list articles for subscription",
				slog.String("subscription_id", subscription.ID),
				slog.String("error", err.Error()))
			continue
		}

		// 过滤掉没有摘要的文章
		articlesWithSummary := lo.Filter(articles, func(article *types.RSSArticle, _ int) bool {
			return article.Summary != ""
		})

		// 转换为 RSSDigestArticle 格式，并收集文章ID
		for _, article := range articlesWithSummary {
			articleIDs = append(articleIDs, article.ID)

			allArticles = append(allArticles, &types.RSSDigestArticle{
				ID:          article.ID,
				Title:       article.Title,
				Summary:     article.Summary,
				Keywords:    article.Keywords,
				Link:        article.Link,
				PublishedAt: article.PublishedAt,
				Source:      subscription.Title,
				KnowledgeID: "", // 稍后批量填充
			})
		}
	}

	// 批量获取 Knowledge ID（一次性查询，避免N+1问题）
	knowledgeMap, err := l.batchFindKnowledgeIDs(userID, articleIDs)
	if err != nil {
		slog.Warn("Failed to batch find knowledge IDs",
			slog.String("user_id", userID),
			slog.String("error", err.Error()))
		// 继续处理，但保持 KnowledgeID 为空
	} else {
		// 填充 KnowledgeID
		for _, article := range allArticles {
			if knowledge, ok := knowledgeMap[article.ID]; ok {
				article.KnowledgeID = knowledge.ID
			}
		}
	}

	slog.Info("Collected articles for daily digest",
		slog.String("user_id", userID),
		slog.Int("total_articles", len(allArticles)))

	return allArticles, nil
}

// findKnowledgeID 查找文章对应的用户Knowledge ID
func (l *RSSDailyDigestLogic) findKnowledgeID(userID string, articleID string) (string, error) {
	// 通过 rel_doc_id 查找 Knowledge
	knowledge, err := l.core.Store().KnowledgeStore().GetByRelDocID(l.ctx, userID, articleID)
	if err != nil {
		return "", err
	}
	return knowledge.ID, nil
}

// batchFindKnowledgeIDs 批量查找文章对应的用户Knowledge ID映射
func (l *RSSDailyDigestLogic) batchFindKnowledgeIDs(userID string, articleIDs []string) (map[string]*types.Knowledge, error) {
	if len(articleIDs) == 0 {
		return make(map[string]*types.Knowledge), nil
	}

	// 使用新的批量获取方法
	knowledgeMap, err := l.core.Store().KnowledgeStore().BatchGetByRelDocIDs(l.ctx, userID, articleIDs)
	if err != nil {
		return nil, err
	}

	return knowledgeMap, nil
}

// generateIntegratedDigest 调用AI生成整合后的每日摘要
func (l *RSSDailyDigestLogic) generateIntegratedDigest(articles []*types.RSSDigestArticle, date time.Time) (string, string, error) {
	// 构建提示词
	prompt := l.buildDailyDigestPrompt(articles, date)

	// 准备文章列表
	articlesContent := l.buildArticlesContent(articles)

	// 调用 AI
	aiDriver := l.core.Srv().AI().GetChatAI(false)
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

	// 使用 Eino 接口生成
	einoMessages := ai.ConvertMessageContextToEinoMessages(messages)
	response, err := aiDriver.Generate(l.ctx, einoMessages)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate daily digest: %w", err)
	}

	return response.Content, aiDriver.Config().ModelName, nil
}

// buildDailyDigestPrompt 构建每日摘要的AI提示词
func (l *RSSDailyDigestLogic) buildDailyDigestPrompt(articles []*types.RSSDigestArticle, date time.Time) string {
	return l.buildChineseDigestPrompt(articles, date)
}

// buildChineseDigestPrompt 中文每日摘要提示词
func (l *RSSDailyDigestLogic) buildChineseDigestPrompt(articles []*types.RSSDigestArticle, date time.Time) string {
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
     * 文章标题作为链接，格式为：[标题](#article-KnowledgeID)
     * 一句话总结（20-30字）
     * 关键词标签（使用反引号包裹）

4. **整体风格**：
   - 使用 Markdown 格式
   - 结构清晰，层次分明
   - 重点突出，易于快速浏览
   - 专业、客观、信息密度高

5. **输出格式示例**（请严格按照此格式输出，不要包含代码块标记）：

第一行：# 📅 日期 RSS 每日摘要

第二行：> 今日共收到 N 篇更新，涵盖 M 个主题

空一行

主题标题：## 🏷️ 主题名称

主题概述内容...

空一行

### 相关文章

空一行

文章列表：- **[文章标题](#article-KnowledgeID)** - 一句话总结
  标签：反引号关键词1反引号 反引号关键词2反引号

空一行

分隔符：---

空一行

（继续下一个主题...）

## 重要提醒

- 直接输出 Markdown 内容，不要使用代码块标记（不要用三个反引号）
- 不要遗漏任何文章
- 确保每篇文章都被归类到某个主题下
- 主题分类要合理，避免过于细碎或过于笼统
- 文章链接格式必须是 #article-ID（ID 是纯数字）
- 保持客观中立，不添加个人评价
- 关键词标签使用反引号包裹

现在，请根据下面提供的文章信息，生成今日的 RSS 摘要报告。`,
		date.Format("2006年01月02日"),
		len(articles))
}

// buildEnglishDigestPrompt 英文每日摘要提示词
func (l *RSSDailyDigestLogic) buildEnglishDigestPrompt(articles []*types.RSSDigestArticle, date time.Time) string {
	return fmt.Sprintf(`You are a professional content curator responsible for creating a comprehensive daily digest from users' RSS subscriptions.

## Task

Generate a daily RSS digest report for %s. The user received %d article updates today. Help organize, categorize, and extract core information.

## Report Requirements

1. **Content Organization**:
   - Group related articles by topics (e.g., Tech Updates, Industry News, Product Releases)
   - Each topic contains 1-N related articles
   - Create multiple categories if articles cover diverse topics

2. **Topic Classification**:
   - Automatically identify topics based on keywords and content
   - Prioritize grouping by technology domain, industry category, or content type
   - Group articles with similar themes together
   - Separate articles with unique topics

3. **Each Topic Includes**:
   - Topic name (concise and accurate, with 🏷️ emoji)
   - Topic overview (50-100 words, synthesizing key points from all articles in this topic)
   - Related articles list (for each article):
     * Article title as link: [Title](#article-KnowledgeID)
     * One-sentence summary (15-25 words)
     * Keyword tags (wrapped in backticks)

4. **Overall Style**:
   - Use Markdown format
   - Clear structure with distinct hierarchy
   - Highlight key points for easy scanning
   - Professional, objective, information-dense

5. **Output Format Example** (strictly follow this format, do NOT include code block markers):

First line: # 📅 Date Daily RSS Digest

Second line: > Today's update: N articles covering M topics

Blank line

Topic heading: ## 🏷️ Topic Name

Topic overview content...

Blank line

### Related Articles

Blank line

Article entry: - **[Article Title](#article-ID)** - One-sentence summary
  Tags: backtick-keyword1-backtick backtick-keyword2-backtick

Blank line

Separator: ---

Blank line

(Continue with next topic...)

## Important Reminders

- Output Markdown content directly, do NOT use code block markers (no triple backticks)
- Don't miss any articles
- Ensure every article is categorized under a topic
- Topic classification should be reasonable, avoiding over-fragmentation or over-generalization
- Article link format must be #article-KnowledgeID (ID is numeric only)
- Maintain objectivity and neutrality, no personal opinions
- Wrap keyword tags in backticks

Now, generate today's RSS digest report based on the article information provided below.`,
		date.Format("January 02, 2006"),
		len(articles))
}

// buildArticlesContent 构建文章列表内容（供AI处理）
func (l *RSSDailyDigestLogic) buildArticlesContent(articles []*types.RSSDigestArticle) string {
	var builder strings.Builder

	builder.WriteString("## 文章列表\n\n")

	for i, article := range articles {
		builder.WriteString(fmt.Sprintf("### 文章 %d\n\n", i+1))
		if article.KnowledgeID != "" {
			builder.WriteString(fmt.Sprintf("- **Knowledge ID**: %s\n", article.KnowledgeID))
		}
		builder.WriteString(fmt.Sprintf("- **标题**: %s\n", article.Title))
		builder.WriteString(fmt.Sprintf("- **来源**: %s\n", article.Source))

		if len(article.Keywords) > 0 {
			builder.WriteString(fmt.Sprintf("- **关键词**: %s\n", strings.Join(article.Keywords, ", ")))
		}

		builder.WriteString(fmt.Sprintf("- **摘要**: %s\n", article.Summary))

		if article.Link != "" {
			builder.WriteString(fmt.Sprintf("- **链接**: %s\n", article.Link))
		}

		builder.WriteString("\n")
	}

	return builder.String()
}

// generateEmptyDigestContent 生成空摘要内容（当天没有文章时）
func (l *RSSDailyDigestLogic) generateEmptyDigestContent(date time.Time) string {
	lang := l.core.Srv().AI().Lang()

	switch lang {
	case ai.MODEL_BASE_LANGUAGE_CN:
		return fmt.Sprintf(`# 📅 %s RSS 每日摘要

> 今日暂无新文章更新

您的 RSS 订阅源今天没有新内容发布。建议：

- 检查订阅源是否正常工作
- 考虑添加更多感兴趣的订阅源
- 查看历史摘要回顾往期内容

---
*下次更新时间：明天*`,
			date.Format("2006年01月02日"))

	default:
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
}

// BatchGenerateDailyDigests 批量为多个用户生成每日摘要
func (l *RSSDailyDigestLogic) BatchGenerateDailyDigests(userIDs []string, date time.Time) map[string]*DailyDigestResult {
	results := make(map[string]*DailyDigestResult)

	for _, userID := range userIDs {
		// TODO: 获取用户的默认 SpaceID
		spaceID := "" // 需要从用户信息中获取

		result, err := l.GenerateDailyDigest(userID, spaceID, date)
		if err != nil {
			slog.Error("Failed to generate daily digest for user",
				slog.String("user_id", userID),
				slog.String("error", err.Error()))
			continue
		}

		results[userID] = result
	}

	return results
}
