package protocol

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	ChatSessionIMTopicPrefix = "/chat_session/"
	KnowledgeListTopicPrefix = "/knowledge/list/"
	UserTopicPrefix          = "/user/"
)

func GenIMTopic(spaceID, sessionID string) string {
	return fmt.Sprintf("%s%s/%s", ChatSessionIMTopicPrefix, spaceID, sessionID)
}

func GetChatSessionID(imtopic string) (string, error) {
	idStr := filepath.Base(imtopic)
	return idStr, nil
}

func IsIMTopic(imtopic string) bool {
	return strings.HasPrefix(imtopic, ChatSessionIMTopicPrefix)
}
