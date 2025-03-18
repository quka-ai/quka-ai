package v1_test

import (
	"testing"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
)

func setupChatSessionLogic() *v1.ChatSessionLogic {
	return v1.NewChatSessionLogic(ctx, setupCore())
}

func Test_CreateChatSession(t *testing.T) {
	logic := setupChatSessionLogic()
	sessionID, err := logic.CreateChatSession("")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(sessionID)
}
