package handler

import (
	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

func (s *HttpSrv) GetSpaceApplicationLandingDetail(c *gin.Context) {
	token, _ := c.Params.Get("token")

	detail, err := v1.NewSpaceApplicationLogic(c, s.Core).LandingDetail(token)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, detail)
}

type ApplySpaceRequest struct {
	Desc string `json:"desc"`
}

type ApplySpaceResponse struct {
	Status types.SpaceApplicationType `json:"status"`
}

func (s *HttpSrv) ApplySpace(c *gin.Context) {
	var (
		err error
		req ApplySpaceRequest
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	token, _ := c.Params.Get("token")
	applicationStatus, err := v1.NewSpaceApplicationLogic(c, s.Core).Apply(token, req.Desc)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, ApplySpaceResponse{
		Status: applicationStatus,
	})
}

type HandlerSpaceApplicationRequest struct {
	IDs    []string                   `json:"ids"`
	Status types.SpaceApplicationType `json:"status" binding:"required"`
}

func (s *HttpSrv) HandlerSpaceApplication(c *gin.Context) {
	var (
		err error
		req HandlerSpaceApplicationRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}
	spaceID, _ := c.Params.Get("spaceid")
	err = v1.NewSpaceApplicationLogic(c, s.Core).HandlerApplication(spaceID, req.IDs, req.Status)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type GetSpaceApplicationWaitingListRequest struct {
	Page     uint64 `json:"page" form:"page" binding:"required"`
	Pagesize uint64 `json:"pagesize" form:"pagesize" binding:"required"`
	Keywords string `json:"keywords" form:"keywords"`
}

type GetSpaceApplicationWaitingListResponse struct {
	List  []v1.SpaceApplicationWaitingItem `json:"list"`
	Total int64                            `json:"total"`
}

func (s *HttpSrv) GetSpaceApplicationWaitingList(c *gin.Context) {
	var (
		err error
		req GetSpaceApplicationWaitingListRequest
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := c.Params.Get("spaceid")
	list, total, err := v1.NewSpaceApplicationLogic(c, s.Core).WaitingList(spaceID, types.ListSpaceApplicationOptions{
		Status:   types.SPACE_APPLICATION_WAITING,
		Keywords: req.Keywords,
	}, req.Page, req.Pagesize)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, GetSpaceApplicationWaitingListResponse{
		List:  list,
		Total: total,
	})
}
