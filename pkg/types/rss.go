package types

import "github.com/lib/pq"

// RSSSubscription RSS订阅源
type RSSSubscription struct {
	ID              string `json:"id" db:"id"`
	UserID          string `json:"user_id" db:"user_id"`
	SpaceID         string `json:"space_id" db:"space_id"`
	ResourceID      string `json:"resource_id" db:"resource_id"`
	URL             string `json:"url" db:"url"`
	Title           string `json:"title" db:"title"`
	Description     string `json:"description" db:"description"`
	Category        string `json:"category" db:"category"`
	UpdateFrequency int    `json:"update_frequency" db:"update_frequency"` // 更新频率（秒）
	LastFetchedAt   int64  `json:"last_fetched_at" db:"last_fetched_at"`   // 上次抓取时间戳
	Enabled         bool   `json:"enabled" db:"enabled"`
	CreatedAt       int64  `json:"created_at" db:"created_at"`
	UpdatedAt       int64  `json:"updated_at" db:"updated_at"`
}

// RSSArticle RSS文章（用于去重）
type RSSArticle struct {
	ID             string `json:"id" db:"id"`
	SubscriptionID string `json:"subscription_id" db:"subscription_id"`
	UserID         string `json:"user_id" db:"user_id"` // 最初订阅该文章的用户ID（用于Token归属）
	GUID           string `json:"guid" db:"guid"`       // RSS item guid
	Title          string `json:"title" db:"title"`
	Link           string `json:"link" db:"link"`
	Description    string `json:"description" db:"description"`
	Content        string `json:"content" db:"content"`
	Author         string `json:"author" db:"author"`

	// AI 生成的摘要（所有订阅用户共享）
	Summary            string         `json:"summary" db:"summary"`
	Keywords           pq.StringArray `json:"keywords" db:"keywords"`
	SummaryGeneratedAt int64          `json:"summary_generated_at" db:"summary_generated_at"`
	AIModel            string         `json:"ai_model" db:"ai_model"`

	// 摘要生成重试相关
	SummaryRetryTimes int    `json:"summary_retry_times" db:"summary_retry_times"` // 摘要生成重试次数
	LastSummaryError  string `json:"last_summary_error" db:"last_summary_error"`   // 最后一次摘要生成错误

	PublishedAt int64 `json:"published_at" db:"published_at"` // 发布时间戳
	FetchedAt   int64 `json:"fetched_at" db:"fetched_at"`     // 抓取时间戳
	CreatedAt   int64 `json:"created_at" db:"created_at"`
}

// RSSArticleSummary RSS 文章摘要更新
type RSSArticleSummary struct {
	Summary            string
	Keywords           []string
	SummaryGeneratedAt int64
	AIModel            string
}

// RSSUserInterest 用户兴趣模型
type RSSUserInterest struct {
	ID            string  `json:"id" db:"id"`
	UserID        string  `json:"user_id" db:"user_id"`
	Topic         string  `json:"topic" db:"topic"`
	Weight        float64 `json:"weight" db:"weight"` // 兴趣权重 0.0-1.0
	Source        string  `json:"source" db:"source"` // explicit 或 implicit
	LastUpdatedAt int64   `json:"last_updated_at" db:"last_updated_at"`
	CreatedAt     int64   `json:"created_at" db:"created_at"`
}

// RSSFeedItem RSS Feed中的单个item（解析用）
type RSSFeedItem struct {
	GUID        string
	Title       string
	Link        string
	Description string
	Content     string
	Author      string
	PublishedAt int64
}

// RSSFeed 解析后的RSS Feed
type RSSFeed struct {
	Title       string
	Description string
	Link        string
	Items       []*RSSFeedItem
}

// RSSDailyDigest 每日RSS摘要
type RSSDailyDigest struct {
	ID           string         `json:"id" db:"id"`
	UserID       string         `json:"user_id" db:"user_id"`
	SpaceID      string         `json:"space_id" db:"space_id"`
	Date         string         `json:"date" db:"date"` // YYYY-MM-DD 格式
	Content      string         `json:"content" db:"content"`
	ArticleIDs   pq.StringArray `json:"article_ids" db:"article_ids"`     // 关联的文章ID列表
	ArticleCount int            `json:"article_count" db:"article_count"` // 文章总数
	AIModel      string         `json:"ai_model" db:"ai_model"`
	GeneratedAt  int64          `json:"generated_at" db:"generated_at"`
	CreatedAt    int64          `json:"created_at" db:"created_at"`
}

// RSSDigestArticle 用于生成摘要的文章信息
type RSSDigestArticle struct {
	ID          string
	Title       string
	Summary     string
	Keywords    []string
	Link        string
	PublishedAt int64
	Source      string // 订阅源名称
	KnowledgeID string // 关联的 Knowledge ID
}
