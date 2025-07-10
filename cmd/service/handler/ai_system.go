package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
)

// AIUsageRequest AI使用配置请求
type AIUsageRequest struct {
	Chat      string `json:"chat" binding:"required"`
	Embedding string `json:"embedding" binding:"required"`
	Vision    string `json:"vision,omitempty"`
	Rerank    string `json:"rerank,omitempty"`
	Reader    string `json:"reader,omitempty"`
	Enhance   string `json:"enhance,omitempty"`
}

// ReloadAIConfig 重新加载AI配置
func (s *HttpSrv) ReloadAIConfig(c *gin.Context) {
	if err := s.Core.ReloadAI(c.Request.Context()); err != nil {
		response.APIError(c, errors.New("reload failed", i18n.ERROR_INTERNAL, err))
		return
	}

	response.APISuccess(c, map[string]interface{}{
		"message": i18n.MESSAGE_AI_CONFIG_RELOAD_SUCCESS,
		"time":    time.Now().Unix(),
	})
}

// GetAIStatus 获取AI系统状态
func (s *HttpSrv) GetAIStatus(c *gin.Context) {
	status := s.Core.GetAIStatus()
	status["last_reload_time"] = time.Now().Unix()

	response.APISuccess(c, status)
}

// UpdateAIUsage 更新AI使用配置
func (s *HttpSrv) UpdateAIUsage(c *gin.Context) {
	var req AIUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.APIError(c, errors.New("UpdateAIUsage.BindJSON", i18n.ERROR_INVALIDARGUMENT, err).Code(http.StatusBadRequest))
		return
	}

	// 验证模型配置是否存在
	logic := v1.NewModelConfigLogic(c.Request.Context(), s.Core)

	// 验证必需的模型配置
	if req.Chat != "" {
		if _, err := logic.GetModel(req.Chat); err != nil {
			response.APIError(c, errors.New("UpdateAIUsage.ChatModel.NotFound", i18n.ERROR_AI_CHAT_MODEL_NOT_FOUND, err).Code(http.StatusBadRequest))
			return
		}
	}

	if req.Embedding != "" {
		if _, err := logic.GetModel(req.Embedding); err != nil {
			response.APIError(c, errors.New("UpdateAIUsage.EmbeddingModel.NotFound", i18n.ERROR_AI_EMBEDDING_MODEL_NOT_FOUND, err).Code(http.StatusBadRequest))
			return
		}
	}

	// 保存AI使用配置到数据库
	customLogic := v1.NewCustomConfigLogic(c.Request.Context(), s.Core)

	// 构建配置项
	configs := []types.CustomConfig{
		{
			Name:        types.AI_USAGE_CHAT,
			Category:    types.AI_USAGE_CATEGORY,
			Value:       json.RawMessage(`"` + req.Chat + `"`),
			Description: types.AI_USAGE_CHAT_DESC,
			Status:      types.StatusEnabled,
			CreatedAt:   time.Now().Unix(),
			UpdatedAt:   time.Now().Unix(),
		},
		{
			Name:        types.AI_USAGE_EMBEDDING,
			Category:    types.AI_USAGE_CATEGORY,
			Value:       json.RawMessage(`"` + req.Embedding + `"`),
			Description: types.AI_USAGE_EMBEDDING_DESC,
			Status:      types.StatusEnabled,
			CreatedAt:   time.Now().Unix(),
			UpdatedAt:   time.Now().Unix(),
		},
	}

	// 添加可选配置
	if req.Vision != "" {
		configs = append(configs, types.CustomConfig{
			Name:        types.AI_USAGE_VISION,
			Category:    types.AI_USAGE_CATEGORY,
			Value:       json.RawMessage(`"` + req.Vision + `"`),
			Description: types.AI_USAGE_VISION_DESC,
			Status:      types.StatusEnabled,
			CreatedAt:   time.Now().Unix(),
			UpdatedAt:   time.Now().Unix(),
		})
	}

	if req.Rerank != "" {
		configs = append(configs, types.CustomConfig{
			Name:        types.AI_USAGE_RERANK,
			Category:    types.AI_USAGE_CATEGORY,
			Value:       json.RawMessage(`"` + req.Rerank + `"`),
			Description: types.AI_USAGE_RERANK_DESC,
			Status:      types.StatusEnabled,
			CreatedAt:   time.Now().Unix(),
			UpdatedAt:   time.Now().Unix(),
		})
	}

	if req.Reader != "" {
		configs = append(configs, types.CustomConfig{
			Name:        types.AI_USAGE_READER,
			Category:    types.AI_USAGE_CATEGORY,
			Value:       json.RawMessage(`"` + req.Reader + `"`),
			Description: types.AI_USAGE_READER_DESC,
			Status:      types.StatusEnabled,
			CreatedAt:   time.Now().Unix(),
			UpdatedAt:   time.Now().Unix(),
		})
	}

	if req.Enhance != "" {
		configs = append(configs, types.CustomConfig{
			Name:        types.AI_USAGE_ENHANCE,
			Category:    types.AI_USAGE_CATEGORY,
			Value:       json.RawMessage(`"` + req.Enhance + `"`),
			Description: types.AI_USAGE_ENHANCE_DESC,
			Status:      types.StatusEnabled,
			CreatedAt:   time.Now().Unix(),
			UpdatedAt:   time.Now().Unix(),
		})
	}

	// 保存配置（这里简化处理，实际应该是upsert操作）
	for _, config := range configs {
		if err := customLogic.SetCustomConfigValue(config.Name, config.Category, string(config.Value)); err != nil {
			response.APIError(c, errors.New("UpdateAIUsage.SaveConfig", i18n.ERROR_INTERNAL, err))
			return
		}
	}

	response.APISuccess(c, map[string]interface{}{
		"message": i18n.MESSAGE_AI_USAGE_UPDATE_SUCCESS,
		"configs": configs,
	})
}

// GetAIUsage 获取AI使用配置
func (s *HttpSrv) GetAIUsage(c *gin.Context) {
	customLogic := v1.NewCustomConfigLogic(c.Request.Context(), s.Core)

	// 获取AI使用配置
	configs, _, err := customLogic.ListCustomConfigs("", types.AI_USAGE_CATEGORY, nil, 0, 0)
	if err != nil {
		response.APIError(c, errors.New("GetAIUsage.ListConfigs", i18n.ERROR_INTERNAL, err))
		return
	}

	// 构建响应
	usage := make(map[string]string)
	for _, config := range configs {
		var modelID string
		if err := json.Unmarshal(config.Value, &modelID); err != nil {
			continue
		}

		switch config.Name {
		case types.AI_USAGE_CHAT:
			usage["chat"] = modelID
		case types.AI_USAGE_EMBEDDING:
			usage["embedding"] = modelID
		case types.AI_USAGE_VISION:
			usage["vision"] = modelID
		case types.AI_USAGE_RERANK:
			usage["rerank"] = modelID
		case types.AI_USAGE_READER:
			usage["reader"] = modelID
		case types.AI_USAGE_ENHANCE:
			usage["enhance"] = modelID
		}
	}

	response.APISuccess(c, usage)
}
