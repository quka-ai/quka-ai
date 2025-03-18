package security

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const publicKey = `
-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDRcZ3P3E3V2A11DkO+OwGnUgGP
wFJltZAOraeH/OR+usv//2tlH/Epj07+zZWgDwjCIac8Qr5WC2z53qBpK5JoyIl7
m+b3+IBoZsqW5jn5O32ZFzGVxFZIwERH1MoIW8/u3mCQBy8vopOe6v83MBHnNyWN
t9WpRoecZ+4LqE6iQwIDAQAB
-----END PUBLIC KEY-----
`

const privateKey = `
-----BEGIN PRIVATE KEY-----
MIICdwIBADANBgkqhkiG9w0BAQEFAASCAmEwggJdAgEAAoGBANFxnc/cTdXYDXUO
Q747AadSAY/AUmW1kA6tp4f85H66y///a2Uf8SmPTv7NlaAPCMIhpzxCvlYLbPne
oGkrkmjIiXub5vf4gGhmypbmOfk7fZkXMZXEVkjAREfUyghbz+7eYJAHLy+ik57q
/zcwEec3JY231alGh5xn7guoTqJDAgMBAAECgYB9BsfgF3DXYNvXrqY93teD3G5W
M8Z8NMBEIuHJSevUwwmYTg78FO9Pkd7kODFDlPXzfUdpr6YTk0qAdqdnYL1KfxVG
d/eyS77d7x5N2tOyPird4FQ4iFR80vjVS5cNNQCB4RMJS4LtjkD9Mm8+4n7o2pVE
VOWvaArG9wQJhR7fkQJBAPBoOYoDDHkDOvzUXEGja1ZmwBBOpf+QG1yoINItRElS
kobuGRvnkldal/MRMn2lw5Vr1DMyyAPGKIPOzg1I6f8CQQDfB0ULr5UME+6EQKm/
FGT4zfOPX3gpGCyvmMzgHLBPYviUgZ3+ejCR+A1zUEgwjw9tpoUJf4DIgZ6YoM0S
3B+9AkEAyNwL1wedEC5mxn8XZbAIKnRG3FkZ0GCyu8OtSG2RtEFNM3cTe4ELf/it
I+SmbtxkgR9KeExhWOXjS90pw2e8OQJAVH+umzTU5ZSPo6/UID8b2mA9TS7AmeE8
3PKpfKeh7RH0WF+bepTU3hj7D5t1HmC1WbfjY6vZIR5q9izGRcT8iQJBAKru8t1z
23ZGVdy8n5Ud0Q4CjpJA0yGozzbyyzRzBmVrBGNE0k/tLAtSSJ//DBX40Tx7ubvm
k4NmXzQsMPgW1KM=
-----END PRIVATE KEY-----
`

func Test_JWT(t *testing.T) {
	appid := "test_for_gen"
	jwt, err := GenerateJWT(TokenClaims{
		Appid:   appid,
		AppName: "test_service",
		User:    "tester",
		Fields: map[string]string{
			"version": "1",
			"user_id": "1",
		},
		ExpireTime: time.Now().AddDate(0, 0, 10).Unix(),
		NotBefore:  time.Now().Unix(),
	}, []byte(privateKey))
	if err != nil {
		t.Fatal(err)
	}

	parsedJWT, err := ParseJWT(jwt, []byte(publicKey))
	if err != nil {
		t.Fatal(err)
	}

	if parsedJWT.Appid != appid {
		t.Fatal("jwt not match")
	}

	if parsedJWT.Field("user_id") != "1" {
		t.Fatal("fake jwt")
	}

	t.Log("successful", jwt)
}

func TestErrorIs(t *testing.T) {
	err := fmt.Errorf("error, %w", ErrInvalidJWT)

	t.Log(errors.Is(err, ErrInvalidJWT))
}

func TestClaimsCopy(t *testing.T) {
	ctx := context.Background()
	a := TokenClaims{
		Fields: map[string]string{
			"test": "a",
		},
	}

	ctx = context.WithValue(ctx, "c", a)

	b := ctx.Value("c").(TokenClaims)

	b.Fields["test"] = "b"

	assert.Equal(t, a.Fields["test"], b.Fields["test"])
}
