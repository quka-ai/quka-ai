package process

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/types"
)

// RSSSummarizerLogic RSS 文章摘要生成逻辑
type RSSSummarizerLogic struct {
	ctx  context.Context
	core *core.Core
}

// NewRSSSummarizerLogic 创建新的摘要生成逻辑实例
func NewRSSSummarizerLogic(ctx context.Context, core *core.Core) *RSSSummarizerLogic {
	return &RSSSummarizerLogic{
		ctx:  ctx,
		core: core,
	}
}

// RSSArticleSummaryResult RSS 文章摘要结果
type RSSArticleSummaryResult struct {
	Summary  string        // 摘要文本
	Keywords []string      // 关键词
	Model    string        // 使用的模型
	Usage    *openai.Usage // Token 使用量
}

// GenerateSummary 为 RSS 文章生成摘要
func (l *RSSSummarizerLogic) GenerateSummary(article *types.RSSArticle) (*RSSArticleSummaryResult, error) {
	// 准备文章内容
	content := l.prepareContent(article)
	if content == "" {
		return nil, fmt.Errorf("article content is empty")
	}

	// 构建 RSS 专用 Prompt
	prompt := l.buildRSSPrompt(article)

	// 调用 AI 生成摘要
	aiDriver := l.core.Srv().AI().GetChatAI(false)
	if aiDriver == nil {
		return nil, fmt.Errorf("AI driver not available")
	}

	messages := []*types.MessageContext{
		{
			Role:    types.USER_ROLE_SYSTEM,
			Content: prompt,
		},
		{
			Role:    types.USER_ROLE_USER,
			Content: content,
		},
	}

	// 使用 Eino 接口生成
	einoMessages := ai.ConvertMessageContextToEinoMessages(messages)
	response, err := aiDriver.Generate(l.ctx, einoMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	// 解析 AI 返回结果
	summary, keywords := l.parseSummaryResponse(response.Content)

	return &RSSArticleSummaryResult{
		Summary:  summary,
		Keywords: keywords,
		Model:    aiDriver.Config().ModelName,
		Usage: &openai.Usage{
			PromptTokens:     response.ResponseMeta.Usage.PromptTokens,
			CompletionTokens: response.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:      response.ResponseMeta.Usage.TotalTokens,
		},
	}, nil
}

// prepareContent 准备文章内容（优先使用 content，其次 description）
func (l *RSSSummarizerLogic) prepareContent(article *types.RSSArticle) string {
	content := strings.TrimSpace(article.Content)
	if content == "" {
		content = strings.TrimSpace(article.Description)
	}

	// 限制长度，避免超过模型上下文
	maxLength := 4000 // 约 1000 tokens
	if len([]rune(content)) > maxLength {
		content = string([]rune(content)[:maxLength]) + "..."
	}

	return content
}

// buildRSSPrompt 构建 RSS 专用的摘要 Prompt
func (l *RSSSummarizerLogic) buildRSSPrompt(article *types.RSSArticle) string {
	// 根据 AI 驱动的语言选择 Prompt
	lang := l.core.Srv().AI().Lang()

	switch lang {
	case ai.MODEL_BASE_LANGUAGE_CN:
		return l.buildChinesePrompt(article)
	default:
		return l.buildEnglishPrompt(article)
	}
}

// buildChinesePrompt 中文 Prompt
func (l *RSSSummarizerLogic) buildChinesePrompt(article *types.RSSArticle) string {
	return fmt.Sprintf(`你是一个专业的内容摘要助手，专门为 RSS 订阅内容生成简洁、吸引人的摘要。

## 任务要求

请为以下文章生成摘要和关键词，帮助读者快速了解文章核心内容。

## 文章信息

- 标题：%s
- 作者：%s
- 来源：%s

## 摘要要求

1. **长度**：控制在 100-150 字
2. **风格**：简洁、清晰，突出重点
3. **内容**：
   - 第一句话概括文章主题
   - 提及 1-2 个关键观点或发现
   - 如果是技术文章，保留关键技术名词
   - 如果是新闻，突出时效性和重要性
4. **避免**：
   - 不要使用"这篇文章"、"本文"等引导语
   - 不要包含个人评价
   - 不要重复标题

## 关键词要求

1. 提取 3-5 个关键词
2. 关键词应该是：
   - 文章的核心概念
   - 技术名词（如果是技术文章）
   - 行业术语
   - 重要人物或公司名称

## 输出格式

请严格按照以下格式输出（不要包含其他内容）：

摘要：[你的摘要内容]
关键词：[关键词1],[关键词2],[关键词3]

---

现在，请为下面的文章内容生成摘要和关键词：`,
		article.Title,
		lo.If(article.Author != "", article.Author).Else("未知"),
		lo.If(article.Link != "", article.Link).Else("未知"))
}

// buildEnglishPrompt 英文 Prompt
func (l *RSSSummarizerLogic) buildEnglishPrompt(article *types.RSSArticle) string {
	return fmt.Sprintf(`You are a professional content summarizer specialized in creating concise, engaging summaries for RSS feed articles.

## Task

Generate a summary and keywords for the following article to help readers quickly understand its core content.

## Article Information

- Title: %s
- Author: %s
- Source: %s

## Summary Requirements

1. **Length**: 100-150 words
2. **Style**: Concise, clear, highlighting key points
3. **Content**:
   - First sentence summarizes the main topic
   - Mention 1-2 key points or findings
   - For technical articles, retain key technical terms
   - For news, emphasize timeliness and importance
4. **Avoid**:
   - Don't use introductory phrases like "This article" or "The article"
   - Don't include personal opinions
   - Don't repeat the title

## Keywords Requirements

1. Extract 3-5 keywords
2. Keywords should be:
   - Core concepts from the article
   - Technical terms (for technical articles)
   - Industry terminology
   - Important people or company names

## Output Format

Please output strictly in the following format (no additional content):

Summary: [Your summary content]
Keywords: [keyword1],[keyword2],[keyword3]

---

Now, please generate a summary and keywords for the following article content:`,
		article.Title,
		lo.If(article.Author != "", article.Author).Else("Unknown"),
		lo.If(article.Link != "", article.Link).Else("Unknown"))
}

// parseSummaryResponse 解析 AI 返回的摘要结果
func (l *RSSSummarizerLogic) parseSummaryResponse(response string) (string, []string) {
	lines := strings.Split(response, "\n")

	var summary string
	var keywords []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 解析摘要
		if strings.HasPrefix(line, "摘要：") || strings.HasPrefix(line, "Summary:") {
			summary = strings.TrimSpace(strings.TrimPrefix(
				strings.TrimPrefix(line, "摘要："),
				"Summary:"))
		}

		// 解析关键词
		if strings.HasPrefix(line, "关键词：") || strings.HasPrefix(line, "Keywords:") {
			keywordsStr := strings.TrimSpace(strings.TrimPrefix(
				strings.TrimPrefix(line, "关键词："),
				"Keywords:"))
			keywords = parseKeywords(keywordsStr)
		}
	}

	// 如果解析失败，使用整个响应作为摘要
	if summary == "" {
		summary = strings.TrimSpace(response)
		// 限制长度
		if len([]rune(summary)) > 200 {
			summary = string([]rune(summary)[:200]) + "..."
		}
	}

	return summary, keywords
}

// parseKeywords 解析关键词字符串
func parseKeywords(keywordsStr string) []string {
	// 支持逗号、顿号、分号等分隔符
	keywordsStr = strings.ReplaceAll(keywordsStr, "、", ",")
	keywordsStr = strings.ReplaceAll(keywordsStr, "；", ",")
	keywordsStr = strings.ReplaceAll(keywordsStr, ";", ",")

	parts := strings.Split(keywordsStr, ",")
	keywords := make([]string, 0, len(parts))

	for _, part := range parts {
		keyword := strings.TrimSpace(part)
		if keyword != "" {
			keywords = append(keywords, keyword)
		}
	}

	return keywords
}

// BatchGenerateSummaries 批量生成摘要（提高效率）
func (l *RSSSummarizerLogic) BatchGenerateSummaries(articles []*types.RSSArticle, maxConcurrency int) map[string]*RSSArticleSummaryResult {
	if maxConcurrency <= 0 {
		maxConcurrency = 3 // 默认并发数
	}

	results := make(map[string]*RSSArticleSummaryResult)
	resultChan := make(chan struct {
		articleID string
		result    *RSSArticleSummaryResult
		err       error
	}, len(articles))

	// 信号量控制并发
	semaphore := make(chan struct{}, maxConcurrency)

	// 并发生成摘要
	for _, article := range articles {
		go func(art *types.RSSArticle) {
			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			result, err := l.GenerateSummary(art)
			resultChan <- struct {
				articleID string
				result    *RSSArticleSummaryResult
				err       error
			}{
				articleID: art.ID,
				result:    result,
				err:       err,
			}
		}(article)
	}

	// 收集结果
	for range articles {
		res := <-resultChan
		if res.err != nil {
			slog.Error("Failed to generate summary for article",
				slog.String("article_id", res.articleID),
				slog.String("error", res.err.Error()))
			continue
		}
		results[res.articleID] = res.result
	}

	return results
}

// GenerateArticleSummaryWithTokenTracking 为文章生成 AI 摘要并记录 Token 使用量到 quka_ai_token_usage 表
func (l *RSSSummarizerLogic) GenerateArticleSummaryWithTokenTracking(articleID string, spaceID string) error {
	const maxRetryTimes = 3 // 最大重试次数

	// 获取文章
	article, err := l.core.Store().RSSArticleStore().Get(l.ctx, articleID)
	if err != nil {
		return fmt.Errorf("failed to get article: %w", err)
	}

	// 检查是否已有摘要
	if article.Summary != "" {
		slog.Debug("Article summary already exists, skipping",
			slog.String("article_id", articleID))
		return nil
	}

	// 检查重试次数
	if article.SummaryRetryTimes >= maxRetryTimes {
		slog.Warn("Article summary retry limit exceeded, skipping",
			slog.String("article_id", articleID),
			slog.Int("retry_times", article.SummaryRetryTimes),
			slog.String("last_error", article.LastSummaryError))
		return fmt.Errorf("retry limit exceeded (%d times)", maxRetryTimes)
	}

	// 使用 RSS 专用摘要器生成摘要
	result, err := l.GenerateSummary(article)
	if err != nil {
		// 生成失败，增加重试次数并记录错误
		errorMsg := err.Error()
		if len(errorMsg) > 500 {
			errorMsg = errorMsg[:500] // 限制错误信息长度
		}

		if updateErr := l.core.Store().RSSArticleStore().IncrementSummaryRetry(l.ctx, articleID, errorMsg); updateErr != nil {
			slog.Error("Failed to increment summary retry count",
				slog.String("article_id", articleID),
				slog.String("error", updateErr.Error()))
		}

		slog.Error("Failed to generate article summary",
			slog.String("article_id", articleID),
			slog.Int("retry_times", article.SummaryRetryTimes+1),
			slog.String("error", err.Error()))

		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// 更新文章摘要（成功时会清除错误信息）
	if err := l.core.Store().RSSArticleStore().UpdateSummary(l.ctx, articleID, &types.RSSArticleSummary{
		Summary:            result.Summary,
		Keywords:           result.Keywords,
		SummaryGeneratedAt: time.Now().Unix(),
		AIModel:            result.Model,
	}); err != nil {
		return fmt.Errorf("failed to update article summary: %w", err)
	}

	// 记录 Token 使用量到 quka_ai_token_usage 表
	if err := l.core.Store().AITokenUsageStore().Create(l.ctx, types.AITokenUsage{
		SpaceID:     spaceID,
		UserID:      article.UserID, // 使用最初订阅该文章的用户ID
		Type:        types.USAGE_TYPE_KNOWLEDGE,
		SubType:     "rss_summary", // RSS 摘要生成
		ObjectID:    articleID,
		Model:       result.Model,
		UsagePrompt: result.Usage.PromptTokens,
		UsageOutput: result.Usage.CompletionTokens,
		CreatedAt:   time.Now().Unix(),
	}); err != nil {
		slog.Error("Failed to record RSS summary token usage",
			slog.String("article_id", articleID),
			slog.String("user_id", article.UserID),
			slog.String("error", err.Error()))
		// 不返回错误，因为摘要已经成功生成
	}

	slog.Info("RSS article summary generated successfully",
		slog.String("article_id", articleID),
		slog.String("model", result.Model),
		slog.String("user_id", article.UserID),
		slog.Int("summary_length", len(result.Summary)),
		slog.Int("keywords_count", len(result.Keywords)),
		slog.Int("retry_times", article.SummaryRetryTimes),
		slog.Int("prompt_tokens", result.Usage.PromptTokens),
		slog.Int("completion_tokens", result.Usage.CompletionTokens),
		slog.Int("total_tokens", result.Usage.TotalTokens))

	return nil
}

// BatchGenerateArticleSummaries 批量生成文章摘要（提高效率）
func (l *RSSSummarizerLogic) BatchGenerateArticleSummaries(subscriptionID string, limit int) error {
	// 获取没有摘要的文章
	articles, err := l.core.Store().RSSArticleStore().ListWithoutSummary(l.ctx, subscriptionID, limit)
	if err != nil {
		return fmt.Errorf("failed to list articles without summary: %w", err)
	}

	if len(articles) == 0 {
		slog.Debug("No articles without summary found",
			slog.String("subscription_id", subscriptionID))
		return nil
	}

	slog.Info("Starting batch summary generation",
		slog.String("subscription_id", subscriptionID),
		slog.Int("article_count", len(articles)))

	// 批量生成摘要（并发控制）
	results := l.BatchGenerateSummaries(articles, 3)

	// 更新数据库并记录 Token 使用量到 quka_ai_token_usage
	successCount := 0
	totalPromptTokens := 0
	totalCompletionTokens := 0
	totalTokens := 0

	// 建立 articleID 到 article 的映射，用于获取 user_id 和 space_id
	articleMap := make(map[string]*types.RSSArticle)
	for _, article := range articles {
		articleMap[article.ID] = article
	}

	for articleID, result := range results {
		// 更新文章摘要
		if err := l.core.Store().RSSArticleStore().UpdateSummary(l.ctx, articleID, &types.RSSArticleSummary{
			Summary:            result.Summary,
			Keywords:           result.Keywords,
			SummaryGeneratedAt: time.Now().Unix(),
			AIModel:            result.Model,
		}); err != nil {
			slog.Error("Failed to update article summary",
				slog.String("article_id", articleID),
				slog.String("error", err.Error()))
			continue
		}

		// 记录 Token 使用量到 quka_ai_token_usage 表
		article := articleMap[articleID]
		if article != nil {
			// 获取文章的 space_id（通过 subscription）
			subscription, err := l.core.Store().RSSSubscriptionStore().Get(l.ctx, article.SubscriptionID)
			if err == nil {
				if err := l.core.Store().AITokenUsageStore().Create(l.ctx, types.AITokenUsage{
					SpaceID:     subscription.SpaceID,
					UserID:      article.UserID,
					Type:        types.USAGE_TYPE_KNOWLEDGE,
					SubType:     "rss_summary",
					ObjectID:    articleID,
					Model:       result.Model,
					UsagePrompt: result.Usage.PromptTokens,
					UsageOutput: result.Usage.CompletionTokens,
					CreatedAt:   time.Now().Unix(),
				}); err != nil {
					slog.Error("Failed to record RSS summary token usage",
						slog.String("article_id", articleID),
						slog.String("user_id", article.UserID),
						slog.String("error", err.Error()))
				}
			}
		}

		successCount++
		totalPromptTokens += result.Usage.PromptTokens
		totalCompletionTokens += result.Usage.CompletionTokens
		totalTokens += result.Usage.TotalTokens
	}

	slog.Info("Batch summary generation completed",
		slog.String("subscription_id", subscriptionID),
		slog.Int("total", len(articles)),
		slog.Int("success", successCount),
		slog.Int("failed", len(articles)-successCount),
		slog.Int("total_prompt_tokens", totalPromptTokens),
		slog.Int("total_completion_tokens", totalCompletionTokens),
		slog.Int("total_tokens", totalTokens))

	return nil
}
