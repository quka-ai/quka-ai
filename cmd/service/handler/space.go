package handler

import (
	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type ListUserSpacesResponse struct {
	List []types.UserSpaceDetail `json:"list"`
}

func (s *HttpSrv) ListUserSpaces(c *gin.Context) {
	list, err := v1.NewSpaceLogic(c, s.Core).ListUserSpace()
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, ListUserSpacesResponse{
		List: list,
	})
}

type CreateUserSpaceRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	BasePrompt  string `json:"base_prompt"`
	ChatPrompt  string `json:"chat_prompt"`
}

type CreateUserSpaceResponse struct {
	SpaceID string `json:"space_id"`
}

func (s *HttpSrv) CreateUserSpace(c *gin.Context) {
	var (
		err error
		req CreateUserSpaceRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, err := v1.NewSpaceLogic(c, s.Core).CreateUserSpace(req.Title, req.Description, req.BasePrompt, req.ChatPrompt)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, CreateUserSpaceResponse{
		SpaceID: spaceID,
	})
}

type ListSpaceUsersRequest struct {
	Keywords string `json:"keywords" form:"keywords"`
	Page     uint64 `json:"page" form:"page" binding:"required"`
	PageSize uint64 `json:"pagesize" form:"pagesize" binding:"required,lte=50"`
}

type ListSpaceUsersResponse struct {
	List  []v1.SpaceUser `json:"list"`
	Total int64          `json:"total"`
}

func (s *HttpSrv) ListSpaceUsers(c *gin.Context) {
	var (
		err error
		req ListSpaceUsersRequest
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	list, total, err := v1.NewSpaceLogic(c, s.Core).ListSpaceUsers(spaceID, req.Keywords, req.Page, req.PageSize)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, ListSpaceUsersResponse{
		List:  list,
		Total: total,
	})
}

type SetUserSpaceRoleRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"`
}

func (s *HttpSrv) SetUserSpaceRole(c *gin.Context) {
	var (
		err error
		req SetUserSpaceRoleRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}
	spaceID, _ := v1.InjectSpaceID(c)
	if err = v1.NewSpaceLogic(c, s.Core).SetUserSpaceRole(spaceID, req.UserID, req.Role); err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

func (s *HttpSrv) UpdateSpace(c *gin.Context) {
	var (
		err error
		req CreateUserSpaceRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}
	spaceID, _ := v1.InjectSpaceID(c)
	err = v1.NewSpaceLogic(c, s.Core).UpdateSpace(spaceID, req.Title, req.Description, req.BasePrompt, req.ChatPrompt)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

func (s *HttpSrv) DeleteUserSpace(c *gin.Context) {
	spaceID, _ := v1.InjectSpaceID(c)
	err := v1.NewSpaceLogic(c, s.Core).DeleteUserSpace(spaceID)
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

func (s *HttpSrv) LeaveSpace(c *gin.Context) {
	spaceID, _ := v1.InjectSpaceID(c)
	err := v1.NewSpaceLogic(c, s.Core).LeaveSpace(spaceID)
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

func (s *HttpSrv) RemoveSpaceUser(c *gin.Context) {
	userID, _ := c.Params.Get("userid")
	spaceID, _ := v1.InjectSpaceID(c)
	if err := v1.NewSpaceLogic(c, s.Core).DeleteSpaceUser(spaceID, userID); err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}
