# RSS AI 智能摘要功能设计方案（修订版）

## 问题分析

### ❌ 错误设计
同一个 RSS 文章可能被多个用户订阅，如果在 `RSSArticle` 表中只存一个 `knowledge_id`，会导致：
1. 只能关联到第一个订阅用户的 Knowledge
2. 其他用户无法正确跳转
3. 存在隐私泄露风险

### ✅ 正确理解

```
RSS Article (全局共享，去重用)
    ↓ 一对多
User A → Knowledge A (用户 A 的个人副本)
User B → Knowledge B (用户 B 的个人副本)
User C → Knowledge C (用户 C 的个人副本)
```

每个用户都有自己的 Knowledge 副本，但 AI 摘要可以共享！

## 架构设计（修订）

### 数据关系

```
RSSArticle (原文 + 共享摘要)
    ↑ N:1
Knowledge (用户的个人副本)
    ↓
Vector (用户空间的向量)
```

### 核心设计原则

1. **RSSArticle**：存储原文和**共享的 AI 摘要**（所有用户共享）
2. **Knowledge**：每个用户都有自己的副本（通过 `user_id` + `space_id` 区分）
3. **关联方式**：`Knowledge.rel_doc_id` 存储 `article.id`（多对一）

## 数据库设计

### 1. RSS Article 表（修订）

```sql
-- 修改 RSS Article 表
ALTER TABLE quka_rss_articles
ADD COLUMN summary TEXT,                       -- AI 生成的摘要（所有用户共享）
ADD COLUMN keywords TEXT[],                    -- AI 提取的关键词（共享）
ADD COLUMN summary_generated_at BIGINT,        -- 摘要生成时间
ADD COLUMN ai_model VARCHAR(128);              -- 使用的 AI 模型

-- 索引
CREATE INDEX IF NOT EXISTS idx_rss_articles_subscription_id ON quka_rss_articles(subscription_id);
CREATE INDEX IF NOT EXISTS idx_rss_articles_published_at ON quka_rss_articles(published_at DESC);

-- 字段注释
COMMENT ON COLUMN quka_rss_articles.summary IS 'AI 生成的摘要（所有订阅用户共享）';
COMMENT ON COLUMN quka_rss_articles.keywords IS 'AI 提取的关键词（所有订阅用户共享）';
COMMENT ON COLUMN quka_rss_articles.summary_generated_at IS '摘要生成时间戳';
COMMENT ON COLUMN quka_rss_articles.ai_model IS '生成摘要使用的 AI 模型';
```

**关键点**：
- ✅ 摘要存在 Article 表，所有用户共享（节省 AI 调用成本）
- ✅ 不存 `knowledge_id`（因为一对多）
- ✅ 通过 `article.id` 可以反查所有关联的 Knowledge

### 2. Knowledge 表（已有，无需修改）

```go
type Knowledge struct {
    ID       string  `db:"id"`
    UserID   string  `db:"user_id"`   // 🔑 区分用户
    SpaceID  string  `db:"space_id"`  // 🔑 区分空间
    RelDocID string  `db:"rel_doc_id"` // 存储 article.id
    Summary  string  `db:"summary"`    // 用户个性化的摘要（可选）
    // ... 其他字段
}
```

## 代码实现

### 1. 数据类型定义

```go
// pkg/types/rss.go

type RSSArticle struct {
    ID                  int64          `json:"id" db:"id"`
    SubscriptionID      int64          `json:"subscription_id" db:"subscription_id"`
    GUID                string         `json:"guid" db:"guid"`
    Title               string         `json:"title" db:"title"`
    Link                string         `json:"link" db:"link"`
    Description         string         `json:"description" db:"description"`
    Content             string         `json:"content" db:"content"`
    Author              string         `json:"author" db:"author"`

    // AI 摘要（所有用户共享）
    Summary             string         `json:"summary" db:"summary"`
    Keywords            pq.StringArray `json:"keywords" db:"keywords"`
    SummaryGeneratedAt  int64          `json:"summary_generated_at" db:"summary_generated_at"`
    AIModel             string         `json:"ai_model" db:"ai_model"`

    PublishedAt         int64          `json:"published_at" db:"published_at"`
    FetchedAt           int64          `json:"fetched_at" db:"fetched_at"`
    CreatedAt           int64          `json:"created_at" db:"created_at"`
}

// RSS 文章列表项（用于展示）
type RSSArticleListItem struct {
    *RSSArticle
    KnowledgeID string `json:"knowledge_id"` // 当前用户的 Knowledge ID（动态查询）
    IsRead      bool   `json:"is_read"`      // 是否已读
}

// RSS 文章摘要更新
type RSSArticleSummary struct {
    Summary            string
    Keywords           []string
    SummaryGeneratedAt int64
    AIModel            string
}
```

### 2. Processor 处理流程（关键修改）

```go
// pkg/rss/processor.go

func (p *Processor) ProcessArticle(ctx context.Context, subscription *types.RSSSubscription, article *types.RSSArticle) error {
    // 1. 检查文章是否已存在（全局去重）
    existingArticle, err := p.core.Store().RSSArticleStore().GetByGUID(ctx, subscription.ID, article.GUID)
    if err != nil && err != sql.ErrNoRows {
        return fmt.Errorf("failed to check article existence: %w", err)
    }

    var articleID int64

    if existingArticle == nil {
        // 2. 文章不存在，创建新记录
        article.SubscriptionID = subscription.ID
        article.FetchedAt = time.Now().Unix()

        if err := p.core.Store().RSSArticleStore().Create(ctx, article); err != nil {
            return fmt.Errorf("failed to create article record: %w", err)
        }
        articleID = article.ID

        // 3. 异步生成 AI 摘要（后台任务）
        go safe.Run(func() {
            p.generateArticleSummary(context.Background(), articleID)
        })
    } else {
        // 文章已存在，使用已有的
        articleID = existingArticle.ID
        article = existingArticle
    }

    // 4. 为当前用户创建 Knowledge（每个用户都有自己的副本）
    knowledgeID, err := p.createUserKnowledge(ctx, subscription, article, articleID)
    if err != nil {
        return fmt.Errorf("failed to create user knowledge: %w", err)
    }

    slog.Info("RSS article processed successfully",
        slog.String("article_title", article.Title),
        slog.Int64("article_id", articleID),
        slog.String("knowledge_id", knowledgeID),
        slog.String("user_id", subscription.UserID))

    return nil
}

// generateArticleSummary 为文章生成 AI 摘要（异步，仅第一次）
func (p *Processor) generateArticleSummary(ctx context.Context, articleID int64) {
    article, err := p.core.Store().RSSArticleStore().Get(ctx, articleID)
    if err != nil {
        slog.Error("Failed to get article for summary generation",
            slog.Int64("article_id", articleID),
            slog.String("error", err.Error()))
        return
    }

    // 检查是否已生成摘要
    if article.Summary != "" {
        slog.Debug("Article summary already exists, skipping",
            slog.Int64("article_id", articleID))
        return
    }

    // 准备内容
    content := article.Content
    if content == "" {
        content = article.Description
    }

    // 调用 AI 生成摘要
    summary, err := p.core.Srv().AI().Summarize(ctx, &content)
    if err != nil {
        slog.Error("Failed to generate article summary",
            slog.Int64("article_id", articleID),
            slog.String("error", err.Error()))
        return
    }

    // 更新文章摘要
    if err := p.core.Store().RSSArticleStore().UpdateSummary(ctx, articleID, &types.RSSArticleSummary{
        Summary:            summary.Summary,
        Keywords:           summary.Tags,
        SummaryGeneratedAt: time.Now().Unix(),
        AIModel:            summary.Model,
    }); err != nil {
        slog.Error("Failed to update article summary",
            slog.Int64("article_id", articleID),
            slog.String("error", err.Error()))
        return
    }

    slog.Info("Article summary generated successfully",
        slog.Int64("article_id", articleID),
        slog.String("model", summary.Model))
}

// createUserKnowledge 为用户创建 Knowledge 副本
func (p *Processor) createUserKnowledge(ctx context.Context, subscription *types.RSSSubscription, article *types.RSSArticle, articleID int64) (string, error) {
    // 检查该用户是否已创建过这篇文章的 Knowledge
    existingKnowledge, err := p.core.Store().KnowledgeStore().GetByRelDocID(ctx, subscription.SpaceID, subscription.UserID, fmt.Sprintf("%d", articleID))
    if err != nil && err != sql.ErrNoRows {
        return "", fmt.Errorf("failed to check existing knowledge: %w", err)
    }

    if existingKnowledge != nil {
        // 用户已有这篇文章的 Knowledge
        slog.Debug("User knowledge already exists",
            slog.String("user_id", subscription.UserID),
            slog.Int64("article_id", articleID),
            slog.String("knowledge_id", existingKnowledge.ID))
        return existingKnowledge.ID, nil
    }

    // 获取 Resource 配置
    resource, err := p.core.Store().ResourceStore().GetResource(ctx, subscription.SpaceID, subscription.ResourceID)
    if err != nil {
        return "", fmt.Errorf("failed to get resource config: %w", err)
    }

    // 准备 Knowledge 内容
    content := p.buildKnowledgeContent(article)

    // 加密内容
    encryptedContent, err := p.core.EncryptData(types.KnowledgeContent(content))
    if err != nil {
        return "", fmt.Errorf("failed to encrypt content: %w", err)
    }

    // 计算过期时间
    var expiredAt int64
    if resource.Cycle > 0 {
        expiredAt = time.Now().Add(time.Duration(resource.Cycle) * time.Hour * 24).Unix()
    }

    // 创建用户的 Knowledge 记录
    knowledgeID := utils.GenUniqIDStr()
    knowledge := types.Knowledge{
        ID:          knowledgeID,
        SpaceID:     subscription.SpaceID,
        UserID:      subscription.UserID,  // 🔑 用户的个人副本
        Resource:    subscription.ResourceID,
        Kind:        "rss",
        ContentType: types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN,
        Content:     encryptedContent,
        RelDocID:    fmt.Sprintf("%d", articleID), // 🔑 关联到共享的 Article
        Stage:       types.KNOWLEDGE_STAGE_EMBEDDING, // 直接进入向量化阶段（摘要已在 Article 生成）
        Title:       article.Title,
        CreatedAt:   time.Now().Unix(),
        UpdatedAt:   time.Now().Unix(),
        ExpiredAt:   expiredAt,
        RetryTimes:  0,
    }

    if err := p.core.Store().KnowledgeStore().Create(ctx, knowledge); err != nil {
        return "", fmt.Errorf("failed to create knowledge: %w", err)
    }

    // 触发向量化处理
    process.NewEmbeddingRequest(knowledge)

    return knowledgeID, nil
}
```

### 3. Store 方法

```go
// app/store/sqlstore/rss_article_store.go

// GetByGUID 根据订阅和 GUID 获取文章
func (s *RSSArticleStore) GetByGUID(ctx context.Context, subscriptionID int64, guid string) (*types.RSSArticle, error) {
    query := sq.Select("*").
        From(s.GetTable()).
        Where(sq.Eq{
            "subscription_id": subscriptionID,
            "guid":            guid,
        })

    queryString, args, err := query.ToSql()
    if err != nil {
        return nil, ErrorSqlBuild(err)
    }

    var article types.RSSArticle
    if err := s.GetReplica(ctx).Get(&article, queryString, args...); err != nil {
        if err == sql.ErrNoRows {
            return nil, nil
        }
        return nil, err
    }

    return &article, nil
}

// UpdateSummary 更新文章摘要（所有用户共享）
func (s *RSSArticleStore) UpdateSummary(ctx context.Context, articleID int64, summary *types.RSSArticleSummary) error {
    query := sq.Update(s.GetTable()).
        Set("summary", summary.Summary).
        Set("keywords", pq.Array(summary.Keywords)).
        Set("summary_generated_at", summary.SummaryGeneratedAt).
        Set("ai_model", summary.AIModel).
        Where(sq.Eq{"id": articleID})

    queryString, args, err := query.ToSql()
    if err != nil {
        return ErrorSqlBuild(err)
    }

    _, err = s.GetMaster(ctx).Exec(queryString, args...)
    return err
}

// ListBySubscriptionWithKnowledge 获取订阅文章列表（包含用户的 Knowledge ID）
func (s *RSSArticleStore) ListBySubscriptionWithKnowledge(ctx context.Context, subscriptionID int64, userID, spaceID string, limit int) ([]*types.RSSArticleListItem, error) {
    // 使用 LEFT JOIN 关联用户的 Knowledge
    query := `
        SELECT
            a.*,
            k.id as knowledge_id
        FROM quka_rss_articles a
        LEFT JOIN quka_knowledges k ON (
            k.rel_doc_id = CAST(a.id AS VARCHAR)
            AND k.user_id = $1
            AND k.space_id = $2
            AND k.resource = 'rss'
        )
        WHERE a.subscription_id = $3
        ORDER BY a.published_at DESC
        LIMIT $4
    `

    var items []*types.RSSArticleListItem
    if err := s.GetReplica(ctx).Select(&items, query, userID, spaceID, subscriptionID, limit); err != nil {
        return nil, err
    }

    return items, nil
}

// app/store/sqlstore/knowledge_store.go

// GetByRelDocID 根据关联文档 ID 获取用户的 Knowledge
func (s *KnowledgeStore) GetByRelDocID(ctx context.Context, spaceID, userID, relDocID string) (*types.Knowledge, error) {
    query := sq.Select("*").
        From(s.GetTable()).
        Where(sq.Eq{
            "space_id":   spaceID,
            "user_id":    userID,
            "rel_doc_id": relDocID,
        })

    queryString, args, err := query.ToSql()
    if err != nil {
        return nil, ErrorSqlBuild(err)
    }

    var knowledge types.Knowledge
    if err := s.GetReplica(ctx).Get(&knowledge, queryString, args...); err != nil {
        if err == sql.ErrNoRows {
            return nil, nil
        }
        return nil, err
    }

    return &knowledge, nil
}
```

## API 设计

### 1. 获取订阅的文章列表（带摘要和 Knowledge 关联）

```go
// GET /api/v1/rss/subscriptions/:id/articles

type GetRSSArticlesRequest struct {
    SubscriptionID int64  `path:"id"`
    Page           int    `query:"page" default:"1"`
    PageSize       int    `query:"page_size" default:"20"`
}

type GetRSSArticlesResponse struct {
    Articles []*RSSArticleListItem `json:"articles"`
    Total    int64                 `json:"total"`
}

type RSSArticleListItem struct {
    ID          int64    `json:"id"`
    Title       string   `json:"title"`
    Summary     string   `json:"summary"`       // 共享的 AI 摘要
    Keywords    []string `json:"keywords"`      // 关键词
    Author      string   `json:"author"`
    Link        string   `json:"link"`
    PublishedAt int64    `json:"published_at"`
    KnowledgeID string   `json:"knowledge_id"`  // 🔑 当前用户的 Knowledge ID（用于跳转）
}

// 实现
func (h *RSSHandler) ListRSSArticles(ctx *gin.Context) {
    var req GetRSSArticlesRequest
    // ... 参数绑定

    // 获取订阅信息（验证权限）
    subscription, err := h.logic.GetRSSSubscription(req.SubscriptionID)
    // ... 错误处理

    // 获取文章列表（包含用户的 Knowledge 关联）
    articles, err := h.core.Store().RSSArticleStore().ListBySubscriptionWithKnowledge(
        ctx,
        req.SubscriptionID,
        userID,
        spaceID,
        req.PageSize,
    )

    response.Success(ctx, GetRSSArticlesResponse{
        Articles: articles,
        Total:    total,
    })
}
```

### 2. 点击文章跳转到 Knowledge

```go
// GET /api/v1/rss/articles/:article_id/knowledge

func (h *RSSHandler) GetArticleKnowledge(ctx *gin.Context) {
    articleID := ctx.Param("article_id")
    userID := getUserID(ctx)
    spaceID := getSpaceID(ctx)

    // 查询用户的 Knowledge
    knowledge, err := h.core.Store().KnowledgeStore().GetByRelDocID(
        ctx,
        spaceID,
        userID,
        articleID,
    )

    if knowledge == nil {
        // 用户还没有创建 Knowledge，提示订阅或创建
        response.Error(ctx, "请先订阅此 RSS 源")
        return
    }

    // 返回完整的 Knowledge 内容
    response.Success(ctx, knowledge)
}
```

### 3. 每日智能推荐

```go
// GET /api/v1/rss/recommendations/daily

func (h *RSSHandler) GetDailyRecommendations(ctx *gin.Context) {
    userID := getUserID(ctx)
    spaceID := getSpaceID(ctx)

    // 1. 获取用户兴趣
    interests, _ := h.core.Store().RSSUserInterestStore().GetByUserID(ctx, userID)

    // 2. 基于兴趣和向量相似度推荐文章
    articles, err := h.logic.GetRecommendedArticles(ctx, userID, spaceID, interests)

    // 3. 对于推荐的文章，自动创建 Knowledge（如果用户还没有）
    for _, article := range articles {
        if article.KnowledgeID == "" {
            // 用户还没有这篇文章的 Knowledge，提示或自动创建
        }
    }

    response.Success(ctx, DailyRecommendation{
        Date:     time.Now().Format("2006-01-02"),
        Articles: articles,
    })
}
```

## 工作流程

### 场景 1：用户 A 订阅 RSS

```
1. 用户 A 订阅 "TechCrunch"
2. 系统抓取 10 篇新文章
3. 每篇文章：
   - 保存到 RSSArticle（全局共享）
   - 异步生成 AI 摘要（存在 Article）
   - 为用户 A 创建 Knowledge（个人副本）
   - 向量化用户 A 的 Knowledge
```

### 场景 2：用户 B 也订阅同一个 RSS

```
1. 用户 B 订阅 "TechCrunch"
2. 系统发现文章已存在（GUID 去重）
3. 每篇文章：
   - ✅ 复用已有的 RSSArticle 和摘要（节省成本！）
   - 为用户 B 创建新的 Knowledge（个人副本）
   - 向量化用户 B 的 Knowledge
```

### 场景 3：用户查看文章列表

```
1. 请求：GET /api/v1/rss/subscriptions/1/articles
2. 查询：
   - 获取所有 RSSArticle（含共享摘要）
   - LEFT JOIN 用户的 Knowledge
3. 返回：
   [
     {
       "id": 123,
       "title": "AI 突破",
       "summary": "本文介绍了...",  // 共享摘要
       "knowledge_id": "user_a_k1"  // 用户 A 的 Knowledge
     },
     ...
   ]
```

### 场景 4：用户点击摘要查看详情

```
1. 点击文章（article_id=123）
2. 通过 knowledge_id 跳转
3. GET /api/v1/knowledge/{knowledge_id}
4. 显示完整内容
```

## 优势总结

### ✅ 成本优化
- **AI 摘要共享**：同一篇文章的摘要所有用户共享，只生成一次
- **去重存储**：文章原文只存一份

### ✅ 用户隐私
- **Knowledge 隔离**：每个用户有自己的 Knowledge 副本
- **向量隔离**：每个用户的向量在自己的空间

### ✅ 功能完整
- **快速浏览**：通过共享摘要快速扫描
- **深度阅读**：点击跳转到个人 Knowledge
- **个性化**：基于用户 Knowledge 的向量进行推荐

### ✅ 性能优化
- **摘要缓存**：Article 表缓存摘要，无需重复生成
- **异步处理**：摘要生成不阻塞文章抓取
- **按需加载**：列表只显示摘要，详情按需加载

## 配置建议

```toml
[rss]
# 摘要生成配置
summary_enabled = true
summary_async = true           # 异步生成摘要
summary_batch_size = 10        # 批量处理数量

# 去重配置
dedup_enabled = true           # 启用全局去重
dedup_by_guid = true           # 基于 GUID 去重

# 推荐配置
recommendation_enabled = true
recommendation_count = 10      # 每日推荐数量
```

## 总结

这个修订方案解决了关键问题：

1. ✅ **多用户支持**：同一文章可被多个用户订阅
2. ✅ **成本优化**：AI 摘要共享，只生成一次
3. ✅ **隐私保护**：每个用户有独立的 Knowledge
4. ✅ **完美关联**：通过 `rel_doc_id` 实现双向关联
5. ✅ **用户体验**：摘要 → Knowledge 无缝跳转
