package handler

import (
	"github.com/gin-gonic/gin"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type ToolsReaderRequest struct {
	Endpoint string `json:"endpoint" form:"endpoint" binding:"required"`
}

func (s *HttpSrv) ToolsReader(c *gin.Context) {
	var (
		err error
		req ToolsReaderRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	res, err := v1.NewReaderLogic(c, s.Core).Reader(req.Endpoint)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, res)
}

type DescribeImageRequest struct {
	URL string `json:"url"`
}

type DescribeImageResponse struct {
	Content string `json:"content"`
}

func (s *HttpSrv) DescribeImage(c *gin.Context) {
	var (
		err error
		req DescribeImageRequest
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	// v1.KnowledgeQueryResult
	result, err := v1.NewReaderLogic(c, s.Core).DescribeImage(req.URL)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, DescribeImageResponse{
		Content: result,
	})
}
