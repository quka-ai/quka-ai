package types

import (
	"errors"

	"github.com/quka-ai/quka-ai/pkg/security"
)

const (
	DEFAULT_ACCESS_TOKEN_VERSION = "v1"
)

type AccessToken struct {
	ID        int64  `json:"id" db:"id"`                 // 主键，自增ID
	Appid     string `json:"appid" db:"appid"`           // 租户id
	UserID    string `json:"user_id" db:"user_id"`       // 用户ID，标识该 token 所属的用户
	Token     string `json:"token" db:"token"`           // 第三方用户的 access_token
	Version   string `json:"version" db:"version"`       // token存储格式的版本号，不同版本号对应的token claim结构可能不同
	Info      string `json:"info" db:"info"`             // token 描述，描述 token 的用途或其他信息
	CreatedAt int64  `json:"created_at" db:"created_at"` // 创建时间，UNIX时间戳
	ExpiresAt int64  `json:"expires_at" db:"expires_at"` // 过期时间，UNIX时间戳
}

func (s *AccessToken) TokenClaims() (security.TokenClaims, error) {
	if s.Version != "" && s.Version != DEFAULT_ACCESS_TOKEN_VERSION {
		return security.TokenClaims{}, errors.New("unkown access token version")
	}
	claim := security.NewTokenClaims(s.Appid, "quka", s.UserID, s.UserID, "cs", s.ExpiresAt)
	return claim, nil
}
