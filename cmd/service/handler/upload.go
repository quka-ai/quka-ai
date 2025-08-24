package handler

import (
	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type GenUploadKeyRequest struct {
	ObjectType string `json:"object_type" binding:"required"`
	Kind       string `json:"kind" binding:"required"`
	FileName   string `json:"file_name" binding:"required"`
	Size       int64  `json:"size" binding:"required"`
}

type GenUploadKeyResponse struct {
	v1.UploadKey
	URL string `json:"url"`
}

// GenUploadKey
func (s *HttpSrv) GenUploadKey(c *gin.Context) {
	var (
		err error
		req GenUploadKeyRequest
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	logic := v1.NewUploadLogic(c, s.Core)
	result, err := logic.GenClientUploadKey(req.ObjectType, req.Kind, req.FileName, req.Size)
	if err != nil {
		response.APIError(c, err)
		return
	}

	presignURL, err := s.Core.Plugins.FileStorage().GenGetObjectPreSignURL(result.FullPath)
	if err != nil {
		response.APIError(c, errors.New("GenUploadKey.GenGetObjectPreSignURL", "Failed to generate presigned URL", err))
		return
	}

	response.APISuccess(c, GenUploadKeyResponse{
		UploadKey: result,
		URL:       presignURL,
	})
}
