package handler

import (
	"github.com/gin-gonic/gin"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type SpaceInvitationRequest struct {
	Invitee string `json:"invitee" binding:"required"`
	Role    string `json:"role" binding:"required"`
}

func (s *HttpSrv) SpaceInvitation(c *gin.Context) {
	var (
		err error
		req SpaceInvitationRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	err = v1.NewSpaceInvitationLogic(c, s.Core).CreateSpaceInvitation(spaceID, req.Invitee, req.Role)
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}
