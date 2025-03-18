package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/holdno/firetower/protocol"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func Websocket(core *core.Core) func(c *gin.Context) {
	if core.Srv().Tower() == nil {
		return func(c *gin.Context) {
			response.APIError(c, errors.New("api.Websocket", "this server not support websocket service", nil))
		}
	}
	return func(c *gin.Context) {
		var ws *websocket.Conn
		var err error

		tower := core.Srv().Tower()
		tokenClaim, _ := v1.InjectTokenClaim(c)

		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			slog.Error("Websocket Upgrade err", slog.String("error", err.Error()))
			response.APIError(c, errors.New("api.Websocket", "failed to upgrade http", err))
			return
		}

		id := utils.GenRandomID()
		thisTower, err := tower.BuildTower(ws, id)
		if err != nil {
			response.APIError(c, errors.New("api.Websocket", "failed to build firetower", err))
			return
		}
		thisTower.SetUserID(tokenClaim.User)

		thisTower.SetReadHandler(func(fire protocol.ReadOnlyFire[srv.PublishData]) bool {
			// 当前用户是不能通过websocket发送消息的，所以固定返回false
			return false
		})

		thisTower.SetReceivedHandler(func(fi protocol.ReadOnlyFire[srv.PublishData]) bool {
			raw, err := json.Marshal(fi.GetMessage())
			if err != nil {
				slog.Error("failed to marshal firetower received message", slog.String("error", err.Error()))
				return false
			}
			thisTower.SendToClient(raw)
			return false
		})

		thisTower.SetReadTimeoutHandler(func(fire protocol.ReadOnlyFire[srv.PublishData]) {
			slog.Error("read timeout trigger", slog.String("component", "firetower"))
		})

		thisTower.SetBeforeSubscribeHandler(func(fireCtx protocol.FireLife, topics []string) bool {
			for _, v := range topics {
				if strings.Contains(v, "/knowledge/list/") {
					spaceID := filepath.Base(v)

					spaceRole, err := core.Store().UserSpaceStore().GetUserSpaceRole(c, tokenClaim.User, spaceID)
					if err != nil || spaceRole == nil {
						slog.Error("failed to subscribe topic, user is not belong to project", slog.String("component", "firetower"),
							slog.String("user", tokenClaim.User), slog.String("topic", v), slog.Any("exist_error", err))
						return false
					}
				} else if strings.Contains(v, "session") {

				} else if strings.Contains(v, "user") {
					// TODO
				} else {
					return false
				}
			}
			return true
		})

		thisTower.SetSubscribeHandler(func(context protocol.FireLife, topic []string) {
			for _, v := range topic {
				resp := &protocol.TopicMessage[json.RawMessage]{
					Topic: v,
					Type:  protocol.SubscribeOperation,
				}
				resp.Data = json.RawMessage(`{"status":"success"}`)
				msg, _ := json.Marshal(resp)
				thisTower.SendToClient(msg)
			}
		})

		thisTower.SetUnSubscribeHandler(func(context protocol.FireLife, topic []string) {
			for _, v := range topic {
				resp := &protocol.TopicMessage[json.RawMessage]{
					Topic: v,
					Type:  protocol.UnSubscribeOperation,
				}
				resp.Data = json.RawMessage(`{"status":"success"}`)
				msg, _ := json.Marshal(resp)
				thisTower.SendToClient(msg)
			}
		})

		thisTower.Run()
	}

}
