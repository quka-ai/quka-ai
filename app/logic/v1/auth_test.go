package v1_test

import (
	"context"
	"testing"
	"time"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
)

func Test_GenAccessToken(t *testing.T) {
	userID := ""
	expiresAt := time.Now().AddDate(999, 0, 0).Unix()

	core := NewCore()

	logic := v1.NewAuthLogic(context.Background(), core)
	token, err := logic.GenAccessToken(core.DefaultAppid(), "internal generate", userID, expiresAt)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(token)
}
