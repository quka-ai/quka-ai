package handler

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// CreateRSSSubscriptionRequest 创建RSS订阅请求
type CreateRSSSubscriptionRequest struct {
	ResourceID      string `json:"resource_id" binding:"required"`
	URL             string `json:"url" binding:"required,url"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Category        string `json:"category"`
	UpdateFrequency int    `json:"update_frequency"` // 秒，默认3600
}

// CreateRSSSubscriptionResponse 创建RSS订阅响应
type CreateRSSSubscriptionResponse struct {
	*types.RSSSubscription
}

func (s *HttpSrv) CreateRSSSubscription(c *gin.Context) {
	var req CreateRSSSubscriptionRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	subscription, err := v1.NewRSSSubscriptionLogic(c, s.Core).CreateSubscription(
		spaceID,
		req.ResourceID,
		req.URL,
		req.Title,
		req.Description,
		req.Category,
		req.UpdateFrequency,
	)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, CreateRSSSubscriptionResponse{
		RSSSubscription: subscription,
	})
}

func (s *HttpSrv) GetRSSSubscription(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		response.APIError(c, errors.New("GetRSSSubscription.EmptyID", i18n.ERROR_INVALIDARGUMENT, fmt.Errorf("id is required")))
		return
	}

	subscription, err := v1.NewRSSSubscriptionLogic(c, s.Core).GetSubscription(idStr)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, subscription)
}

// ListRSSSubscriptionsRequest 获取RSS订阅列表请求
type ListRSSSubscriptionsRequest struct {
	// 暂时没有查询参数，后续可以添加分页、筛选等
}

func (s *HttpSrv) ListRSSSubscriptions(c *gin.Context) {
	var req ListRSSSubscriptionsRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	subscriptions, err := v1.NewRSSSubscriptionLogic(c, s.Core).ListSubscriptions(spaceID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, subscriptions)
}

// UpdateRSSSubscriptionRequest 更新RSS订阅请求
type UpdateRSSSubscriptionRequest struct {
	Title           string `json:"title"`
	Description     string `json:"description"`
	Category        string `json:"category"`
	UpdateFrequency int    `json:"update_frequency"`
	ResourceID      string `json:"resource_id"`
	Enabled         *bool  `json:"enabled"`
}

func (s *HttpSrv) UpdateRSSSubscription(c *gin.Context) {
	var req UpdateRSSSubscriptionRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	idStr := c.Param("id")
	if idStr == "" {
		response.APIError(c, errors.New("UpdateRSSSubscription.EmptyID", i18n.ERROR_INVALIDARGUMENT, fmt.Errorf("id is required")))
		return
	}

	logic := v1.NewRSSSubscriptionLogic(c, s.Core)
	err := logic.UpdateSubscription(
		idStr,
		req.Title,
		req.Description,
		req.Category,
		req.UpdateFrequency,
		req.ResourceID,
		req.Enabled,
	)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

// DeleteRSSSubscriptionRequest 删除RSS订阅请求
func (s *HttpSrv) DeleteRSSSubscription(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		response.APIError(c, errors.New("DeleteRSSSubscription.EmptyID", i18n.ERROR_INVALIDARGUMENT, fmt.Errorf("id is required")))
		return
	}

	logic := v1.NewRSSSubscriptionLogic(c, s.Core)
	err := logic.DeleteSubscription(idStr)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

// TriggerRSSFetchRequest 手动触发RSS抓取请求
type TriggerRSSFetchRequest struct {
	ID string `json:"id" binding:"required"`
}

func (s *HttpSrv) TriggerRSSFetch(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		response.APIError(c, errors.New("TriggerRSSFetch.EmptyID", i18n.ERROR_INVALIDARGUMENT, fmt.Errorf("id is required")))
		return
	}

	logic := v1.NewRSSSubscriptionLogic(c, s.Core)
	err := logic.TriggerFetch(idStr)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, gin.H{
		"message": "抓取任务已触发",
	})
}

// =============== Daily Digest API ===============

// GenerateDailyDigestRequest 手动触发生成每日摘要请求
type GenerateDailyDigestRequest struct {
	Date string `json:"date" form:"date"` // 可选，格式：2006-01-02，默认为前一天
}

// GenerateDailyDigestResponse 生成每日摘要响应
type GenerateDailyDigestResponse struct {
	ID           string   `json:"id"`
	Date         string   `json:"date"`
	Content      string   `json:"content"`
	ArticleCount int      `json:"article_count"`
	ArticleIDs   []string `json:"article_ids"`
	Model        string   `json:"model"`
	GeneratedAt  int64    `json:"generated_at"`
}

func (s *HttpSrv) GenerateDailyDigest(c *gin.Context) {
	var req GenerateDailyDigestRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	claims, _ := v1.InjectTokenClaim(c)
	userID := claims.User
	spaceID, _ := v1.InjectSpaceID(c)

	// 解析日期，默认为前一天
	var targetDate time.Time
	var dateStr string
	if req.Date != "" {
		parsedDate, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			response.APIError(c, err)
			return
		}
		targetDate = parsedDate
		dateStr = req.Date
	} else {
		targetDate = time.Now().UTC().AddDate(0, 0, -1) // 默认UTC前一天，确保时区一致性
		dateStr = targetDate.Format("2006-01-02")
	}

	// 检查是否已存在
	existing, _ := s.Core.Store().RSSDailyDigestStore().GetByUserAndDate(c, userID, spaceID, dateStr)
	if existing != nil {
		// 已存在，直接返回
		response.APISuccess(c, GenerateDailyDigestResponse{
			ID:           existing.ID,
			Date:         existing.Date,
			Content:      existing.Content,
			ArticleCount: existing.ArticleCount,
			ArticleIDs:   existing.ArticleIDs,
			Model:        existing.AIModel,
			GeneratedAt:  existing.GeneratedAt,
		})
		return
	}

	// 生成每日摘要
	logic := v1.NewRSSDailyDigestLogic(c, s.Core)
	result, err := logic.GenerateDailyDigest(userID, spaceID, targetDate)
	if err != nil {
		response.APIError(c, err)
		return
	}

	// 保存到数据库
	digest := &types.RSSDailyDigest{
		ID:           utils.GenUniqIDStr(),
		UserID:       userID,
		SpaceID:      spaceID,
		Date:         dateStr,
		Content:      result.Content,
		ArticleIDs:   result.ArticleIDs,
		ArticleCount: result.ArticleCount,
		AIModel:      result.Model,
		GeneratedAt:  time.Now().Unix(),
		CreatedAt:    time.Now().Unix(),
	}

	if err := s.Core.Store().RSSDailyDigestStore().Create(c, digest); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, GenerateDailyDigestResponse{
		ID:           digest.ID,
		Date:         digest.Date,
		Content:      digest.Content,
		ArticleCount: digest.ArticleCount,
		ArticleIDs:   digest.ArticleIDs,
		Model:        digest.AIModel,
		GeneratedAt:  digest.GeneratedAt,
	})
}

// GetDailyDigestRequest 获取每日摘要请求
type GetDailyDigestRequest struct {
	Date string `json:"date" form:"date"` // 可选，格式：2006-01-02，默认为前一天
}

// GetDailyDigestResponse 每日摘要响应
type GetDailyDigestResponse struct {
	ID           string   `json:"id"`
	Date         string   `json:"date"`
	Content      string   `json:"content"`
	ArticleCount int      `json:"article_count"`
	ArticleIDs   []string `json:"article_ids"`
	Model        string   `json:"model"`
	GeneratedAt  int64    `json:"generated_at"`
}

func (s *HttpSrv) GetDailyDigest(c *gin.Context) {
	var req GetDailyDigestRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	claims, _ := v1.InjectTokenClaim(c)
	userID := claims.User
	spaceID, _ := v1.InjectSpaceID(c)

	// 解析日期，默认为前一天
	var targetDate time.Time
	var dateStr string
	if req.Date != "" {
		parsedDate, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			response.APIError(c, err)
			return
		}
		targetDate = parsedDate
		dateStr = req.Date
	} else {
		targetDate = time.Now().AddDate(0, 0, -1) // 默认UTC前一天，确保时区一致性
		dateStr = targetDate.Format("2006-01-02")
	}

	// 查询是否已存在
	digest, err := s.Core.Store().RSSDailyDigestStore().GetByUserAndDate(c, userID, spaceID, dateStr)
	if err != nil && err != sql.ErrNoRows {
		response.APIError(c, errors.New("Api.GetDailyDigest.RSSDailyDigestStore.GetByUserAndDate", i18n.ERROR_INTERNAL, nil))
		return
	}

	if digest != nil {
		response.APISuccess(c, GetDailyDigestResponse{
			ID:           digest.ID,
			Date:         digest.Date,
			Content:      digest.Content,
			ArticleCount: digest.ArticleCount,
			ArticleIDs:   digest.ArticleIDs,
			Model:        digest.AIModel,
			GeneratedAt:  digest.GeneratedAt,
		})
		return
	}

	// 不存在，自动生成
	logic := v1.NewRSSDailyDigestLogic(c, s.Core)
	result, genErr := logic.GenerateDailyDigest(userID, spaceID, targetDate)
	if genErr != nil {
		response.APIError(c, genErr)
		return
	}

	// 保存到数据库
	digest = &types.RSSDailyDigest{
		ID:           utils.GenUniqIDStr(),
		UserID:       userID,
		SpaceID:      spaceID,
		Date:         dateStr,
		Content:      result.Content,
		ArticleIDs:   result.ArticleIDs,
		ArticleCount: result.ArticleCount,
		AIModel:      result.Model,
		GeneratedAt:  time.Now().Unix(),
		CreatedAt:    time.Now().Unix(),
	}

	if result.ArticleCount > 0 {
		if saveErr := s.Core.Store().RSSDailyDigestStore().Create(c, digest); saveErr != nil {
			response.APIError(c, errors.New("Api.GetDailyDigest.RSSDailyDigestStore.Create", i18n.ERROR_INTERNAL, saveErr))
			return
		}
	}

	response.APISuccess(c, GetDailyDigestResponse{
		ID:           digest.ID,
		Date:         digest.Date,
		Content:      digest.Content,
		ArticleCount: digest.ArticleCount,
		ArticleIDs:   digest.ArticleIDs,
		Model:        digest.AIModel,
		GeneratedAt:  digest.GeneratedAt,
	})
}

// ListDailyDigestsRequest 获取历史摘要列表请求
type ListDailyDigestsRequest struct {
	StartDate string `json:"start_date" form:"start_date"` // 可选，格式：2006-01-02
	EndDate   string `json:"end_date" form:"end_date"`     // 可选，格式：2006-01-02
	Limit     int    `json:"limit" form:"limit"`           // 可选，默认30
}

// DailyDigestItem 摘要列表项
type DailyDigestItem struct {
	ID           string `json:"id"`
	Date         string `json:"date"`
	ArticleCount int    `json:"article_count"`
	Model        string `json:"model"`
	GeneratedAt  int64  `json:"generated_at"`
}

func (s *HttpSrv) ListDailyDigests(c *gin.Context) {
	var req ListDailyDigestsRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	claims, _ := v1.InjectTokenClaim(c)
	userID := claims.User
	spaceID, _ := v1.InjectSpaceID(c)

	// 设置默认 limit
	limit := req.Limit
	if limit <= 0 {
		limit = 30
	}

	// 查询历史摘要
	var digests []*types.RSSDailyDigest
	var err error

	if req.StartDate != "" && req.EndDate != "" {
		digests, err = s.Core.Store().RSSDailyDigestStore().ListByDateRange(c, userID, spaceID, req.StartDate, req.EndDate, limit)
	} else {
		digests, err = s.Core.Store().RSSDailyDigestStore().ListByUser(c, userID, spaceID, limit)
	}

	if err != nil {
		response.APIError(c, err)
		return
	}

	// 转换为响应格式
	items := make([]*DailyDigestItem, 0, len(digests))
	for _, digest := range digests {
		items = append(items, &DailyDigestItem{
			ID:           digest.ID,
			Date:         digest.Date,
			ArticleCount: digest.ArticleCount,
			Model:        digest.AIModel,
			GeneratedAt:  digest.GeneratedAt,
		})
	}

	response.APISuccess(c, items)
}

func (s *HttpSrv) GetDailyDigestByID(c *gin.Context) {
	idStr := c.Param("id")
	digest, err := s.Core.Store().RSSDailyDigestStore().Get(c, idStr)
	if err != nil {
		response.APIError(c, err)
		return
	}

	claims, _ := v1.InjectTokenClaim(c)
	if digest.UserID != claims.User {
		response.APIError(c, errors.New("Api.DeleteDailyDigest.user", i18n.ERROR_UNAUTHORIZED, nil))
		return
	}

	response.APISuccess(c, GetDailyDigestResponse{
		ID:           digest.ID,
		Date:         digest.Date,
		Content:      digest.Content,
		ArticleCount: digest.ArticleCount,
		ArticleIDs:   digest.ArticleIDs,
		Model:        digest.AIModel,
		GeneratedAt:  digest.GeneratedAt,
	})
}

// DeleteDailyDigestRequest 删除摘要请求
func (s *HttpSrv) DeleteDailyDigest(c *gin.Context) {
	idStr := c.Param("id")

	detail, err := s.Core.Store().RSSDailyDigestStore().Get(c, idStr)
	if err != nil {
		response.APIError(c, err)
		return
	}

	claims, _ := v1.InjectTokenClaim(c)
	if detail.UserID != claims.User {
		response.APIError(c, errors.New("Api.DeleteDailyDigest.user", i18n.ERROR_UNAUTHORIZED, nil))
		return
	}

	err = s.Core.Store().RSSDailyDigestStore().Delete(c, idStr)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
