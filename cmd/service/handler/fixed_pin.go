package handler

import (
	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type UpsertFixedPinRequest struct {
	Content     types.KnowledgeContent     `json:"content" binding:"required"`
	ContentType types.KnowledgeContentType `json:"content_type"`
}

func (s *HttpSrv) GetFixedPin(c *gin.Context) {
	spaceID, _ := v1.InjectSpaceID(c)
	data, err := v1.NewFixedPinLogic(c, s.Core).Get(spaceID)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, data)
}

func (s *HttpSrv) UpsertFixedPin(c *gin.Context) {
	var req UpsertFixedPinRequest
	if err := utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	data, err := v1.NewFixedPinLogic(c, s.Core).Upsert(spaceID, req.Content, req.ContentType)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, data)
}

func (s *HttpSrv) DeleteFixedPin(c *gin.Context) {
	spaceID, _ := v1.InjectSpaceID(c)
	if err := v1.NewFixedPinLogic(c, s.Core).Delete(spaceID); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}
