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

	// 创建提供商
	providerID := utils.GenUniqIDStr()
	provider := types.ModelProvider{
		ID:          providerID,
		Name:        req.Name,
		Description: req.Description,
		ApiUrl:      req.ApiUrl,
		ApiKey:      req.ApiKey,
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
		updated.ApiKey = req.ApiKey
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

	// 检查是否存在关联的模型配置
	models, err := l.core.Store().ModelConfigStore().List(l.ctx, types.ListModelConfigOptions{
		ProviderID: id,
	})
	if err != nil {
		return errors.New("ModelProviderLogic.DeleteProvider.ListModels", i18n.ERROR_INTERNAL, err)
	}

	if len(models) > 0 {
		return errors.New("ModelProviderLogic.DeleteProvider.HasModels", "此提供商下还有模型配置，请先删除所有模型配置", nil).Code(http.StatusBadRequest)
	}

	// 删除提供商
	if err := l.core.Store().ModelProviderStore().Delete(l.ctx, id); err != nil {
		if err == sql.ErrNoRows {
			return errors.New("ModelProviderLogic.DeleteProvider.NotFound", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNotFound)
		}
		return errors.New("ModelProviderLogic.DeleteProvider.Delete", i18n.ERROR_INTERNAL, err)
	}

	return nil
}

// ListProviders 列出模型提供商
func (l *ModelProviderLogic) ListProviders(name string, status *int) ([]types.ModelProvider, error) {
	opts := types.ListModelProviderOptions{
		Name:   name,
		Status: status,
	}

	providers, err := l.core.Store().ModelProviderStore().List(l.ctx, opts, 0, 0)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("ModelProviderLogic.ListProviders.List", i18n.ERROR_INTERNAL, err)
	}

	// 不返回API密钥
	for i := range providers {
		providers[i].ApiKey = ""
	}

	return providers, nil
}

// GetProviderTotal 获取提供商总数
func (l *ModelProviderLogic) GetProviderTotal(name string, status *int) (int64, error) {
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
