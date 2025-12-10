package v1_test

import (
	"os"
	"testing"
	"time"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
	"github.com/samber/lo"
)

func setupChatLogic() *v1.ChatLogic {
	return v1.NewChatLogic(ctx, NewCore())
}

func Test_GetRelevanceKnowledges(t *testing.T) {
	knowledgeLogic := setupKnowledgeLogic()

	spaceID := os.Getenv("TEST_SPACE_ID")
	userID := os.Getenv("TEST_USER_ID")
	message := "React 路由如何配置？"

	docs, _, err := knowledgeLogic.GetQueryRelevanceKnowledges(spaceID, userID, message, nil)
	if err != nil {
		t.Error(err)
	}

	t.Log(lo.Map(docs.Refs, func(item types.QueryResult, i int) map[string]any {
		return map[string]any{
			"id":  item.KnowledgeID,
			"cos": item.Cos,
		}
	}))
}

func Test_NewMessageSend(t *testing.T) {
	chatLogic := setupChatLogic()
	chatSessionLogic := setupChatSessionLogic()

	spaceID := os.Getenv("TEST_SPACE_ID")
	sessionID := os.Getenv("TEST_SESSION_ID")
	message := "我昨天做了哪些工作？"

	chatSession, err := chatSessionLogic.GetByID(spaceID, sessionID)
	if err != nil {
		t.Fatal(err)
	}

	seqID, err := chatLogic.NewUserMessage(chatSession, types.CreateChatMessageArgs{
		ID:       utils.GenSpecIDStr(),
		SendTime: time.Now().Unix(),
		MsgType:  types.MESSAGE_TYPE_TEXT,
		Message:  message,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(seqID)

	time.Sleep(time.Minute * 3)
}

func TestLoUniq(t *testing.T) {
	t.Log(lo.Union([]int64{1, 2, 3}, []int64{2, 3, 4}))
	t.Log(lo.Difference([]int64{1, 2, 3}, []int64{2, 3, 4}))
}
