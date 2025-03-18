package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupFromENV(t *testing.T) {
	core := MustSetupCore(LoadBaseConfigFromENV())
	assert.NotEqual(t, core, nil)
}
