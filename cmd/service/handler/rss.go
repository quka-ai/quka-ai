package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
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
	subscription, err := v1.NewRSSSubscriptionLogic(c.Request.Context(), s.Core).CreateSubscription(
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

// GetRSSSubscriptionRequest 获取RSS订阅详情请求
type GetRSSSubscriptionRequest struct {
	ID string `json:"id" form:"id" binding:"required"`
}

func (s *HttpSrv) GetRSSSubscription(c *gin.Context) {
	var req GetRSSSubscriptionRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		response.APIError(c, err)
		return
	}

	subscription, err := v1.NewRSSSubscriptionLogic(c.Request.Context(), s.Core).GetSubscription(id)
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
	subscriptions, err := v1.NewRSSSubscriptionLogic(c.Request.Context(), s.Core).ListSubscriptions(spaceID)
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
	Enabled         *bool  `json:"enabled"`
}

func (s *HttpSrv) UpdateRSSSubscription(c *gin.Context) {
	var req UpdateRSSSubscriptionRequest

	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.APIError(c, err)
		return
	}

	logic := v1.NewRSSSubscriptionLogic(c.Request.Context(), s.Core)
	err = logic.UpdateSubscription(
		id,
		req.Title,
		req.Description,
		req.Category,
		req.UpdateFrequency,
		req.Enabled,
	)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

// DeleteRSSSubscriptionRequest 删除RSS订阅请求
type DeleteRSSSubscriptionRequest struct {
	ID string `json:"id" binding:"required"`
}

func (s *HttpSrv) DeleteRSSSubscription(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.APIError(c, err)
		return
	}

	logic := v1.NewRSSSubscriptionLogic(c.Request.Context(), s.Core)
	err = logic.DeleteSubscription(id)
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
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.APIError(c, err)
		return
	}

	logic := v1.NewRSSSubscriptionLogic(c.Request.Context(), s.Core)
	err = logic.TriggerFetch(id)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, gin.H{
		"message": "抓取任务已触发",
	})
}