package i18n

import (
	"testing"
)

func TestLang(t *testing.T) {
	l := NewLocalizer("zh-CN", "en")
	t.Log(l.GetWithData("en", "error.internal", map[string]interface{}{
		"message": "123",
	}))

	t.Log(l.Get("en", "error.internal"))
}
