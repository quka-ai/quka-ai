package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// CreateModelConfig 创建模型配置
func (s *HttpSrv) CreateModelConfig(c *gin.Context) {
	var req v1.CreateModelRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	logic := v1.NewModelConfigLogic(c.Request.Context(), s.Core)
	model, err := logic.CreateModel(req)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, model)
}

// GetModelConfig 获取模型配置详情
func (s *HttpSrv) GetModelConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.APIError(c, errors.New("GetModelConfig.ID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest))
		return
	}

	logic := v1.NewModelConfigLogic(c.Request.Context(), s.Core)
	model, err := logic.GetModel(id)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, model)
}

// ListModelConfigsRequest 获取模型配置列表请求
type ListModelConfigsRequest struct {
	ProviderID       string `form:"provider_id"`       // 提供商ID
	ModelType        string `form:"model_type"`        // 模型类型
	ModelName        string `form:"model_name"`        // 模型名称（用于搜索 display_name）
	Status           *int   `form:"status"`            // 状态
	IsMultiModal     *bool  `form:"is_multi_modal"`    // 是否多模态
	ThinkingSupport  *int   `form:"thinking_support"`  // 思考功能支持类型
	ThinkingRequired *bool  `form:"thinking_required"` // 是否需要思考功能
}

// ListModelConfigs 获取模型配置列表
func (s *HttpSrv) ListModelConfigs(c *gin.Context) {
	var req ListModelConfigsRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	logic := v1.NewModelConfigLogic(c.Request.Context(), s.Core)

	// 构建筛选选项
	opts := types.ListModelConfigOptions{
		ProviderID:       req.ProviderID,
		ModelType:        req.ModelType,
		DisplayName:      req.ModelName, // 注意：前端的 model_name 参数映射到 DisplayName 字段
		Status:           req.Status,
		IsMultiModal:     req.IsMultiModal,
		ThinkingSupport:  req.ThinkingSupport,
		ThinkingRequired: req.ThinkingRequired,
	}

	// 直接使用筛选条件查询，不再在内存中过滤
	models, err := logic.ListModelsWithProviderFiltered(opts)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, map[string]interface{}{
		"list": models,
	})
}

// UpdateModelConfig 更新模型配置
func (s *HttpSrv) UpdateModelConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.APIError(c, errors.New("UpdateModelConfig.ID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest))
		return
	}

	var req v1.UpdateModelRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	logic := v1.NewModelConfigLogic(c.Request.Context(), s.Core)
	model, err := logic.UpdateModel(id, req)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, model)
}

// DeleteModelConfig 删除模型配置
func (s *HttpSrv) DeleteModelConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.APIError(c, errors.New("DeleteModelConfig.ID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest))
		return
	}

	logic := v1.NewModelConfigLogic(c.Request.Context(), s.Core)
	err := logic.DeleteModel(id)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, map[string]interface{}{
		"message": "模型配置删除成功",
	})
}

// GetAvailableModels 获取可用的模型配置
func (s *HttpSrv) GetAvailableModels(c *gin.Context) {
	modelType := c.Query("model_type")

	var isMultiModal *bool
	if isMultiModalStr := c.Query("is_multi_modal"); isMultiModalStr != "" {
		if val, err := strconv.ParseBool(isMultiModalStr); err == nil {
			isMultiModal = &val
		}
	}

	var thinkingRequired *bool
	if thinkingRequiredStr := c.Query("thinking_required"); thinkingRequiredStr != "" {
		if val, err := strconv.ParseBool(thinkingRequiredStr); err == nil {
			thinkingRequired = &val
		}
	}

	logic := v1.NewModelConfigLogic(c.Request.Context(), s.Core)
	models, err := logic.GetAvailableModels(modelType, isMultiModal, thinkingRequired)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, models)
}

// GetThinkingModels 获取支持思考功能的模型
func (s *HttpSrv) GetThinkingModels(c *gin.Context) {
	var needsThinking bool = true
	if needsThinkingStr := c.Query("needs_thinking"); needsThinkingStr != "" {
		if val, err := strconv.ParseBool(needsThinkingStr); err == nil {
			needsThinking = val
		}
	}

	logic := v1.NewModelConfigLogic(c.Request.Context(), s.Core)
	models, err := logic.GetAvailableThinkingModels(needsThinking)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, models)
}
