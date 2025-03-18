package v1

import (
	"context"

	"github.com/quka-ai/quka-ai/pkg/security"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/samber/lo"
)

const (
	TOKEN_CONTEXT_KEY = "__brew.access_token"
	LANGUAGE_KEY      = "__brew.accept_language"
	APPID_KEY         = "__brew.appid"
)

func InjectAppid(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(APPID_KEY).(string)
	return val, ok
}

// InjectTokenClaim get user/platform token claims from context
func InjectTokenClaim(ctx context.Context) (security.TokenClaims, bool) {
	val, ok := ctx.Value(TOKEN_CONTEXT_KEY).(security.TokenClaims)
	return val, ok
}

const SPACEID_CONTEXT_KEY = "__brew.spaceid"

func InjectSpaceID(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(SPACEID_CONTEXT_KEY).(string)
	return val, ok
}

func InjectLanguage(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(LANGUAGE_KEY).(string)
	return val, ok
}

func GetContentByClientLanguage[T any](c context.Context, enRes T, cnRes T) T {
	clientLang, _ := InjectLanguage(c)
	return lo.If(clientLang == types.LANGUAGE_EN_KEY, enRes).Else(cnRes)
}
