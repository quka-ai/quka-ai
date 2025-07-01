package core

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupConfigFromEnv(t *testing.T) {
	addr := "localhost:11111"
	os.Setenv("QUKA_API_SERVICE_ADDRESS", addr)

	cfg := LoadBaseConfigFromENV()

	fmt.Println(cfg)

	assert.Equal(t, cfg.Addr, addr)
}
