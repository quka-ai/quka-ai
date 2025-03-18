package types

type AITokenUsage struct {
	SpaceID     string `json:"space_id" db:"space_id"`         // 空间 ID
	UserID      string `json:"user_id" db:"user_id"`           // 用户 ID
	Type        string `json:"type" db:"type"`                 // 主类别
	SubType     string `json:"sub_type" db:"sub_type"`         // 子类别
	ObjectID    string `json:"object_id" db:"object_id"`       // 对象 ID
	Model       string `json:"model" db:"model"`               // 模型名称
	UsagePrompt int    `json:"usage_prompt" db:"usage_prompt"` // 使用的提示词令牌数
	UsageOutput int    `json:"usage_output" db:"usage_output"` // 使用的输出令牌数
	CreatedAt   int64  `json:"created_at" db:"created_at"`     // 记录创建时间
}

type AITokenSummary struct {
	Model       string `json:"model" db:"model"`               // 模型名称
	UsagePrompt int    `json:"usage_prompt" db:"usage_prompt"` // 使用的提示词令牌数
	UsageOutput int    `json:"usage_output" db:"usage_output"` // 使用的输出令牌数
}

type UserTokenUsageWithType struct {
	UserID      string `json:"user_id" db:"user_id"`           // 用户 ID
	Type        string `json:"type" db:"type"`                 // 主类别
	SubType     string `json:"sub_type" db:"sub_type"`         // 子类别
	UsagePrompt int    `json:"usage_prompt" db:"usage_prompt"` // 使用的提示词令牌数
	UsageOutput int    `json:"usage_output" db:"usage_output"` // 使用的输出令牌数
}

type UserTokenUsage struct {
	UserID      string `json:"user_id" db:"user_id"`           // 用户 ID
	UsagePrompt int    `json:"usage_prompt" db:"usage_prompt"` // 使用的提示词令牌数
	UsageOutput int    `json:"usage_output" db:"usage_output"` // 使用的输出令牌数
}

const (
	USAGE_TYPE_KNOWLEDGE = "knowledge"
	USAGE_TYPE_CHAT      = "chat"
	USAGE_TYPE_SYSTEM    = "system"
	USAGE_TYPE_USER      = "user"

	USAGE_SUB_TYPE_EMBEDDING     = "embedding"
	USAGE_SUB_TYPE_SUMMARY       = "summary"
	USAGE_SUB_TYPE_CHAT          = "chat"
	USAGE_SUB_TYPE_QUERY         = "query"
	USAGE_SUB_TYPE_NAMED_CHAT    = "named_chat"
	USAGE_SUB_TYPE_QUERY_ENHANCE = "query_enhance"
	USAGE_SUB_TYPE_RERANK        = "rerank"
	USAGE_SUB_TYPE_READ          = "read"

	USAGE_SUB_TYPE_DESCRIBE_IMAGE = "describe_image"
)
