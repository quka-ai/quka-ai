package handler

import (
	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type CreateResourceRequest struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Cycle       *int   `json:"cycle"`
	Tag         string `json:"tag" binding:"required"`
	Description string `json:"description"`
}

func (s *HttpSrv) CreateResource(c *gin.Context) {
	var (
		err error
		req CreateResourceRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	cycle := 0
	if req.Cycle != nil {
		cycle = *req.Cycle
	}

	spaceID, _ := v1.InjectSpaceID(c)
	if req.ID == "" {
		req.ID = utils.GenUniqIDStr()
	}
	err = v1.NewResourceLogic(c, s.Core).CreateResource(spaceID, req.ID, req.Title, req.Description, req.Tag, cycle)
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type UpdateResourceRequest struct {
	ID          string `json:"id" binding:"required"`
	Title       string `json:"title"`
	Cycle       *int   `json:"cycle"`
	Tag         string `json:"tag"`
	Description string `json:"description"`
}

func (s *HttpSrv) UpdateResource(c *gin.Context) {
	var (
		err error
		req UpdateResourceRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	cycle := 0
	if req.Cycle != nil {
		cycle = *req.Cycle
	}

	spaceID, _ := v1.InjectSpaceID(c)
	err = v1.NewResourceLogic(c, s.Core).Update(spaceID, req.ID, req.Title, req.Description, req.Tag, cycle)
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

func (s *HttpSrv) DeleteResource(c *gin.Context) {
	resourceID, _ := c.Params.Get("resourceid")

	spaceID, _ := v1.InjectSpaceID(c)
	err := v1.NewResourceLogic(c, s.Core).Delete(spaceID, resourceID)
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type ListResponse struct {
	List []types.Resource `json:"list"`
}

func (s *HttpSrv) ListResource(c *gin.Context) {
	spaceID, _ := v1.InjectSpaceID(c)
	list, err := v1.NewResourceLogic(c, s.Core).ListSpaceResources(spaceID)
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, ListResponse{
		List: list,
	})
}

type GetResourceRequest struct {
	ID string `json:"id" binding:"required"`
}

func (s *HttpSrv) GetResource(c *gin.Context) {
	var (
		err error
		req GetResourceRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	data, err := v1.NewResourceLogic(c, s.Core).GetResource(spaceID, req.ID)
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, data)
}

type ListUserResourcesRequest struct {
	Page     uint64 `json:"page"`
	Pagesize uint64 `json:"pagesize"`
}

type ListUserResourcesResponse struct {
	List []types.Resource `json:"list"`
}

func (s *HttpSrv) ListUserResources(c *gin.Context) {
	var (
		err error
		req ListUserResourcesRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	list, err := v1.NewResourceLogic(c, s.Core).ListUserResources(req.Page, req.Pagesize)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, ListUserResourcesResponse{
		List: list,
	})
}
