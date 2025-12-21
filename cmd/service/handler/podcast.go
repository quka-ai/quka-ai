package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// CreatePodcastResponse 创建播客响应
type CreatePodcastResponse struct {
	ID     string              `json:"id"`
	Status types.PodcastStatus `json:"status"`
}

// CreatePodcast 创建播客
func (s *HttpSrv) CreatePodcast(c *gin.Context) {
	var req types.CreatePodcastRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	tokenClaim, _ := v1.InjectTokenClaim(c)
	userID := tokenClaim.User
	spaceID := c.Param("spaceid")

	// 使用全局的 PodcastQueue 实例（在 logic 层内部获取）
	logic := v1.NewPodcastLogic(c, s.Core)
	podcast, err := logic.CreatePodcast(userID, spaceID, &req)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, CreatePodcastResponse{
		ID:     podcast.ID,
		Status: podcast.Status,
	})
}

// BatchCreatePodcastResponse 批量创建播客响应
type BatchCreatePodcastResponse struct {
	CreatedCount int      `json:"created_count"`
	PodcastIDs   []string `json:"podcast_ids"`
}

// BatchCreatePodcast 批量创建播客
func (s *HttpSrv) BatchCreatePodcast(c *gin.Context) {
	var req types.BatchCreatePodcastRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	tokenClaim, _ := v1.InjectTokenClaim(c)
	userID := tokenClaim.User
	spaceID := c.Param("spaceid")

	// 使用全局的 PodcastQueue 实例（在 logic 层内部获取）
	logic := v1.NewPodcastLogic(c, s.Core)
	podcastIDs, err := logic.BatchCreatePodcast(userID, spaceID, &req)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, BatchCreatePodcastResponse{
		CreatedCount: len(podcastIDs),
		PodcastIDs:   podcastIDs,
	})
}

// GetPodcast 获取单个播客
func (s *HttpSrv) GetPodcast(c *gin.Context) {
	id := c.Param("id")

	// 使用全局的 PodcastQueue 实例（在 logic 层内部获取）
	logic := v1.NewPodcastLogic(c, s.Core)
	podcast, err := logic.GetPodcast(id)
	if err != nil {
		response.APIError(c, err)
		return
	}

	// 生成音频文件的预签名URL
	s.generatePodcastPresignedURL(podcast)

	response.APISuccess(c, podcast)
}

// GetPodcastBySource 根据源类型和源ID获取播客
func (s *HttpSrv) GetPodcastBySource(c *gin.Context) {
	var req types.GetPodcastBySourceRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	logic := v1.NewPodcastLogic(c, s.Core)
	podcast, err := logic.GetPodcastBySource(req.SourceType, req.SourceID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	// 生成音频文件的预签名URL
	s.generatePodcastPresignedURL(podcast)

	response.APISuccess(c, podcast)
}

// ListPodcasts 获取播客列表
func (s *HttpSrv) ListPodcasts(c *gin.Context) {
	var req types.ListPodcastsRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := c.Params.Get("spaceid")
	// 使用全局的 PodcastQueue 实例（在 logic 层内部获取）
	logic := v1.NewPodcastLogic(c, s.Core)
	resp, err := logic.ListPodcasts(spaceID, &req)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, resp)
}

// DeletePodcast 删除播客
func (s *HttpSrv) DeletePodcast(c *gin.Context) {
	id := c.Param("id")

	// 使用全局的 PodcastQueue 实例（在 logic 层内部获取）
	logic := v1.NewPodcastLogic(c, s.Core)
	err := logic.DeletePodcast(id)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

// RegeneratePodcastResponse 重新生成播客响应
type RegeneratePodcastResponse struct {
	Message string `json:"message"`
}

// RegeneratePodcast 重新生成播客
func (s *HttpSrv) RegeneratePodcast(c *gin.Context) {
	id := c.Param("id")

	// 使用全局的 PodcastQueue 实例（在 logic 层内部获取）
	logic := v1.NewPodcastLogic(c, s.Core)
	err := logic.RegeneratePodcast(id)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

// generatePodcastPresignedURL 为 Podcast 的音频文件生成预签名 URL
// 将数据库中存储的 S3 相对路径转换为可访问的预签名 URL
func (s *HttpSrv) generatePodcastPresignedURL(podcast *types.Podcast) {
	// 如果音频URL为空或状态不是已完成，则跳过
	if podcast == nil || podcast.AudioURL == "" || podcast.Status != types.PODCAST_STATUS_COMPLETED {
		return
	}

	// 使用 FileStorage 生成预签名 URL
	fileStorage := s.Core.Plugins.FileStorage()
	presignedURL, err := fileStorage.GenGetObjectPreSignURL(podcast.AudioURL)
	if err != nil {
		slog.Error("Failed to generate presigned URL for podcast audio",
			slog.String("podcast_id", podcast.ID),
			slog.String("s3_path", podcast.AudioURL),
			slog.String("error", err.Error()))
		// 如果生成预签名URL失败，使用静态域名拼接（降级方案）
		podcast.AudioURL = fileStorage.GetStaticDomain() + "/" + podcast.AudioURL
		return
	}

	// 用预签名 URL 替换原始的 S3 路径
	podcast.AudioURL = presignedURL
}
