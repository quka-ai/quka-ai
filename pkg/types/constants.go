package types

// AI配置相关常量
const (
	// AI使用配置分类
	AI_USAGE_CATEGORY = "ai_usage"

	// AI使用配置名称
	AI_USAGE_CHAT          = "ai_usage_chat"
	AI_USAGE_CHAT_THINKING = "ai_usage_chat_thinking"
	AI_USAGE_EMBEDDING     = "ai_usage_embedding"
	AI_USAGE_VISION        = "ai_usage_vision"
	AI_USAGE_RERANK        = "ai_usage_rerank"
	AI_USAGE_READER        = "ai_usage_reader"
	AI_USAGE_ENHANCE       = "ai_usage_enhance"

	// 模型类型常量
	MODEL_TYPE_CHAT       = "chat"
	MODEL_TYPE_EMBEDDING  = "embedding"
	MODEL_TYPE_COMPLETION = "completion"
	MODEL_TYPE_VISION     = "vision"
	MODEL_TYPE_RERANK     = "rerank"
	MODEL_TYPE_ENHANCE    = "enhance"
	MODEL_TYPE_READER     = "reader" // 虚拟模型类型，用于标识Reader提供商
)

// AI使用配置描述
const (
	AI_USAGE_CHAT_DESC          = "聊天功能使用的模型"
	AI_USAGE_CHAT_THINKING_DESC = "思考聊天模型配置"
	AI_USAGE_EMBEDDING_DESC     = "向量化功能使用的模型"
	AI_USAGE_VISION_DESC        = "视觉功能使用的模型"
	AI_USAGE_RERANK_DESC        = "重排序功能使用的模型"
	AI_USAGE_READER_DESC        = "阅读功能使用的模型"
	AI_USAGE_ENHANCE_DESC       = "增强功能使用的模型"
)

// 分页相关常量已在common.go中定义
