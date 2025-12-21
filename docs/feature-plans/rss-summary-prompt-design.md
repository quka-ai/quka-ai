# RSS 专用摘要功能设计

## 设计理念

RSS 文章摘要与 Knowledge 摘要有不同的目标和用途，因此需要**独立的 Prompt 和处理逻辑**。

## RSS 摘要 vs Knowledge 摘要

| 特性 | Knowledge 摘要 | RSS 摘要 |
|-----|--------------|---------|
| **目标** | 提取核心信息用于检索和向量化 | 快速预览，吸引读者点击 |
| **长度** | 可能较长，包含详细信息 | 简洁（100-150字） |
| **风格** | 结构化（标题+标签+分块） | 新闻/文章预览风格 |
| **Prompt** | 通用知识提取 | RSS 专用（突出新鲜度、重点） |
| **用途** | 向量化、搜索、分块 | 列表展示、每日推荐 |
| **存储** | Knowledge.summary（用户隔离） | RSSArticle.summary（全局共享） |
| **生成时机** | 创建 Knowledge 时（同步/异步） | 抓取文章后（异步，独立） |

## 核心设计

### 1. 独立的摘要生成器

```go
// pkg/rss/summarizer.go
type Summarizer struct {
    core *core.Core
}

// 专门为 RSS 文章设计的摘要生成
func (s *Summarizer) GenerateSummary(ctx context.Context, article *types.RSSArticle) (*RSSArticleSummaryResult, error)
```

**优势**：
- ✅ 不依赖 Knowledge 处理流程
- ✅ 可以单独优化 RSS 摘要的 Prompt
- ✅ 支持批量处理（提高效率）
- ✅ 独立的重试和错误处理

### 2. RSS 专用 Prompt

#### 中文 Prompt

```
你是一个专业的内容摘要助手，专门为 RSS 订阅内容生成简洁、吸引人的摘要。

## 任务要求
请为以下文章生成摘要和关键词，帮助读者快速了解文章核心内容。

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
摘要：[你的摘要内容]
关键词：[关键词1],[关键词2],[关键词3]
```

#### 英文 Prompt

```
You are a professional content summarizer specialized in creating concise, engaging summaries for RSS feed articles.

## Summary Requirements
1. **Length**: 100-150 words
2. **Style**: Concise, clear, highlighting key points
3. **Content**:
   - First sentence summarizes the main topic
   - Mention 1-2 key points or findings
   - For technical articles, retain key technical terms
   - For news, emphasize timeliness and importance
4. **Avoid**:
   - Don't use introductory phrases like "This article"
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
Summary: [Your summary content]
Keywords: [keyword1],[keyword2],[keyword3]
```

### 3. 异步处理流程

```
RSS Feed 抓取
    ↓
创建 RSSArticle（不含摘要）
    ↓
异步任务：生成摘要
    ↓
检查是否已有摘要 (article.summary != "")
    ↓ 如果没有
调用 RSS 专用 Summarizer
    ↓
更新 RSSArticle.summary
```

**优势**：
- ✅ 不阻塞文章抓取流程
- ✅ 支持批量生成（提高效率）
- ✅ 自动去重（已有摘要的跳过）
- ✅ 失败可重试

### 4. 数据存储

```go
type RSSArticle struct {
    // 原始内容
    ID          int64
    Title       string
    Content     string

    // AI 生成的摘要（所有用户共享）
    Summary            string         // AI 摘要
    Keywords           pq.StringArray // 关键词
    SummaryGeneratedAt int64          // 生成时间
    AIModel            string         // 使用的模型
}
```

**为什么存在 Article 表？**
- ✅ 所有订阅该 RSS 的用户共享摘要（节省成本）
- ✅ 快速查询（无需关联 Knowledge）
- ✅ 缓存机制（避免重复生成）

## 代码实现

### 核心组件

1. **Summarizer** - [pkg/rss/summarizer.go](../../pkg/rss/summarizer.go)
   - `GenerateSummary()` - 单篇文章摘要
   - `BatchGenerateSummaries()` - 批量生成
   - RSS 专用 Prompt（中英文）

2. **Processor** - [pkg/rss/processor.go](../../pkg/rss/processor.go)
   - `GenerateArticleSummary()` - 为文章生成摘要
   - `BatchGenerateArticleSummaries()` - 批量处理

3. **RSSArticleStore** - [app/store/sqlstore/rss_article_store.go](../../app/store/sqlstore/rss_article_store.go)
   - `UpdateSummary()` - 更新摘要
   - `ListWithoutSummary()` - 获取没有摘要的文章

### 使用示例

#### 单篇文章生成摘要

```go
processor := rss.NewProcessor(core)

// 异步生成摘要
go func() {
    if err := processor.GenerateArticleSummary(ctx, articleID); err != nil {
        slog.Error("Failed to generate summary", slog.Any("error", err))
    }
}()
```

#### 批量生成摘要

```go
// 为订阅的前 50 篇文章生成摘要
err := processor.BatchGenerateArticleSummaries(ctx, subscriptionID, 50)
```

#### 定时任务（补充缺失的摘要）

```go
// 每小时运行一次，补充缺失的摘要
func (s *Scheduler) generateMissingSummaries() {
    subscriptions := s.getAllEnabledSubscriptions()

    for _, sub := range subscriptions {
        processor.BatchGenerateArticleSummaries(ctx, sub.ID, 100)
    }
}
```

## Prompt 优化方向

### 1. 根据内容类型定制

```go
func (s *Summarizer) buildPrompt(article *types.RSSArticle) string {
    // 根据内容特征选择 Prompt
    if isTechnicalArticle(article) {
        return PROMPT_RSS_TECH_SUMMARY_CN
    } else if isNewsArticle(article) {
        return PROMPT_RSS_NEWS_SUMMARY_CN
    } else {
        return PROMPT_RSS_GENERAL_SUMMARY_CN
    }
}
```

### 2. 用户偏好定制

```go
// 未来可以根据用户偏好调整摘要风格
type SummaryPreference struct {
    Length   string // "short", "medium", "long"
    Style    string // "formal", "casual"
    Focus    string // "technical", "business", "general"
}
```

### 3. 多语言支持

```go
func (s *Summarizer) detectLanguage(content string) string {
    // 自动检测文章语言
    // 使用对应语言的 Prompt
}
```

## 性能优化

### 1. 批量处理

```go
// 并发生成摘要（控制并发数）
results := summarizer.BatchGenerateSummaries(ctx, articles, maxConcurrency: 3)
```

**优势**：
- ✅ 减少 API 调用延迟
- ✅ 提高吞吐量
- ✅ 降低成本（批量折扣）

### 2. 智能去重

```go
// 只处理没有摘要的文章
articles := store.ListWithoutSummary(ctx, subscriptionID, limit)
```

**优势**：
- ✅ 避免重复生成
- ✅ 节省 Token 成本
- ✅ 提高效率

### 3. 失败重试

```go
// 摘要生成失败时自动重试
func (s *Summarizer) GenerateSummaryWithRetry(ctx context.Context, article *types.RSSArticle, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        result, err := s.GenerateSummary(ctx, article)
        if err == nil {
            return nil
        }
        time.Sleep(time.Second * time.Duration(i+1))
    }
    return err
}
```

## 成本分析

### 摘要生成成本

假设：
- 平均文章长度：2000 tokens（输入）
- 摘要长度：150 tokens（输出）
- GPT-4 价格：$0.03/1K tokens（输入），$0.06/1K tokens（输出）

**单篇文章成本**：
```
输入: 2000 tokens * $0.03/1000 = $0.06
输出: 150 tokens * $0.06/1000 = $0.009
总计: $0.069 ≈ $0.07
```

**100 位用户订阅同一 RSS**：
- ❌ 传统方式（每人生成）：$0.07 × 100 = **$7.00**
- ✅ 共享方式（生成一次）：$0.07 × 1 = **$0.07**

**节省 99% 成本！**

## 与 Knowledge 的关系

### RSS 处理完整流程

```
RSS Feed 抓取
    ↓
创建 RSSArticle
    ↓
异步生成摘要 → RSSArticle.summary (共享)
    ↓
为每个用户创建 Knowledge (用户副本)
    ↓
Knowledge 向量化 (使用 article 内容，不是摘要)
    ↓
用户可以：
  - 在列表中看到共享的摘要（快速浏览）
  - 点击跳转到个人 Knowledge（深度阅读）
  - 通过向量搜索相关内容
```

### 数据关系

```
RSSArticle (共享摘要)
    ↑ N:1
Knowledge (用户副本)
    - 通过 rel_doc_id 关联
    - 每个用户有独立的 Knowledge
    - 向量化使用完整内容（不是摘要）
```

## 总结

### ✅ 设计优势

1. **独立性** - 不依赖 Knowledge 处理流程，可独立优化
2. **专业化** - RSS 专用 Prompt，更适合新闻/文章预览
3. **高效性** - 批量处理、异步执行、智能去重
4. **经济性** - 摘要共享，节省 99% AI 成本
5. **灵活性** - 可以根据内容类型、用户偏好定制

### 🎯 实施要点

1. ✅ 使用独立的 `Summarizer`，不复用 Knowledge 的 `AI.Summarize()`
2. ✅ RSS 专用 Prompt（简洁、吸引人、突出重点）
3. ✅ 摘要存储在 `RSSArticle` 表（全局共享）
4. ✅ 异步处理，不阻塞文章抓取
5. ✅ 支持批量生成，提高效率

### 📋 待实现

- [ ] 数据库迁移（添加摘要字段）
- [ ] 集成到文章抓取流程
- [ ] 实现定时补充任务
- [ ] 添加摘要质量监控
- [ ] 根据用户反馈优化 Prompt
