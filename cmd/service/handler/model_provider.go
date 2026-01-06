package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// CreateModelProvider 创建模型提供商
func (s *HttpSrv) CreateModelProvider(c *gin.Context) {
	var req v1.CreateProviderRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	logic := v1.NewModelProviderLogic(c, s.Core)
	provider, err := logic.CreateProvider(req)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, provider)
}

// GetModelProvider 获取模型提供商详情
func (s *HttpSrv) GetModelProvider(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.APIError(c, errors.New("GetModelProvider.ID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest))
		return
	}

	logic := v1.NewModelProviderLogic(c, s.Core)
	provider, err := logic.GetProvider(id)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, provider)
}

// ListModelProviders 获取模型提供商列表
func (s *HttpSrv) ListModelProviders(c *gin.Context) {
	var status *int
	if statusStr := c.Query("status"); statusStr != "" {
		if statusInt, err := strconv.Atoi(statusStr); err == nil {
			status = &statusInt
		}
	}

	var isReader *bool
	if readerStr := c.Query("is_reader"); readerStr != "" {
		if readerBool, err := strconv.ParseBool(readerStr); err == nil {
			isReader = &readerBool
		}
	}

	var isOCR *bool
	if ocrStr := c.Query("is_ocr"); ocrStr != "" {
		if ocrBool, err := strconv.ParseBool(ocrStr); err == nil {
			isOCR = &ocrBool
		}
	}

	name := c.Query("name")

	logic := v1.NewModelProviderLogic(c, s.Core)
	providers, err := logic.ListProviders(name, status, isReader, isOCR)
	if err != nil {
		response.APIError(c, err)
		return
	}

	total, err := logic.GetProviderTotal(name, status, isReader, isOCR)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, map[string]interface{}{
		"list":  providers,
		"total": total,
	})
}

// UpdateModelProvider 更新模型提供商
func (s *HttpSrv) UpdateModelProvider(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.APIError(c, errors.New("UpdateModelProvider.ID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest))
		return
	}

	var req v1.UpdateProviderRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	logic := v1.NewModelProviderLogic(c, s.Core)
	provider, err := logic.UpdateProvider(id, req)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, provider)
}

// DeleteModelProvider 删除模型提供商
func (s *HttpSrv) DeleteModelProvider(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.APIError(c, errors.New("DeleteModelProvider.ID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest))
		return
	}

	logic := v1.NewModelProviderLogic(c, s.Core)
	err := logic.DeleteProvider(id)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, map[string]interface{}{
		"message": "提供商删除成功",
	})
}
