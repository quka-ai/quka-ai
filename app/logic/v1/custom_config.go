package v1

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
)

// CustomConfigLogic 自定义配置逻辑
type CustomConfigLogic struct {
	ctx  context.Context
	core *core.Core
	UserInfo
}

// NewCustomConfigLogic 创建新的自定义配置逻辑实例
func NewCustomConfigLogic(ctx context.Context, core *core.Core) *CustomConfigLogic {
	l := &CustomConfigLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}
	return l
}

// UpsertCustomConfig 插入或更新自定义配置
func (l *CustomConfigLogic) UpsertCustomConfig(name, description, category string, value interface{}, status int) (*types.CustomConfig, error) {
	// 参数验证
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("CustomConfigLogic.UpsertCustomConfig.Name.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	if strings.TrimSpace(category) == "" {
		return nil, errors.New("CustomConfigLogic.UpsertCustomConfig.Category.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	// 验证状态值
	if status != types.StatusEnabled && status != types.StatusDisabled {
		return nil, errors.New("CustomConfigLogic.UpsertCustomConfig.Status.Invalid", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	// 序列化值
	valueJson, err := json.Marshal(value)
	if err != nil {
		return nil, errors.New("CustomConfigLogic.UpsertCustomConfig.MarshalValue", i18n.ERROR_INTERNAL, err)
	}

	// 创建配置对象
	config := types.CustomConfig{
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		Value:       valueJson,
		Category:    strings.TrimSpace(category),
		Status:      status,
	}

	// 执行 upsert
	if err := l.core.Store().CustomConfigStore().Upsert(l.ctx, config); err != nil {
		return nil, errors.New("CustomConfigLogic.UpsertCustomConfig.Upsert", i18n.ERROR_INTERNAL, err)
	}

	// 返回更新后的配置
	return l.GetCustomConfig(name)
}

// GetCustomConfig 根据名称获取自定义配置
func (l *CustomConfigLogic) GetCustomConfig(name string) (*types.CustomConfig, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("CustomConfigLogic.GetCustomConfig.Name.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	config, err := l.core.Store().CustomConfigStore().Get(l.ctx, name)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("CustomConfigLogic.GetCustomConfig.Get", i18n.ERROR_INTERNAL, err)
	}

	return config, nil
}

// DeleteCustomConfig 删除自定义配置
func (l *CustomConfigLogic) DeleteCustomConfig(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("CustomConfigLogic.DeleteCustomConfig.Name.Empty", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	// 检查配置是否存在
	existing, err := l.core.Store().CustomConfigStore().Get(l.ctx, name)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("CustomConfigLogic.DeleteCustomConfig.Get", i18n.ERROR_INTERNAL, err)
	}

	if existing == nil {
		return errors.New("CustomConfigLogic.DeleteCustomConfig.NotFound", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNotFound)
	}

	// 删除配置
	if err := l.core.Store().CustomConfigStore().Delete(l.ctx, name); err != nil {
		return errors.New("CustomConfigLogic.DeleteCustomConfig.Delete", i18n.ERROR_INTERNAL, err)
	}

	return nil
}

// ListCustomConfigs 列出自定义配置
func (l *CustomConfigLogic) ListCustomConfigs(nameFilter, category string, status *int, page, pageSize uint64) ([]types.CustomConfig, int64, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 10
	}

	opts := types.ListCustomConfigOptions{
		Name:     nameFilter,
		Category: category,
		Status:   status,
	}

	configs, err := l.core.Store().CustomConfigStore().List(l.ctx, opts, page, pageSize)
	if err != nil {
		return nil, 0, errors.New("CustomConfigLogic.ListCustomConfigs.List", i18n.ERROR_INTERNAL, err)
	}

	total, err := l.core.Store().CustomConfigStore().Total(l.ctx, opts)
	if err != nil {
		return nil, 0, errors.New("CustomConfigLogic.ListCustomConfigs.Total", i18n.ERROR_INTERNAL, err)
	}

	return configs, total, nil
}

// GetCustomConfigValue 获取自定义配置的值（类型安全的获取方法）
func (l *CustomConfigLogic) GetCustomConfigValue(name string, defaultValue interface{}) (interface{}, error) {
	config, err := l.GetCustomConfig(name)
	if err != nil {
		if err == sql.ErrNoRows {
			return defaultValue, nil
		}
		return nil, err
	}

	if config.Status != types.StatusEnabled {
		return defaultValue, nil
	}

	var value interface{}
	if err := json.Unmarshal(config.Value, &value); err != nil {
		return nil, errors.New("CustomConfigLogic.GetCustomConfigValue.UnmarshalValue", i18n.ERROR_INTERNAL, err)
	}

	return value, nil
}

// GetCustomConfigValueString 获取字符串类型的配置值
func (l *CustomConfigLogic) GetCustomConfigValueString(name, defaultValue string) (string, error) {
	value, err := l.GetCustomConfigValue(name, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	if strValue, ok := value.(string); ok {
		return strValue, nil
	}

	return defaultValue, nil
}

// GetCustomConfigValueInt 获取整数类型的配置值
func (l *CustomConfigLogic) GetCustomConfigValueInt(name string, defaultValue int) (int, error) {
	value, err := l.GetCustomConfigValue(name, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	if intValue, ok := value.(int); ok {
		return intValue, nil
	}

	if floatValue, ok := value.(float64); ok {
		return int(floatValue), nil
	}

	return defaultValue, nil
}

// GetCustomConfigValueBool 获取布尔类型的配置值
func (l *CustomConfigLogic) GetCustomConfigValueBool(name string, defaultValue bool) (bool, error) {
	value, err := l.GetCustomConfigValue(name, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	if boolValue, ok := value.(bool); ok {
		return boolValue, nil
	}

	return defaultValue, nil
}

// SetCustomConfigValue 设置自定义配置的值（便捷方法）
func (l *CustomConfigLogic) SetCustomConfigValue(name, category string, value interface{}) error {
	// 先尝试获取现有配置
	existing, err := l.GetCustomConfig(name)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	description := ""
	status := types.StatusEnabled

	// 如果配置已存在，保持原有的描述和状态
	if existing != nil {
		description = existing.Description
		status = existing.Status
	}

	_, err = l.UpsertCustomConfig(name, description, category, value, status)
	return err
}

// BatchUpsertCustomConfigs 批量插入或更新自定义配置（事务安全）
func (l *CustomConfigLogic) BatchUpsertCustomConfigs(configs []types.CustomConfig) error {
	// 使用数据库事务来保证批量操作的原子性
	return l.core.Store().CustomConfigStore().BatchUpsert(l.ctx, configs)
}

// EnableCustomConfig 启用自定义配置
func (l *CustomConfigLogic) EnableCustomConfig(name string) error {
	config, err := l.GetCustomConfig(name)
	if err != nil {
		return err
	}

	if config.Status == types.StatusEnabled {
		return nil // 已经是启用状态
	}

	// 解析当前值
	var currentValue interface{}
	if err := json.Unmarshal(config.Value, &currentValue); err != nil {
		return errors.New("CustomConfigLogic.EnableCustomConfig.UnmarshalValue", i18n.ERROR_INTERNAL, err)
	}

	_, err = l.UpsertCustomConfig(config.Name, config.Description, config.Category, currentValue, types.StatusEnabled)
	return err
}

// DisableCustomConfig 禁用自定义配置
func (l *CustomConfigLogic) DisableCustomConfig(name string) error {
	config, err := l.GetCustomConfig(name)
	if err != nil {
		return err
	}

	if config.Status == types.StatusDisabled {
		return nil // 已经是禁用状态
	}

	// 解析当前值
	var currentValue interface{}
	if err := json.Unmarshal(config.Value, &currentValue); err != nil {
		return errors.New("CustomConfigLogic.DisableCustomConfig.UnmarshalValue", i18n.ERROR_INTERNAL, err)
	}

	_, err = l.UpsertCustomConfig(config.Name, config.Description, config.Category, currentValue, types.StatusDisabled)
	return err
}
