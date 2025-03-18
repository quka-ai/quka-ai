package v1_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/quka-ai/quka-ai/app/core"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/pkg/plugins"
)

func NewCore() *core.Core {
	core := core.MustSetupCore(core.MustLoadBaseConfig(os.Getenv("TEST_CONFIG_PATH")))
	plugins.Setup(core.InstallPlugins, "selfhost")
	return core
}

func Test_UserRegister(t *testing.T) {
	core := NewCore()
	logic := v1.NewUserLogic(context.Background(), core)

	userName := ""
	userEmail := ""

	userID, err := logic.Register(core.DefaultAppid(), userName, userEmail, "testpwd", "Main")
	if err != nil {
		t.Fatal(err)
	}

	user, err := logic.GetUser(core.DefaultAppid(), userID)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, user.Name, userName)
	t.Log(userID)
}
