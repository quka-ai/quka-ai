package types

// RSSSubscription RSS订阅源
type RSSSubscription struct {
	ID              int64  `json:"id" db:"id"`
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
	ID             int64  `json:"id" db:"id"`
	SubscriptionID int64  `json:"subscription_id" db:"subscription_id"`
	GUID           string `json:"guid" db:"guid"`                     // RSS item guid
	Title          string `json:"title" db:"title"`
	Link           string `json:"link" db:"link"`
	Description    string `json:"description" db:"description"`
	Content        string `json:"content" db:"content"`
	Author         string `json:"author" db:"author"`
	PublishedAt    int64  `json:"published_at" db:"published_at"` // 发布时间戳
	FetchedAt      int64  `json:"fetched_at" db:"fetched_at"`     // 抓取时间戳
	CreatedAt      int64  `json:"created_at" db:"created_at"`
}

// RSSUserInterest 用户兴趣模型
type RSSUserInterest struct {
	ID            int64   `json:"id" db:"id"`
	UserID        string  `json:"user_id" db:"user_id"`
	Topic         string  `json:"topic" db:"topic"`
	Weight        float64 `json:"weight" db:"weight"`                         // 兴趣权重 0.0-1.0
	Source        string  `json:"source" db:"source"`                         // explicit 或 implicit
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
