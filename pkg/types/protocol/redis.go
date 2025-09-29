package protocol

import (
	"fmt"
	"strings"
)

// REDIS_CACHE_KEY_PREFIX redis cache key generator
const (
	REDIS_CACHE_KEY_PREFIX = "quka_"
)

func GenPromptCacheKey(sessionID string) string {
	return fmt.Sprintf("%sprompt_%s", REDIS_CACHE_KEY_PREFIX, sessionID)
}

func GenChatSessionAIRequestKey(sessionID string) string {
	return fmt.Sprintf("%ssession_ai_request_%s", REDIS_CACHE_KEY_PREFIX, sessionID)
}

const (
	RedisCacheKeyNamespaceSep string = ":"
)

const (
	RedisCacheKeyPrefixIMServer RedisCacheKeyProjectPrefix = "imserver"

	RedisCacheKeyPrefixMessage RedisCacheKeyDomainPrefix = "message"

	RedisCacheKeyPrefixMessageEjectOperationCard = "eject-operation-card"
)

const (
	// RedisCacheStringValueDefaultValue String 类型的 Value 的默认值
	//
	// 为什么设置 0 而非空字符串（""）？因为 0 更省内存，参考 https://stackoverflow.com/a/59292672/16189360
	RedisCacheStringValueDefaultValue = 0
)

type RedisCacheKeyProjectPrefix string

type RedisCacheKeyDomainPrefix string

func GenRedisCacheKey(p RedisCacheKeyProjectPrefix, d RedisCacheKeyDomainPrefix, fields ...string) string {
	return strings.Join(append([]string{string(p), string(d)}, fields...), RedisCacheKeyNamespaceSep)
}

// GenIMServerRedisCacheKey imserver:{your-key}
func GenIMServerRedisCacheKey(d RedisCacheKeyDomainPrefix, fields ...string) string {
	return GenRedisCacheKey(RedisCacheKeyPrefixIMServer, d, fields...)
}

// GenIMServerMessageRedisCacheKey imserver:message:{your-key}
func GenIMServerMessageRedisCacheKey(fields ...string) string {
	return GenIMServerRedisCacheKey(RedisCacheKeyPrefixMessage, fields...)
}

// GenIMServerMessageEjectOperationCardRedisCacheKey imserver:message:eject-operation-card:{user-id}
func GenIMServerMessageEjectOperationCardRedisCacheKey(userID string) string {
	return GenIMServerMessageRedisCacheKey(RedisCacheKeyPrefixMessageEjectOperationCard, userID)
}
