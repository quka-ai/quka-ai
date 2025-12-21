package process

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai/volcengine/voice"
	"github.com/quka-ai/quka-ai/pkg/queue"
	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
)

func init() {
	register.RegisterFunc[*Process](ProcessKey{}, func(p *Process) {
		// 启动 Podcast 消费者
		go startPodcastConsumer(p)

		slog.Info("Podcast task consumer started")
	})
}

// startPodcastConsumer 启动 Podcast Asynq worker
func startPodcastConsumer(p *Process) {
	core := p.Core()

	// 获取或创建 asynq Client 和 Server
	client := p.AsynqClient()
	mux := p.AsynqServerMux()
	if client == nil || mux == nil {
		slog.Error("Asynq client or server not initialized")
		return
	}

	// 使用共享的 Client 和 Server 创建 PodcastQueue，并发数为 2（TTS 生成比较耗时，控制并发）
	podcastQueue := queue.NewPodcastQueueWithClientServer(core.Cfg().Redis.KeyPrefix, client)

	// 保存 PodcastQueue 实例到 Process，以便在 Stop 时关闭
	p.SetPodcastQueue(podcastQueue)

	mux.HandleFunc(queue.TaskTypePodcastGeneration, func(ctx context.Context, task *asynq.Task) error {
		// 从任务中解析 Podcast ID
		var payload queue.PodcastGenerationTask
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			slog.Error("Failed to unmarshal podcast task payload", slog.String("error", err.Error()))
			return err
		}

		slog.Info("Processing podcast generation task",
			slog.String("podcast_id", payload.PodcastID))

		// 获取 Podcast 信息
		podcast, err := core.Store().PodcastStore().Get(ctx, payload.PodcastID)
		if err != nil {
			slog.Error("Failed to get podcast",
				slog.String("podcast_id", payload.PodcastID),
				slog.String("error", err.Error()))
			return err
		}

		// 处理 Podcast 生成
		if err := processPodcastGeneration(ctx, core, podcast); err != nil {
			slog.Error("Failed to generate podcast",
				slog.String("podcast_id", payload.PodcastID),
				slog.String("error", err.Error()))

			// 更新失败状态
			updateErr := core.Store().PodcastStore().UpdateStatus(
				ctx,
				podcast.ID,
				types.PODCAST_STATUS_FAILED,
				err.Error(),
			)
			if updateErr != nil {
				slog.Error("Failed to update podcast status to failed",
					slog.String("podcast_id", podcast.ID),
					slog.String("error", updateErr.Error()))
			}

			// 增加重试次数
			if retryErr := core.Store().PodcastStore().IncrementRetry(ctx, podcast.ID); retryErr != nil {
				slog.Error("Failed to increment retry count",
					slog.String("podcast_id", podcast.ID),
					slog.String("error", retryErr.Error()))
			}

			return err
		}

		slog.Info("Podcast generation completed",
			slog.String("podcast_id", payload.PodcastID))

		return nil
	})
}

// processPodcastGeneration 处理 Podcast 生成任务
func processPodcastGeneration(ctx context.Context, core *core.Core, podcast *types.Podcast) error {
	// 1. 更新状态为 processing
	if err := core.Store().PodcastStore().UpdateStatus(ctx, podcast.ID, types.PODCAST_STATUS_PROCESSING, ""); err != nil {
		return fmt.Errorf("failed to update status to processing: %w", err)
	}

	// 2. 获取源内容
	sourceContent, err := getSourceContentForPodcast(ctx, core, podcast)
	if err != nil {
		return fmt.Errorf("failed to get source content: %w", err)
	}

	// 3. 文本预处理（提取标题、描述和文本内容）
	processedText, title, description, err := preprocessTextForTTS(ctx, core, sourceContent)
	if err != nil {
		return fmt.Errorf("failed to preprocess text: %w", err)
	}

	// 4. 调用 TTS 服务生成音频（带进度回调）
	progressCallback := voice.ProgressCallback(func() {
		// 更新进度时间戳到数据库，表示生成仍在进行中
		if err := core.Store().PodcastStore().UpdateGenerationProgress(ctx, podcast.ID); err != nil {
			slog.Error("Failed to update generation progress timestamp",
				slog.String("podcast_id", podcast.ID),
				slog.String("error", err.Error()))
		}
	})

	result, err := generateAudioWithTTS(ctx, core, podcast.ID, processedText, progressCallback)
	if err != nil {
		return fmt.Errorf("failed to generate audio: %w", err)
	}

	// 5. 下载临时音频并上传到S3（volcengine的URL 1小时后过期）
	s3Path, audioSize, err := downloadAndUploadAudio(ctx, core, podcast.ID, podcast.SpaceID, result.Meta.MetaInfo.AudioUrl)
	if err != nil {
		return fmt.Errorf("failed to upload audio to S3: %w", err)
	}

	// 6. 更新 Podcast 记录（存储相对路径，不包含域名）
	updates := map[string]interface{}{
		"title":          title,
		"description":    description,
		"audio_url":      s3Path,                    // 存储相对路径，响应时再生成预签名URL
		"audio_duration": int(result.AudioDuration), // 转换为秒
		"audio_size":     audioSize,
		"audio_format":   "mp3",
		"tts_provider":   "volcengine",
		"tts_model":      "API-websocket-v3",
		"status":         types.PODCAST_STATUS_COMPLETED,
		"generated_at":   time.Now().Unix(),
	}

	if err := core.Store().PodcastStore().Update(ctx, podcast.ID, updates); err != nil {
		return fmt.Errorf("failed to update podcast: %w", err)
	}

	slog.Info("Podcast generated successfully",
		slog.String("podcast_id", podcast.ID),
		slog.String("s3_path", s3Path),
		slog.Float64("audio_duration", result.AudioDuration),
		slog.Int64("audio_size", audioSize))

	return nil
}

// SourceContentData 源内容数据
type SourceContentData struct {
	Title       string
	Content     string
	ContentType string
}

// getSourceContentForPodcast 获取源内容
func getSourceContentForPodcast(ctx context.Context, core *core.Core, podcast *types.Podcast) (*SourceContentData, error) {
	switch podcast.SourceType {
	case types.PODCAST_SOURCE_KNOWLEDGE:
		knowledge, err := core.Store().KnowledgeStore().GetKnowledge(ctx, podcast.SpaceID, podcast.SourceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get knowledge: %w", err)
		}

		// 1. 解密内容
		decryptedContent, err := core.DecryptData(knowledge.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt knowledge content: %w", err)
		}

		// 2. 根据内容类型转换为 markdown
		var markdownContent string
		if knowledge.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS {
			// blocks 格式转换为 markdown
			markdownContent, err = editorjs.ConvertEditorJSRawToMarkdown(json.RawMessage(decryptedContent))
			if err != nil {
				return nil, fmt.Errorf("failed to convert blocks to markdown: %w", err)
			}
		} else {
			// 其他格式直接使用字符串
			markdownContent = string(decryptedContent)
		}

		return &SourceContentData{
			Title:       knowledge.Title,
			Content:     markdownContent,
			ContentType: "markdown", // 统一转换为 markdown
		}, nil

	case types.PODCAST_SOURCE_RSS_DIGEST:
		digest, err := core.Store().RSSDailyDigestStore().Get(ctx, podcast.SourceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get rss daily digest: %w", err)
		}
		return &SourceContentData{
			Title:       fmt.Sprintf("RSS Daily Digest - %s", digest.Date),
			Content:     digest.Content,
			ContentType: "markdown",
		}, nil

	default:
		return nil, fmt.Errorf("unsupported source type: %s", podcast.SourceType)
	}
}

// preprocessTextForTTS 文本预处理
// 返回：处理后的文本、标题、描述、错误
func preprocessTextForTTS(ctx context.Context, core *core.Core, source *SourceContentData) (string, string, string, error) {
	// TODO: 根据 contentType 进行不同的处理
	// - 如果是 blocks 格式，转换为 markdown
	// - 如果是 markdown，移除格式标记
	// - 如果是 HTML，转换为纯文本
	// - 调用 LLM 优化朗读效果

	slog.Info("Preprocessing text for TTS",
		slog.String("content_type", source.ContentType),
		slog.Int("content_length", len(source.Content)))

	// 简单实现：直接返回内容
	// 实际应该调用 LLM 进行优化
	processedText := source.Content
	title := source.Title
	description := source.Title

	// TODO: 调用 LLM 优化文本
	// - 移除不适合朗读的内容（如代码块、表格等）
	// - 优化语言流畅度
	// - 提取摘要作为 description

	return processedText, title, description, nil
}

// generateAudioWithTTS 调用 TTS 服务生成音频
// 返回 volcengine TTS Result，包含音频 URL 和时长等信息
func generateAudioWithTTS(ctx context.Context, core *core.Core, podcastID string, text string, progressCallback voice.ProgressCallback) (*voice.Result, error) {
	slog.Info("Generating audio with TTS",
		slog.String("podcast_id", podcastID),
		slog.Int("text_length", len(text)))

	// 调用 volcengine Podcast TTS 服务
	podcastService := core.Srv().Podcast()

	// inputID 使用 podcast ID
	result, err := podcastService.Gen(ctx, podcastID, text, true, false, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("TTS generation failed: %w", err)
	}

	slog.Info("TTS audio generated successfully",
		slog.String("podcast_id", podcastID),
		slog.String("audio_url", result.Meta.MetaInfo.AudioUrl),
		slog.Float64("duration", result.AudioDuration),
		slog.String("request_id", result.RequestID))

	return result, nil
}

// downloadAndUploadAudio 从临时URL下载音频并上传到S3
// 返回：S3相对路径、文件大小、错误
func downloadAndUploadAudio(ctx context.Context, core *core.Core, podcastID, spaceID, tempURL string) (string, int64, error) {
	slog.Info("Downloading audio from temporary URL",
		slog.String("podcast_id", podcastID),
		slog.String("temp_url", tempURL))

	// 1. 从临时URL下载音频
	resp, err := http.Get(tempURL)
	if err != nil {
		return "", 0, fmt.Errorf("failed to download audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("failed to download audio: HTTP %d", resp.StatusCode)
	}

	// 2. 读取音频数据
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read audio data: %w", err)
	}

	audioSize := int64(len(audioData))
	slog.Info("Audio downloaded successfully",
		slog.String("podcast_id", podcastID),
		slog.Int64("size", audioSize))

	// 3. 生成S3文件路径（使用随机数避免CDN缓存）
	// 格式: {podcastID}_{随机字符串}.mp3
	randomSuffix := utils.RandomStr(8)
	fileName := fmt.Sprintf("%s_%s.mp3", podcastID, randomSuffix)
	s3Path := types.GenS3FilePath(spaceID, "podcasts", fileName)

	// 4. 上传到S3
	fileStorage := core.Plugins.FileStorage()
	if err := fileStorage.SaveFile(s3Path, audioData); err != nil {
		return "", 0, fmt.Errorf("failed to save audio to S3: %w", err)
	}

	slog.Info("Audio uploaded to S3 successfully",
		slog.String("podcast_id", podcastID),
		slog.String("s3_path", s3Path),
		slog.Int64("size", audioSize))

	// 返回S3相对路径（不包含域名），响应时再生成预签名URL
	return s3Path, audioSize, nil
}
