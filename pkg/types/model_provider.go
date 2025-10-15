package types

import (
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/cloudwego/eino/components/model"
)

// CustomConfig 自定义配置
type CustomConfig struct {
	Name        string          `json:"name" db:"name"`               // 配置名称（主键）
	Description string          `json:"description" db:"description"` // 配置描述
	Value       json.RawMessage `json:"value" db:"value"`             // 配置值（JSON格式）
	Category    string          `json:"category" db:"category"`       // 配置分类
	Status      int             `json:"status" db:"status"`           // 状态：1-启用，0-禁用
	CreatedAt   int64           `json:"created_at" db:"created_at"`   // 创建时间
	UpdatedAt   int64           `json:"updated_at" db:"updated_at"`   // 更新时间
}

// ModelProvider 模型提供商
type ModelProvider struct {
	ID          string          `json:"id" db:"id"`                   // 提供商ID
	Name        string          `json:"name" db:"name"`               // 提供商名称（如：链接流动、白山、AiHubMix）
	Description string          `json:"description" db:"description"` // 提供商描述
	ApiUrl      string          `json:"api_url" db:"api_url"`         // API地址
	ApiKey      string          `json:"-" db:"api_key"`               // API密钥（不返回给前端）
	Status      int             `json:"status" db:"status"`           // 状态：1-启用，0-禁用
	Config      json.RawMessage `json:"config" db:"config"`           // 额外配置（JSON格式，包含is_reader等厂商特有功能配置）
	CreatedAt   int64           `json:"created_at" db:"created_at"`   // 创建时间
	UpdatedAt   int64           `json:"updated_at" db:"updated_at"`   // 更新时间
}

// ModelProviderConfig 模型提供商配置结构
type ModelProviderConfig struct {
	IsReader   bool `json:"is_reader"`   // 是否支持Reader功能（厂商特有功能）
	Timeout    int  `json:"timeout"`     // 请求超时时间（秒）
	MaxRetries int  `json:"max_retries"` // 最大重试次数
}

// ModelConfig 模型配置
type ModelConfig struct {
	ID              string          `json:"id" db:"id"`                             // 配置ID
	ProviderID      string          `json:"provider_id" db:"provider_id"`           // 提供商ID
	ModelName       string          `json:"model_name" db:"model_name"`             // 模型名称（如：BAAI/bge-m3）
	DisplayName     string          `json:"display_name" db:"display_name"`         // 显示名称
	ModelType       string          `json:"model_type" db:"model_type"`             // 模型类型（chat/embedding/completion）
	IsMultiModal    bool            `json:"is_multi_modal" db:"is_multi_modal"`     // 是否是多模态模型（仅对chat类型有效）
	ThinkingSupport int             `json:"thinking_support" db:"thinking_support"` // 思考功能支持类型：0-不支持，1-可选，2-强制
	Status          int             `json:"status" db:"status"`                     // 状态：1-启用，0-禁用
	Config          json.RawMessage `json:"config" db:"config"`                     // 模型配置（JSON格式，包含参数、限制等）
	CreatedAt       int64           `json:"created_at" db:"created_at"`             // 创建时间
	UpdatedAt       int64           `json:"updated_at" db:"updated_at"`             // 更新时间

	// 关联数据
	Provider *ModelProvider `json:"provider,omitempty" db:"-"` // 提供商信息
}

type ChatModel interface {
	model.ToolCallingChatModel
	Config() ModelConfig
}

type CommonAIWithMeta struct {
	model.ToolCallingChatModel
	Cfg ModelConfig
}

func (c *CommonAIWithMeta) Config() ModelConfig {
	return c.Cfg
}

// 查询选项
type ListModelProviderOptions struct {
	Status *int
	Name   string
}

func (opt ListModelProviderOptions) Apply(query *sq.SelectBuilder) {
	if opt.Status != nil {
		*query = query.Where(sq.Eq{"status": *opt.Status})
	}
	if opt.Name != "" {
		*query = query.Where(sq.Like{"name": "%" + opt.Name + "%"})
	}
}

type ListModelConfigOptions struct {
	ProviderID       string
	ModelType        string
	ModelName        string // 精确匹配 model_name 字段
	DisplayName      string // 模糊搜索 display_name 字段
	IsMultiModal     *bool
	ThinkingSupport  *int  // 思考功能支持过滤：0-不支持，1-可选，2-强制
	ThinkingRequired *bool // 是否需要思考功能（用于筛选支持思考的模型）
	Status           *int
}

func (opt ListModelConfigOptions) Apply(query *sq.SelectBuilder) {
	if opt.ProviderID != "" {
		*query = query.Where(sq.Eq{"provider_id": opt.ProviderID})
	}
	if opt.ModelType != "" {
		*query = query.Where(sq.Eq{"model_type": opt.ModelType})
	}
	if opt.IsMultiModal != nil {
		*query = query.Where(sq.Eq{"is_multi_modal": *opt.IsMultiModal})
	}
	if opt.ThinkingSupport != nil {
		*query = query.Where(sq.Eq{"thinking_support": *opt.ThinkingSupport})
	}
	if opt.ThinkingRequired != nil {
		if *opt.ThinkingRequired {
			// 筛选支持思考的模型（可选或强制）
			*query = query.Where(sq.Gt{"thinking_support": ThinkingSupportNone})
		} else {
			// 筛选不需要思考的模型（不支持或可选）
			*query = query.Where(sq.Lt{"thinking_support": ThinkingSupportForced})
		}
	}
	if opt.Status != nil {
		*query = query.Where(sq.Eq{"status": *opt.Status})
	}
	if opt.ModelName != "" {
		*query = query.Where(sq.Like{"model_name": "%" + opt.ModelName + "%"})
	}
	if opt.DisplayName != "" {
		*query = query.Where(sq.Like{"display_name": "%" + opt.DisplayName + "%"})
	}
}

// 思考功能支持类型常量
const (
	ThinkingSupportNone     = 0 // 不支持思考
	ThinkingSupportOptional = 1 // 可选思考
	ThinkingSupportForced   = 2 // 强制思考
)

type ListCustomConfigOptions struct {
	Name     string
	Category string
	Status   *int
}

func (opt ListCustomConfigOptions) Apply(query *sq.SelectBuilder) {
	if opt.Name != "" {
		*query = query.Where(sq.Like{"name": "%" + opt.Name + "%"})
	}
	if opt.Category != "" {
		*query = query.Where(sq.Eq{"category": opt.Category})
	}
	if opt.Status != nil {
		*query = query.Where(sq.Eq{"status": *opt.Status})
	}
}

// 状态常量
const (
	StatusDisabled = 0
	StatusEnabled  = 1
)
