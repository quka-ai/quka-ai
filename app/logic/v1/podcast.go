package v1

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
)

// PodcastLogic Podcast 业务逻辑
type PodcastLogic struct {
	ctx  context.Context
	core *core.Core
}

// NewPodcastLogic 创建 Podcast 业务逻辑实例
func NewPodcastLogic(ctx context.Context, core *core.Core) *PodcastLogic {
	return &PodcastLogic{
		ctx:  ctx,
		core: core,
	}
}

// CreatePodcast 创建 Podcast
func (l *PodcastLogic) CreatePodcast(userID, spaceID string, req *types.CreatePodcastRequest) (*types.Podcast, error) {
	// 1. 验证源内容是否存在
	sourceContent, err := l.getSourceContent(spaceID, req.SourceType, req.SourceID)
	if err != nil {
		return nil, errors.New("PodcastLogic.CreatePodcast.getSourceContent", i18n.ERROR_INTERNAL, err)
	}

	// 2. 检查是否已经创建过 Podcast
	exists, err := l.core.Store().PodcastStore().ExistsBySource(l.ctx, req.SourceType, req.SourceID)
	if err != nil {
		return nil, errors.New("PodcastLogic.CreatePodcast.ExistsBySource", i18n.ERROR_INTERNAL, err)
	}
	if exists {
		return nil, errors.New("PodcastLogic.CreatePodcast.ExistsBySource", i18n.ERROR_EXIST, fmt.Errorf("podcast already exists for this source"))
	}

	// 3. 创建 Podcast 记录
	podcast := &types.Podcast{
		ID:          utils.GenUniqIDStr(),
		UserID:      userID,
		SpaceID:     spaceID,
		SourceType:  req.SourceType,
		SourceID:    req.SourceID,
		Title:       sourceContent.Title,
		Description: sourceContent.Description,
		Tags:        sourceContent.Tags,
		Status:      types.PODCAST_STATUS_PENDING,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	// 4. 保存到数据库
	if err := l.core.Store().PodcastStore().Create(l.ctx, podcast); err != nil {
		return nil, errors.New("PodcastLogic.CreatePodcast.PodcastStore.Create", i18n.ERROR_INTERNAL, err)
	}

	// 5. 将任务加入队列
	if err := process.PodcastQueue().EnqueueGenerationTask(l.ctx, podcast.ID); err != nil {
		slog.Error("Failed to enqueue podcast generation task",
			slog.String("podcast_id", podcast.ID),
			slog.String("error", err.Error()))
		// 不返回错误，podcast 已创建，后续可以重试
	}

	slog.Info("Podcast created successfully",
		slog.String("podcast_id", podcast.ID),
		slog.String("source_type", string(req.SourceType)),
		slog.String("source_id", req.SourceID))

	return podcast, nil
}

// BatchCreatePodcast 批量创建 Podcast
func (l *PodcastLogic) BatchCreatePodcast(userID, spaceID string, req *types.BatchCreatePodcastRequest) ([]string, error) {
	var podcastIDs []string

	for _, sourceID := range req.SourceIDs {
		podcast, err := l.CreatePodcast(userID, spaceID, &types.CreatePodcastRequest{
			SourceType: req.SourceType,
			SourceID:   sourceID,
		})

		if err != nil {
			slog.Error("Failed to create podcast in batch",
				slog.String("source_type", string(req.SourceType)),
				slog.String("source_id", sourceID),
				slog.String("error", err.Error()))
			continue
		}

		podcastIDs = append(podcastIDs, podcast.ID)
	}

	return podcastIDs, nil
}

// GetPodcast 获取单个 Podcast
func (l *PodcastLogic) GetPodcast(id string) (*types.Podcast, error) {
	podcast, err := l.core.Store().PodcastStore().Get(l.ctx, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("PodcastLogic.GetPodcast.PodcastStore.Get", i18n.ERROR_INTERNAL, err)
	}
	return podcast, nil
}

// GetPodcastBySource 根据源类型和源ID获取 Podcast
func (l *PodcastLogic) GetPodcastBySource(sourceType types.PodcastSourceType, sourceID string) (*types.Podcast, error) {
	podcast, err := l.core.Store().PodcastStore().GetBySource(l.ctx, sourceType, sourceID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("PodcastLogic.GetPodcastBySource.PodcastStore.GetBySource", i18n.ERROR_INTERNAL, err)
	}
	return podcast, nil
}

// ListPodcasts 获取 Podcast 列表
func (l *PodcastLogic) ListPodcasts(spaceID string, req *types.ListPodcastsRequest) (*types.ListPodcastsResponse, error) {
	// 设置默认分页参数
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}

	podcasts, total, err := l.core.Store().PodcastStore().List(l.ctx, spaceID, req)
	if err != nil {
		return nil, errors.New("PodcastLogic.ListPodcasts.PodcastStore.List", i18n.ERROR_INTERNAL, err)
	}

	return &types.ListPodcastsResponse{
		Podcasts: podcasts,
		Total:    total,
	}, nil
}

// DeletePodcast 删除 Podcast
func (l *PodcastLogic) DeletePodcast(id string) error {
	// 1. 获取 Podcast 信息
	podcast, err := l.core.Store().PodcastStore().Get(l.ctx, id)
	if err != nil {
		return errors.New("PodcastLogic.DeletePodcast.PodcastStore.Get", i18n.ERROR_INTERNAL, err)
	}

	// 2. 删除 S3 上的音频文件（如果存在）
	if podcast.AudioURL != "" {
		if err := l.deleteAudioFile(podcast.AudioURL); err != nil {
			slog.Error("Failed to delete audio file",
				slog.String("podcast_id", id),
				slog.String("audio_url", podcast.AudioURL),
				slog.String("error", err.Error()))
			// 继续执行删除操作
		}
	}

	// 3. 删除数据库记录
	if err := l.core.Store().PodcastStore().Delete(l.ctx, id); err != nil {
		return errors.New("PodcastLogic.DeletePodcast.PodcastStore.Delete", i18n.ERROR_INTERNAL, err)
	}

	slog.Info("Podcast deleted successfully", slog.String("podcast_id", id))
	return nil
}

// RegeneratePodcast 重新生成 Podcast
func (l *PodcastLogic) RegeneratePodcast(id string) error {
	// 1. 获取 Podcast 信息
	podcast, err := l.core.Store().PodcastStore().Get(l.ctx, id)
	if err != nil {
		return errors.New("PodcastLogic.RegeneratePodcast.PodcastStore.Get", i18n.ERROR_INTERNAL, err)
	}

	// 2. 删除旧的音频文件（如果存在）
	if podcast.AudioURL != "" {
		if err := l.deleteAudioFile(podcast.AudioURL); err != nil {
			slog.Error("Failed to delete old audio file",
				slog.String("podcast_id", id),
				slog.String("audio_url", podcast.AudioURL),
				slog.String("error", err.Error()))
			// 继续执行删除操作，不返回错误
		}
	}

	// 3. 更新状态为 pending，清空旧的音频信息
	updates := map[string]any{
		"status":         types.PODCAST_STATUS_PENDING,
		"error_message":  "",
		"retry_times":    0,
		"audio_url":      "", // 清空旧音频URL，避免用户访问到旧文件
		"audio_duration": 0,  // 清空音频时长
		"audio_size":     0,  // 清空音频大小
		"generated_at":   0,  // 清空生成时间
	}
	if err := l.core.Store().PodcastStore().Update(l.ctx, id, updates); err != nil {
		return errors.New("PodcastLogic.RegeneratePodcast.PodcastStore.Update", i18n.ERROR_INTERNAL, err)
	}

	// 4. 将任务重新加入队列
	if err := process.PodcastQueue().EnqueueGenerationTask(l.ctx, podcast.ID); err != nil {
		return errors.New("PodcastLogic.RegeneratePodcast.EnqueueGenerationTask", i18n.ERROR_INTERNAL, err)
	}

	slog.Info("Podcast regeneration task enqueued", slog.String("podcast_id", id))
	return nil
}

// SourceContent 源内容信息
type SourceContent struct {
	Title       string
	Description string
	Tags        []string
	Content     string
	ContentType string
}

// getSourceContent 获取源内容
func (l *PodcastLogic) getSourceContent(spaceID string, sourceType types.PodcastSourceType, sourceID string) (*SourceContent, error) {
	switch sourceType {
	case types.PODCAST_SOURCE_KNOWLEDGE:
		return l.getKnowledgeContent(spaceID, sourceID)
	case types.PODCAST_SOURCE_JOURNAL:
		return l.getJournalContent(sourceID)
	case types.PODCAST_SOURCE_RSS_DIGEST:
		return l.getRSSDailyDigestContent(sourceID)
	default:
		return nil, errors.New("PodcastLogic.getSourceContent", i18n.ERROR_INVALIDARGUMENT, fmt.Errorf("unsupported source type: %s", sourceType))
	}
}

// getKnowledgeContent 获取 Knowledge 内容
func (l *PodcastLogic) getKnowledgeContent(spaceID, knowledgeID string) (*SourceContent, error) {
	knowledge, err := l.core.Store().KnowledgeStore().GetKnowledge(l.ctx, spaceID, knowledgeID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("PodcastLogic.getKnowledgeContent.KnowledgeStore.GetKnowledge", i18n.ERROR_NOT_FOUND, err)
	}

	// 1. 解密内容
	decryptedContent, err := l.core.DecryptData(knowledge.Content)
	if err != nil {
		return nil, errors.New("PodcastLogic.getKnowledgeContent.DecryptData", i18n.ERROR_INTERNAL, err)
	}

	// 2. 根据内容类型转换为 markdown
	var markdownContent string
	if knowledge.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS {
		// blocks 格式转换为 markdown
		markdownContent, err = editorjs.ConvertEditorJSRawToMarkdown(json.RawMessage(decryptedContent))
		if err != nil {
			return nil, errors.New("PodcastLogic.getKnowledgeContent.ConvertEditorJSRawToMarkdown", i18n.ERROR_INTERNAL, err)
		}
	} else {
		// 其他格式直接使用字符串
		markdownContent = string(decryptedContent)
	}

	return &SourceContent{
		Title:       knowledge.Title,
		Description: knowledge.Summary,
		Tags:        knowledge.Tags,
		Content:     markdownContent,
		ContentType: "markdown", // 统一转换为 markdown
	}, nil
}

// getJournalContent 获取 Journal 内容
func (l *PodcastLogic) getJournalContent(_ string) (*SourceContent, error) {
	// Journal 的 Get 方法需要 spaceID, userID, date
	// 这里需要根据实际情况调整
	// 暂时返回错误，后续实现
	return nil, errors.New("PodcastLogic.getJournalContent", i18n.ERROR_UNSUPPORTED_FEATURE, fmt.Errorf("journal source not implemented yet"))
}

// getRSSDailyDigestContent 获取 RSS Daily Digest 内容
func (l *PodcastLogic) getRSSDailyDigestContent(digestID string) (*SourceContent, error) {
	digest, err := l.core.Store().RSSDailyDigestStore().Get(l.ctx, digestID)
	if err != nil {
		return nil, errors.New("PodcastLogic.getRSSDailyDigestContent.RSSDailyDigestStore.Get", i18n.ERROR_NOT_FOUND, err)
	}

	return &SourceContent{
		Title:       fmt.Sprintf("RSS Daily Digest - %s", digest.Date),
		Description: "Daily RSS articles digest",
		Tags:        []string{"rss", "digest"},
		Content:     digest.Content,
		ContentType: "markdown",
	}, nil
}

// deleteAudioFile 删除 S3 音频文件
func (l *PodcastLogic) deleteAudioFile(audioURL string) error {
	if audioURL == "" {
		slog.Info("Audio URL is empty, skipping deletion")
		return nil
	}

	slog.Info("Deleting audio file from S3", slog.String("audio_url", audioURL))

	// 获取文件存储接口
	fileStorage := l.core.Plugins.FileStorage()

	// 从 audio_url 中提取 S3 文件路径
	// audio_url 存储的是相对路径（如：uploads/space123/podcasts/20240101/podcastId_random.mp3）
	// 无需额外处理，直接使用即可
	filePath := audioURL

	// 删除文件
	if err := fileStorage.DeleteFile(filePath); err != nil {
		return fmt.Errorf("failed to delete audio file from S3: %w", err)
	}

	slog.Info("Audio file deleted successfully", slog.String("audio_url", audioURL))
	return nil
}
