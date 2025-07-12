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

// ListModelConfigs 获取模型配置列表
func (s *HttpSrv) ListModelConfigs(c *gin.Context) {
	var status *int
	if statusStr := c.Query("status"); statusStr != "" {
		if statusInt, err := strconv.Atoi(statusStr); err == nil {
			status = &statusInt
		}
	}

	var isMultiModal *bool
	if multiModalStr := c.Query("is_multi_modal"); multiModalStr != "" {
		if multiModalBool, err := strconv.ParseBool(multiModalStr); err == nil {
			isMultiModal = &multiModalBool
		}
	}

	providerID := c.Query("provider_id")
	modelType := c.Query("model_type")
	modelName := c.Query("model_name")

	logic := v1.NewModelConfigLogic(c.Request.Context(), s.Core)

	var models []*types.ModelConfig
	var err error
	models, err = logic.ListModelsWithProvider(providerID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	// 客户端过滤（简化版，生产环境建议在数据库层过滤）
	filteredModels := make([]*types.ModelConfig, 0)
	for _, model := range models {
		if status != nil && model.Status != *status {
			continue
		}
		if modelType != "" && model.ModelType != modelType {
			continue
		}
		if modelName != "" && model.ModelName != modelName {
			continue
		}
		if isMultiModal != nil && model.IsMultiModal != *isMultiModal {
			continue
		}
		filteredModels = append(filteredModels, model)
	}

	response.APISuccess(c, map[string]interface{}{
		"list": filteredModels,
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
