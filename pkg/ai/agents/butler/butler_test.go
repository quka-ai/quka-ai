package butler_test

import (
	"os"
	"testing"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/ai/agents/butler"
)

func newCore() *core.Core {
	core := core.MustSetupCore(core.MustLoadBaseConfig(os.Getenv("TEST_CONFIG_PATH")))
	return core
}

func newBulter() *butler.ButlerAgent {
	core := newCore()
	return butler.NewButlerAgent(core)
}

func TestBulter(t *testing.T) {
	b := newBulter()
	if b == nil {
		t.Fatal("failed to create bulter")
	}

	nextMessage, err := b.QueryTable("tester")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(nextMessage)
}
