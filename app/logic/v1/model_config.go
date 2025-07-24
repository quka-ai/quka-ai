package v1

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type ModelConfigLogic struct {
	ctx  context.Context
	core *core.Core
	UserInfo
}

func NewModelConfigLogic(ctx context.Context, core *core.Core) *ModelConfigLogic {
	l := &ModelConfigLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}

	return l
}

// CreateModelRequest 创建模型配置请求
type CreateModelRequest struct {
	ProviderID   string          `json:"provider_id" binding:"required"`  // 提供商ID
	ModelName    string          `json:"model_name" binding:"required"`   // 模型名称
	DisplayName  string          `json:"display_name" binding:"required"` // 显示名称
	ModelType    string          `json:"model_type" binding:"required"`   // 模型类型
	IsMultiModal bool            `json:"is_multi_modal"`                  // 是否多模态
	Config       json.RawMessage `json:"config,omitempty"`                // 模型配置
}

// UpdateModelRequest 更新模型配置请求
type UpdateModelRequest struct {
	ModelName    string          `json:"model_name,omitempty"`     // 模型名称
	DisplayName  string          `json:"display_name,omitempty"`   // 显示名称
	ModelType    string          `json:"model_type,omitempty"`     // 模型类型
	IsMultiModal *bool           `json:"is_multi_modal,omitempty"` // 是否多模态
	Status       *int            `json:"status,omitempty"`         // 状态
	Config       json.RawMessage `json:"config,omitempty"`         // 模型配置
}

// CreateModel 创建模型配置
func (l *ModelConfigLogic) CreateModel(req CreateModelRequest) (*types.ModelConfig, error) {
	// 参数验证
	if req.ProviderID == "" {
		return nil, errors.New("ModelConfigLogic.CreateModel.ProviderID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}
	if req.ModelName == "" {
		return nil, errors.New("ModelConfigLogic.CreateModel.ModelName.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}
	if req.DisplayName == "" {
		return nil, errors.New("ModelConfigLogic.CreateModel.DisplayName.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}
	if req.ModelType == "" {
		return nil, errors.New("ModelConfigLogic.CreateModel.ModelType.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	// 验证提供商是否存在
	provider, err := l.core.Store().ModelProviderStore().Get(l.ctx, req.ProviderID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ModelConfigLogic.CreateModel.GetProvider", i18n.ERROR_INTERNAL, err)
	}
	if provider == nil {
		return nil, errors.New("ModelConfigLogic.CreateModel.ProviderNotFound", "指定的提供商不存在", nil).Code(http.StatusBadRequest)
	}

	// 检查模型名称在该提供商下是否已存在
	existingModels, err := l.core.Store().ModelConfigStore().List(l.ctx, types.ListModelConfigOptions{
		ProviderID: req.ProviderID,
		ModelName:  req.ModelName,
	})
	if err != nil {
		return nil, errors.New("ModelConfigLogic.CreateModel.ListModels", i18n.ERROR_INTERNAL, err)
	}

	for _, model := range existingModels {
		if model.ModelName == req.ModelName && model.ModelType == req.ModelType {
			return nil, errors.New("ModelConfigLogic.CreateModel.ModelExists", "该提供商下已存在同名模型", nil).Code(http.StatusBadRequest)
		}
	}

	// 创建模型配置
	modelID := utils.GenUniqIDStr()
	model := types.ModelConfig{
		ID:           modelID,
		ProviderID:   req.ProviderID,
		ModelName:    req.ModelName,
		DisplayName:  req.DisplayName,
		ModelType:    req.ModelType,
		IsMultiModal: req.IsMultiModal,
		Status:       types.StatusEnabled,
		Config:       req.Config,
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}

	if err := l.core.Store().ModelConfigStore().Create(l.ctx, model); err != nil {
		return nil, errors.New("ModelConfigLogic.CreateModel.Create", i18n.ERROR_INTERNAL, err)
	}

	return &model, nil
}

// GetModel 获取模型配置详情
func (l *ModelConfigLogic) GetModel(id string) (*types.ModelConfig, error) {
	if id == "" {
		return nil, errors.New("ModelConfigLogic.GetModel.ID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	model, err := l.core.Store().ModelConfigStore().Get(l.ctx, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ModelConfigLogic.GetModel.Get", i18n.ERROR_INTERNAL, err)
	}

	if model == nil {
		return nil, errors.New("ModelConfigLogic.GetModel.NotFound", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNotFound)
	}

	return model, nil
}

// UpdateModel 更新模型配置
func (l *ModelConfigLogic) UpdateModel(id string, req UpdateModelRequest) (*types.ModelConfig, error) {
	if id == "" {
		return nil, errors.New("ModelConfigLogic.UpdateModel.ID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	// 获取原有数据
	existing, err := l.core.Store().ModelConfigStore().Get(l.ctx, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ModelConfigLogic.UpdateModel.Get", i18n.ERROR_INTERNAL, err)
	}

	if existing == nil {
		return nil, errors.New("ModelConfigLogic.UpdateModel.NotFound", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNotFound)
	}

	// 检查模型名称冲突（如果更新了模型名称）
	if req.ModelName != "" && req.ModelName != existing.ModelName {
		existingModels, err := l.core.Store().ModelConfigStore().List(l.ctx, types.ListModelConfigOptions{
			ProviderID: existing.ProviderID,
			ModelName:  req.ModelName,
		})
		if err != nil {
			return nil, errors.New("ModelConfigLogic.UpdateModel.ListModels", i18n.ERROR_INTERNAL, err)
		}

		for _, model := range existingModels {
			if model.ModelName == req.ModelName && model.ID != id {
				return nil, errors.New("ModelConfigLogic.UpdateModel.ModelExists", "该提供商下已存在同名模型", nil).Code(http.StatusBadRequest)
			}
		}
	}

	// 更新字段
	updated := *existing
	if req.ModelName != "" {
		updated.ModelName = req.ModelName
	}
	if req.DisplayName != "" {
		updated.DisplayName = req.DisplayName
	}
	if req.ModelType != "" {
		updated.ModelType = req.ModelType
	}
	if req.IsMultiModal != nil {
		updated.IsMultiModal = *req.IsMultiModal
	}
	if req.Status != nil {
		updated.Status = *req.Status
	}
	if req.Config != nil {
		updated.Config = req.Config
	}
	updated.UpdatedAt = time.Now().Unix()

	if err := l.core.Store().ModelConfigStore().Update(l.ctx, id, updated); err != nil {
		return nil, errors.New("ModelConfigLogic.UpdateModel.Update", i18n.ERROR_INTERNAL, err)
	}

	return &updated, nil
}

// DeleteModel 删除模型配置
func (l *ModelConfigLogic) DeleteModel(id string) error {
	if id == "" {
		return errors.New("ModelConfigLogic.DeleteModel.ID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	// 删除模型配置
	if err := l.core.Store().ModelConfigStore().Delete(l.ctx, id); err != nil {
		if err == sql.ErrNoRows {
			return errors.New("ModelConfigLogic.DeleteModel.NotFound", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNotFound)
		}
		return errors.New("ModelConfigLogic.DeleteModel.Delete", i18n.ERROR_INTERNAL, err)
	}

	return nil
}

// ListModels 列出模型配置
func (l *ModelConfigLogic) ListModels(providerID string) ([]types.ModelConfig, error) {
	opts := types.ListModelConfigOptions{
		ProviderID: providerID,
	}

	models, err := l.core.Store().ModelConfigStore().List(l.ctx, opts)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ModelConfigLogic.ListModels.List", i18n.ERROR_INTERNAL, err)
	}

	return models, nil
}

// ListModelsWithProvider 列出模型配置（包含提供商信息）
func (l *ModelConfigLogic) ListModelsWithProvider(providerID string) ([]*types.ModelConfig, error) {
	opts := types.ListModelConfigOptions{
		ProviderID: providerID,
	}

	models, err := l.core.Store().ModelConfigStore().ListWithProvider(l.ctx, opts)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ModelConfigLogic.ListModelsWithProvider.ListWithProvider", i18n.ERROR_INTERNAL, err)
	}

	// 添加支持Reader功能的提供商作为虚拟模型
	readerModels, err := l.getReaderProviderAsModels(providerID)
	if err != nil {
		return nil, err
	}

	// 合并真实模型和Reader虚拟模型
	allModels := make([]*types.ModelConfig, 0, len(models)+len(readerModels))
	allModels = append(allModels, models...)
	allModels = append(allModels, readerModels...)

	return allModels, nil
}

// getReaderProviderAsModels 获取支持Reader功能的提供商，转换为虚拟模型
func (l *ModelConfigLogic) getReaderProviderAsModels(providerID string) ([]*types.ModelConfig, error) {
	// 构建查询条件
	statusEnabled := types.StatusEnabled
	providerOpts := types.ListModelProviderOptions{
		Status: &statusEnabled,
	}
	if providerID != "" {
		// 如果指定了特定的提供商ID，只查询该提供商
		provider, err := l.core.Store().ModelProviderStore().Get(l.ctx, providerID)
		if err != nil {
			if err == sql.ErrNoRows {
				return []*types.ModelConfig{}, nil
			}
			return nil, errors.New("ModelConfigLogic.getReaderProviderAsModels.Get", i18n.ERROR_INTERNAL, err)
		}

		var config types.ModelProviderConfig
		if err := json.Unmarshal(provider.Config, &config); err != nil {
			return []*types.ModelConfig{}, nil
		}

		if !config.IsReader {
			return []*types.ModelConfig{}, nil
		}

		return l.convertProviderToReaderModel([]*types.ModelProvider{provider}), nil
	}

	// 获取所有启用的提供商
	providers, err := l.core.Store().ModelProviderStore().List(l.ctx, providerOpts, 0, 0)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ModelConfigLogic.getReaderProviderAsModels.List", i18n.ERROR_INTERNAL, err)
	}

	// 过滤出支持Reader功能的提供商
	readerProviders := make([]*types.ModelProvider, 0)
	for i, provider := range providers {
		var config types.ModelProviderConfig
		if err := json.Unmarshal(provider.Config, &config); err != nil {
			continue
		}
		if config.IsReader {
			readerProviders = append(readerProviders, &providers[i])
		}
	}

	return l.convertProviderToReaderModel(readerProviders), nil
}

// convertProviderToReaderModel 将支持Reader的提供商转换为虚拟模型配置
func (l *ModelConfigLogic) convertProviderToReaderModel(providers []*types.ModelProvider) []*types.ModelConfig {
	models := make([]*types.ModelConfig, 0, len(providers))

	for _, provider := range providers {
		model := &types.ModelConfig{
			ID:          provider.ID, // 直接使用provider_id作为虚拟模型ID
			ProviderID:  provider.ID,
			ModelName:   provider.Name, // 使用提供商名称作为模型名称
			DisplayName: provider.Name + " Reader",
			ModelType:   types.MODEL_TYPE_READER, // 虚拟的reader类型
			Status:      provider.Status,
			Config:      provider.Config,
			CreatedAt:   provider.CreatedAt,
			UpdatedAt:   provider.UpdatedAt,
			Provider:    provider,
		}
		models = append(models, model)
	}

	return models
}

// GetModelTotal 获取模型配置总数
func (l *ModelConfigLogic) GetModelTotal(providerID, modelType, modelName string, status *int, isMultiModal *bool) (int64, error) {
	opts := types.ListModelConfigOptions{
		ProviderID:   providerID,
		ModelType:    modelType,
		ModelName:    modelName,
		Status:       status,
		IsMultiModal: isMultiModal,
	}

	total, err := l.core.Store().ModelConfigStore().Total(l.ctx, opts)
	if err != nil {
		return 0, errors.New("ModelConfigLogic.GetModelTotal.Total", i18n.ERROR_INTERNAL, err)
	}

	return total, nil
}

// GetAvailableModels 获取可用的模型配置（只返回启用的）
func (l *ModelConfigLogic) GetAvailableModels(modelType string, isMultiModal *bool) ([]*types.ModelConfig, error) {
	enabledStatus := types.StatusEnabled
	opts := types.ListModelConfigOptions{
		ModelType:    modelType,
		Status:       &enabledStatus,
		IsMultiModal: isMultiModal,
	}

	models, err := l.core.Store().ModelConfigStore().ListWithProvider(l.ctx, opts)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ModelConfigLogic.GetAvailableModels.ListWithProvider", i18n.ERROR_INTERNAL, err)
	}

	// 清除提供商API密钥
	for i := range models {
		if models[i].Provider != nil {
			models[i].Provider.ApiKey = ""
		}
	}

	return models, nil
}
