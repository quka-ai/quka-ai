package v1_test

import (
	"context"
	"testing"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
)

func Test_GenAccessToken(t *testing.T) {

	core := NewCore()

	logic := v1.NewAuthLogic(context.Background(), core)
	token, err := logic.InitAdminUser(core.DefaultAppid())
	if err != nil {
		t.Fatal(err)
	}

	t.Log(token)
}
