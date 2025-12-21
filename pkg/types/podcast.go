package types

import (
	"github.com/lib/pq"
)

// PodcastSourceType 播客来源类型
type PodcastSourceType string

const (
	PODCAST_SOURCE_KNOWLEDGE  PodcastSourceType = "knowledge"
	PODCAST_SOURCE_JOURNAL    PodcastSourceType = "journal"
	PODCAST_SOURCE_RSS_DIGEST PodcastSourceType = "rss_digest"
)

// PodcastStatus 播客状态
type PodcastStatus string

const (
	PODCAST_STATUS_PENDING    PodcastStatus = "pending"
	PODCAST_STATUS_PROCESSING PodcastStatus = "processing"
	PODCAST_STATUS_COMPLETED  PodcastStatus = "completed"
	PODCAST_STATUS_FAILED     PodcastStatus = "failed"
)

// Podcast 播客
type Podcast struct {
	ID      string `json:"id" db:"id"`
	UserID  string `json:"user_id" db:"user_id"`
	SpaceID string `json:"space_id" db:"space_id"`

	// 来源信息
	SourceType PodcastSourceType `json:"source_type" db:"source_type"`
	SourceID   string            `json:"source_id" db:"source_id"`

	// 基本信息
	Title       string         `json:"title" db:"title"`
	Description string         `json:"description" db:"description"`
	Tags        pq.StringArray `json:"tags" db:"tags"`

	// 音频信息
	AudioURL      string `json:"audio_url" db:"audio_url"`
	AudioDuration int    `json:"audio_duration" db:"audio_duration"`
	AudioSize     int64  `json:"audio_size" db:"audio_size"`
	AudioFormat   string `json:"audio_format" db:"audio_format"`

	// TTS 配置
	TTSProvider string `json:"-" db:"tts_provider"`
	TTSModel    string `json:"-" db:"tts_model"`

	// 状态信息
	Status                PodcastStatus `json:"status" db:"status"`
	ErrorMessage          string        `json:"error_message" db:"error_message"`
	RetryTimes            int           `json:"retry_times" db:"retry_times"`
	GenerationLastUpdated int64         `json:"generation_last_updated" db:"generation_last_updated"`

	// 时间戳
	CreatedAt   int64 `json:"created_at" db:"created_at"`
	UpdatedAt   int64 `json:"updated_at" db:"updated_at"`
	GeneratedAt int64 `json:"generated_at" db:"generated_at"`
}

// CreatePodcastRequest 创建播客请求
type CreatePodcastRequest struct {
	SourceType PodcastSourceType `json:"source_type" binding:"required"`
	SourceID   string            `json:"source_id" binding:"required"`
}

// BatchCreatePodcastRequest 批量创建播客请求
type BatchCreatePodcastRequest struct {
	SourceType PodcastSourceType `json:"source_type" binding:"required"`
	SourceIDs  []string          `json:"source_ids" binding:"required"`
}

// ListPodcastsRequest 获取播客列表请求
type ListPodcastsRequest struct {
	SourceType PodcastSourceType `form:"source_type"`
	Status     PodcastStatus     `form:"status"`
	Page       int               `form:"page" binding:"min=1"`
	PageSize   int               `form:"pagesize" binding:"min=1,max=20"`
}

// ListPodcastsResponse 获取播客列表响应
type ListPodcastsResponse struct {
	Podcasts []*Podcast `json:"podcasts"`
	Total    int64      `json:"total"`
}

// GetPodcastBySourceRequest 根据源类型和源ID获取播客请求
type GetPodcastBySourceRequest struct {
	SourceType PodcastSourceType `form:"source_type" binding:"required"`
	SourceID   string            `form:"source_id" binding:"required"`
}
