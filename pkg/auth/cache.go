package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/redis/go-redis/v9"

	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

// ValidateTokenFromCache 从缓存中验证 auth token
func ValidateTokenFromCache(ctx context.Context, tokenValue string, cache types.Cache) (*types.UserTokenMeta, error) {
	if tokenValue == "" {
		return nil, errors.New("auth.ValidateTokenFromCache.empty_token", i18n.ERROR_UNAUTHORIZED, nil).Code(http.StatusUnauthorized)
	}

	tokenMetaStr, err := cache.Get(ctx, fmt.Sprintf("user:token:%s", utils.MD5(tokenValue)))
	if err != nil && err != redis.Nil {
		return nil, errors.New("auth.ValidateTokenFromCache.cache_get", i18n.ERROR_INTERNAL, err)
	}

	if tokenMetaStr == "" {
		return nil, errors.New("auth.ValidateTokenFromCache.token_not_found", i18n.ERROR_UNAUTHORIZED, fmt.Errorf("nil token")).Code(http.StatusUnauthorized)
	}

	var meta types.UserTokenMeta
	if err := json.Unmarshal([]byte(tokenMetaStr), &meta); err != nil {
		return nil, errors.New("auth.ValidateTokenFromCache.unmarshal", i18n.ERROR_INTERNAL, err).Code(http.StatusUnauthorized)
	}

	return &meta, nil
}
