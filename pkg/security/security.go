package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"

	"github.com/quka-ai/quka-ai/pkg/utils"
)

const (
	TOKEN_KEY = "Authorization"
)

func GenSign(appid, secret string, signTime int64) string {
	signString := fmt.Sprintf("appid=%s&secret=%s&time=%d", appid, secret, signTime)
	signString += fmt.Sprintf("%s&key=%s", signString, utils.MD5(signString))
	return utils.MD5(signString)
}

type TokenClaims struct {
	Appid      string            `json:"aid"` // 后续可能有用吧
	AppName    string            `json:"an"`
	User       string            `json:"u"`   // 对应平台的用户唯一标识
	Fields     map[string]string `json:"f"`   // unsafe
	ExpireTime int64             `json:"exp"` // 过期时间 时间戳
	NotBefore  int64             `json:"nbf"` // 生效时间 时间戳
}

func NewTokenClaims(appid, appName, userID, planID, roleType string, expireTime int64) TokenClaims {
	return TokenClaims{
		Appid:   appid,
		AppName: appName,
		User:    userID,
		Fields: map[string]string{
			ROLE_TYPE_KEY: roleType,
			PLAN_KEY:      planID,
		},
		ExpireTime: expireTime,
		NotBefore:  time.Now().Unix() - 1,
	}
}

func (t TokenClaims) PlanID() string {
	return t.Fields[PLAN_KEY]
}

const (
	ROLE_KEY      = "role"
	ROLE_TYPE_KEY = "role_type"
	PLAN_KEY      = "plan"
)

func (t TokenClaims) GetRole() string {
	return t.Field("role")
}

func (t TokenClaims) GetRoleType() string {
	return t.Field("role_type")
}

func (t TokenClaims) GetUser() string {
	return t.User
}

func (t TokenClaims) Field(key string) string {
	if t.Fields == nil {
		return ""
	}

	return t.Fields[key]
}

func GenerateJWT(info TokenClaims, signBytes []byte) (string, error) {
	claims := jwt.MapClaims{}

	t := reflect.TypeOf(info)
	v := reflect.ValueOf(info)

	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		claims[tag] = v.Field(i).Interface()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(signBytes)
	if err != nil {
		return "", err
	}
	return token.SignedString(privateKey)
}

var (
	ErrInvalidJWT = errors.New("invalid token")
	ErrPublicKey  = errors.New("invalid public key")
)

func VerifyToken(tokenString string, key []byte) (*TokenClaims, error) {
	claims, err := ParseJWT(tokenString, key)
	if err != nil {
		return nil, err
	}

	if claims.ExpireTime < time.Now().Unix() || claims.NotBefore > time.Now().Unix() {
		return nil, fmt.Errorf("expired token, %w", ErrInvalidJWT)
	}

	return claims, nil
}

func ParseJWT(tokenString string, key []byte) (*TokenClaims, error) {
	result := &TokenClaims{}
	_, err := jwt.Parse(tokenString, func(i2 *jwt.Token) (i interface{}, e error) {
		publicKey, err := jwt.ParseRSAPublicKeyFromPEM(key)
		if err != nil {
			return nil, fmt.Errorf("%s, %w", err.Error(), ErrPublicKey)
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	parts := strings.Split(tokenString, ".")
	claimBytes, _ := jwt.DecodeSegment(parts[1])

	if err = json.Unmarshal(claimBytes, &result); err != nil {
		return result, fmt.Errorf("%s, %w", err.Error(), ErrInvalidJWT)
	}
	return result, nil
}

func NewPerRPCCredential(appid, secret string) *perRPCCredential {
	return &perRPCCredential{
		appid:  appid,
		secret: secret,
	}
}

// perRPCCredential implements "grpccredentials.PerRPCCredentials" interface.
type perRPCCredential struct {
	appid  string
	secret string
}

func newPerRPCCredential() *perRPCCredential { return &perRPCCredential{} }

func (rc *perRPCCredential) RequireTransportSecurity() bool { return false }

func (rc *perRPCCredential) GetRequestMetadata(ctx context.Context, s ...string) (map[string]string, error) {
	now := time.Now().Unix()
	return map[string]string{signAppidKey: rc.appid, signSignKey: GenSign(rc.appid, rc.secret, now), signTimeKey: strconv.FormatInt(now, 10)}, nil
}
