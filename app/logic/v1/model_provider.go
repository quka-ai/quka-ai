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

type ModelProviderLogic struct {
	ctx  context.Context
	core *core.Core
	UserInfo
}

func NewModelProviderLogic(ctx context.Context, core *core.Core) *ModelProviderLogic {
	l := &ModelProviderLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}

	return l
}

// CreateProviderRequest 创建模型提供商请求
type CreateProviderRequest struct {
	Name        string          `json:"name" binding:"required"`    // 提供商名称
	Description string          `json:"description"`                // 提供商描述
	ApiUrl      string          `json:"api_url" binding:"required"` // API地址
	ApiKey      string          `json:"api_key" binding:"required"` // API密钥
	Config      json.RawMessage `json:"config,omitempty"`           // 额外配置
}

// UpdateProviderRequest 更新模型提供商请求
type UpdateProviderRequest struct {
	Name        string          `json:"name,omitempty"`        // 提供商名称
	Description string          `json:"description,omitempty"` // 提供商描述
	ApiUrl      string          `json:"api_url,omitempty"`     // API地址
	ApiKey      string          `json:"api_key,omitempty"`     // API密钥
	Status      *int            `json:"status,omitempty"`      // 状态
	Config      json.RawMessage `json:"config,omitempty"`      // 额外配置
}

// CreateProvider 创建模型提供商
func (l *ModelProviderLogic) CreateProvider(req CreateProviderRequest) (*types.ModelProvider, error) {
	// 参数验证
	if req.Name == "" {
		return nil, errors.New("ModelProviderLogic.CreateProvider.Name.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}
	if req.ApiUrl == "" {
		return nil, errors.New("ModelProviderLogic.CreateProvider.ApiUrl.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}
	if req.ApiKey == "" {
		return nil, errors.New("ModelProviderLogic.CreateProvider.ApiKey.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	// 检查名称是否已存在
	existingProviders, err := l.core.Store().ModelProviderStore().List(l.ctx, types.ListModelProviderOptions{
		Name: req.Name,
	}, 0, 0)
	if err != nil {
		return nil, errors.New("ModelProviderLogic.CreateProvider.List", i18n.ERROR_INTERNAL, err)
	}

	for _, provider := range existingProviders {
		if provider.Name == req.Name {
			return nil, errors.New("ModelProviderLogic.CreateProvider.NameExists", i18n.ERROR_EXIST, nil).Code(http.StatusBadRequest)
		}
	}

	secureToken, err := l.core.EncryptData([]byte(req.ApiKey))
	if err != nil {
		return nil, errors.New("ModelProviderLogic.CreateProvider.EncryptApiKey", i18n.ERROR_INTERNAL, err)
	}

	// 创建提供商
	providerID := utils.GenUniqIDStr()
	provider := types.ModelProvider{
		ID:          providerID,
		Name:        req.Name,
		Description: req.Description,
		ApiUrl:      req.ApiUrl,
		ApiKey:      string(secureToken),
		Status:      types.StatusEnabled,
		Config:      req.Config,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	if err := l.core.Store().ModelProviderStore().Create(l.ctx, provider); err != nil {
		return nil, errors.New("ModelProviderLogic.CreateProvider.Create", i18n.ERROR_INTERNAL, err)
	}

	// 返回时不包含API密钥
	provider.ApiKey = ""
	return &provider, nil
}

// GetProvider 获取模型提供商详情
func (l *ModelProviderLogic) GetProvider(id string) (*types.ModelProvider, error) {
	if id == "" {
		return nil, errors.New("ModelProviderLogic.GetProvider.ID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	provider, err := l.core.Store().ModelProviderStore().Get(l.ctx, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ModelProviderLogic.GetProvider.Get", i18n.ERROR_INTERNAL, err)
	}

	if provider == nil {
		return nil, errors.New("ModelProviderLogic.GetProvider.NotFound", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNotFound)
	}

	// 不返回API密钥
	provider.ApiKey = ""
	return provider, nil
}

// UpdateProvider 更新模型提供商
func (l *ModelProviderLogic) UpdateProvider(id string, req UpdateProviderRequest) (*types.ModelProvider, error) {
	if id == "" {
		return nil, errors.New("ModelProviderLogic.UpdateProvider.ID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	// 获取原有数据
	existing, err := l.core.Store().ModelProviderStore().Get(l.ctx, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ModelProviderLogic.UpdateProvider.Get", i18n.ERROR_INTERNAL, err)
	}

	if existing == nil {
		return nil, errors.New("ModelProviderLogic.UpdateProvider.NotFound", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNotFound)
	}

	// 检查名称冲突（如果更新了名称）
	if req.Name != "" && req.Name != existing.Name {
		existingProviders, err := l.core.Store().ModelProviderStore().List(l.ctx, types.ListModelProviderOptions{
			Name: req.Name,
		}, 0, 0)
		if err != nil {
			return nil, errors.New("ModelProviderLogic.UpdateProvider.List", i18n.ERROR_INTERNAL, err)
		}

		for _, provider := range existingProviders {
			if provider.Name == req.Name && provider.ID != id {
				return nil, errors.New("ModelProviderLogic.UpdateProvider.NameExists", i18n.ERROR_EXIST, nil).Code(http.StatusBadRequest)
			}
		}
	}

	// 更新字段
	updated := *existing
	if req.Name != "" {
		updated.Name = req.Name
	}
	if req.Description != "" {
		updated.Description = req.Description
	}
	if req.ApiUrl != "" {
		updated.ApiUrl = req.ApiUrl
	}
	if req.ApiKey != "" {
		secureToken, err := l.core.EncryptData([]byte(req.ApiKey))
		if err != nil {
			return nil, errors.New("ModelProviderLogic.UpdateProvider.EncryptApiKey", i18n.ERROR_INTERNAL, err)
		}
		updated.ApiKey = string(secureToken)
	}
	if req.Status != nil {
		updated.Status = *req.Status
	}
	if req.Config != nil {
		updated.Config = req.Config
	}
	updated.UpdatedAt = time.Now().Unix()

	if err := l.core.Store().ModelProviderStore().Update(l.ctx, id, updated); err != nil {
		return nil, errors.New("ModelProviderLogic.UpdateProvider.Update", i18n.ERROR_INTERNAL, err)
	}

	// 返回时不包含API密钥
	updated.ApiKey = ""
	return &updated, nil
}

// DeleteProvider 删除模型提供商
func (l *ModelProviderLogic) DeleteProvider(id string) error {
	if id == "" {
		return errors.New("ModelProviderLogic.DeleteProvider.ID.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	// 检查是否有正在使用的模型
	statusEnabled := types.StatusEnabled
	usageConfigs, err := l.core.Store().CustomConfigStore().List(l.ctx, types.ListCustomConfigOptions{
		Category: types.AI_USAGE_CATEGORY,
		Status:   &statusEnabled,
	}, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		return errors.New("ModelProviderLogic.DeleteProvider.CheckUsage", i18n.ERROR_INTERNAL, err)
	}

	// 获取该提供商下的所有模型ID
	models, err := l.core.Store().ModelConfigStore().List(l.ctx, types.ListModelConfigOptions{
		ProviderID: id,
	})
	if err != nil {
		return errors.New("ModelProviderLogic.DeleteProvider.GetModels", i18n.ERROR_INTERNAL, err)
	}

	// 构建模型ID映射表用于快速查找
	modelIDs := make(map[string]bool, len(models))
	for _, model := range models {
		modelIDs[model.ID] = true
	}

	// 检查是否有正在使用的模型
	for _, config := range usageConfigs {
		var modelID string
		if err := json.Unmarshal(config.Value, &modelID); err != nil {
			continue
		}
		if modelIDs[modelID] {
			return errors.New("ModelProviderLogic.DeleteProvider.ModelInUse", i18n.ERROR_PROVIDER_MODEL_IN_USE, nil).Code(http.StatusBadRequest)
		}
	}

	// 批量删除该提供商下的所有模型配置
	if len(models) > 0 {
		if err := l.core.Store().ModelConfigStore().DeleteByProviderID(l.ctx, id); err != nil {
			return errors.New("ModelProviderLogic.DeleteProvider.DeleteModels", i18n.ERROR_INTERNAL, err)
		}
	}

	// 删除提供商
	if err := l.core.Store().ModelProviderStore().Delete(l.ctx, id); err != nil {
		return errors.New("ModelProviderLogic.DeleteProvider.Delete", i18n.ERROR_INTERNAL, err)
	}

	return nil
}

// ListProviders 列出模型提供商
func (l *ModelProviderLogic) ListProviders(name string, status *int, isReader *bool, isOCR *bool) ([]types.ModelProvider, error) {
	opts := types.ListModelProviderOptions{
		Name:   name,
		Status: status,
	}

	providers, err := l.core.Store().ModelProviderStore().List(l.ctx, opts, 0, 0)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ModelProviderLogic.ListProviders.List", i18n.ERROR_INTERNAL, err)
	}

	// 根据is_reader和is_ocr过滤提供商
	if isReader != nil || isOCR != nil {
		filteredProviders := make([]types.ModelProvider, 0)
		for _, provider := range providers {
			var config types.ModelProviderConfig
			if err := json.Unmarshal(provider.Config, &config); err != nil {
				continue // 如果配置解析失败，跳过
			}
			
			// 检查reader过滤条件
			if isReader != nil && config.IsReader != *isReader {
				continue
			}
			
			// 检查OCR过滤条件
			if isOCR != nil && config.IsOCR != *isOCR {
				continue
			}
			
			filteredProviders = append(filteredProviders, provider)
		}
		providers = filteredProviders
	}

	// 不返回API密钥
	for i := range providers {
		providers[i].ApiKey = ""
	}

	return providers, nil
}

// GetProviderTotal 获取提供商总数
func (l *ModelProviderLogic) GetProviderTotal(name string, status *int, isReader *bool, isOCR *bool) (int64, error) {
	// 如果需要按Reader或OCR功能筛选，我们需要获取所有数据然后过滤
	if isReader != nil || isOCR != nil {
		providers, err := l.ListProviders(name, status, isReader, isOCR)
		if err != nil {
			return 0, err
		}
		return int64(len(providers)), nil
	}

	// 否则直接从数据库获取统计数据
	opts := types.ListModelProviderOptions{
		Name:   name,
		Status: status,
	}

	total, err := l.core.Store().ModelProviderStore().Total(l.ctx, opts)
	if err != nil {
		return 0, errors.New("ModelProviderLogic.GetProviderTotal.Total", i18n.ERROR_INTERNAL, err)
	}

	return total, nil
}
