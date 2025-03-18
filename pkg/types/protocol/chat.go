package protocol

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	ChatSessionIMTopicPrefix = "/chat_session/"
)

func GenIMTopic(sessionID string) string {
	return fmt.Sprintf("%s%s", ChatSessionIMTopicPrefix, sessionID)
}

func GetChatSessionID(imtopic string) (string, error) {
	idStr := filepath.Base(imtopic)
	return idStr, nil
}

func IsIMTopic(imtopic string) bool {
	return strings.HasPrefix(imtopic, ChatSessionIMTopicPrefix)
}
