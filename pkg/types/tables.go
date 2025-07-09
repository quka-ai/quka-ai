package types

import "fmt"

type TableName string

func (s TableName) Name() string {
	return fmt.Sprintf("%s%s", TABLE_PREFIX, s)
}

const TABLE_PREFIX = "quka_"

const (
	TABLE_KNOWLEDGE         = TableName("knowledge")
	TABLE_KNOWLEDGE_CHUNK   = TableName("knowledge_chunk")
	TABLE_VECTORS           = TableName("vectors")
	TABLE_ACCESS_TOKEN      = TableName("access_token")
	TABLE_USER_SPACE        = TableName("user_space")
	TABLE_SPACE             = TableName("space")
	TABLE_RESOURCE          = TableName("resource")
	TABLE_USER              = TableName("user")
	TABLE_CHAT_SESSION      = TableName("chat_session")
	TABLE_CHAT_SESSION_PIN  = TableName("chat_session_pin")
	TABLE_CHAT_MESSAGE      = TableName("chat_message")
	TABLE_CHAT_SUMMARY      = TableName("chat_summary")
	TABLE_CHAT_MESSAGE_EXT  = TableName("chat_message_ext")
	TABLE_FILE_MANAGEMENT   = TableName("file_management")
	TABLE_AI_TOKEN_USAGE    = TableName("ai_token_usage")
	TABLE_SHARE_TOKEN       = TableName("share_token")
	TABLE_JOURNAL           = TableName("journal")
	TABLE_BUTLER            = TableName("butler")
	TABLE_SPACE_APPLICATION = TableName("space_application")
	TABLE_MODEL_PROVIDER    = TableName("model_provider")
	TABLE_MODEL_CONFIG      = TableName("model_config")
	TABLE_CUSTOM_CONFIG     = TableName("custom_config")
)
