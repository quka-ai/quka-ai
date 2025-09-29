package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type GetUserResponse struct {
	UserName    string `json:"user_name"`
	UserID      string `json:"user_id"`
	Avatar      string `json:"avatar"`
	Email       string `json:"email"`
	ServiceMode string `json:"service_mode"`
	PlanID      string `json:"plan_id"`
	Appid       string `json:"appid"`
}

func (s *HttpSrv) GetUser(c *gin.Context) {
	claims, _ := v1.InjectTokenClaim(c)

	user, err := v1.NewUserLogic(c, s.Core).GetUser(claims.Appid, claims.User)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, GetUserResponse{
		UserID:      user.ID,
		Avatar:      user.Avatar,
		UserName:    user.Name,
		Email:       user.Email,
		PlanID:      user.PlanID,
		Appid:       user.Appid,
		ServiceMode: s.Core.Plugins.Name(),
	})
}

type UpdateUserProfileRequest struct {
	UserName string `json:"user_name" form:"user_name" binding:"required,max=32"`
	Email    string `json:"email" form:"email" binding:"required,email"`
	Avatar   string `json:"avatar" form:"avatar"`
}

func (s *HttpSrv) UpdateUserProfile(c *gin.Context) {
	var (
		err error
		req UpdateUserProfileRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	err = v1.NewAuthedUserLogic(c, s.Core).UpdateUserProfile(req.UserName, req.Email, req.Avatar)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type GetUserAccessTokensRequest struct {
	Page     uint64 `json:"page" form:"page" binding:"required"`
	Pagesize uint64 `json:"pagesize" form:"pagesize" binding:"required"`
}

type GetUserAccessTokensResponse struct {
	List []types.AccessToken `json:"list"`
}

func (s *HttpSrv) GetUserAccessTokens(c *gin.Context) {
	var (
		err error
		req GetUserAccessTokensRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	list, err := v1.NewAuthedUserLogic(c, s.Core).GetAccessTokens(req.Page, req.Pagesize)
	if err != nil {
		response.APIError(c, err)
		return
	}

	masked := lo.Map(list, func(item types.AccessToken, _ int) types.AccessToken {
		item.Token = utils.MaskString(item.Token, 6, 4)
		return item
	})

	response.APISuccess(c, GetUserAccessTokensResponse{
		List: masked,
	})
}

type CreateAccessTokenRequest struct {
	Desc string `json:"desc" binding:"required"`
}

func (s *HttpSrv) CreateAccessToken(c *gin.Context) {
	var (
		err error
		req CreateAccessTokenRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	var token string
	if token, err = v1.NewAuthedUserLogic(c, s.Core).CreateAccessToken(req.Desc); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, token)
}

type DeleteAccessTokensRequest struct {
	IDs []int64 `json:"ids" binding:"required"`
}

func (s *HttpSrv) DeleteAccessTokens(c *gin.Context) {
	var (
		err error
		req DeleteAccessTokensRequest
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if err = v1.NewAuthedUserLogic(c, s.Core).DelAccessTokens(req.IDs); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
