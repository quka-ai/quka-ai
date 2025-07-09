package handler

import (
	"bytes"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
)

type S3ProxyRequest struct {
	ObjectPath string `uri:"object_path" binding:"required"`
}

type S3ProxyResponse struct {
	URL         string            `json:"url,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

func (s *HttpSrv) ObjectHandler(c *gin.Context) {
	var req S3ProxyRequest
	
	if err := c.ShouldBindUri(&req); err != nil {
		response.APIError(c, err)
		return
	}

	decodedPath, err := url.QueryUnescape(req.ObjectPath)
	if err != nil {
		response.APIError(c, errors.New("ObjectHandler.QueryUnescape", i18n.ERROR_INVALIDARGUMENT, err).Code(http.StatusBadRequest))
		return
	}

	logic := v1.NewObjectLogic(c, s.Core)
	
	if logic.IsPublicResource(decodedPath) {
		hasPermission, err := logic.CheckObjectPermission("", "", decodedPath)
		if err != nil {
			response.APIError(c, err)
			return
		}
		
		if !hasPermission {
			response.APIError(c, errors.New("ObjectHandler.CheckObjectPermission", i18n.ERROR_FORBIDDEN, nil).Code(http.StatusForbidden))
			return
		}
	} else {
		claims, ok := v1.InjectTokenClaim(c)
		if !ok {
			response.APIError(c, errors.New("ObjectHandler.InjectTokenClaim", i18n.ERROR_UNAUTHORIZED, nil).Code(http.StatusUnauthorized))
			return
		}

		spaceID := ""
		if spaceIDFromPath, ok := v1.InjectSpaceID(c); ok {
			spaceID = spaceIDFromPath
		} else if spaceIDFromQuery := c.Query("space_id"); spaceIDFromQuery != "" {
			spaceID = spaceIDFromQuery
		}

		if spaceID == "" {
			response.APIError(c, errors.New("ObjectHandler.MissingSpaceID", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest))
			return
		}

		hasPermission, err := logic.CheckObjectPermission(claims.User, spaceID, decodedPath)
		if err != nil {
			response.APIError(c, err)
			return
		}
		
		if !hasPermission {
			response.APIError(c, errors.New("ObjectHandler.CheckObjectPermission", i18n.ERROR_FORBIDDEN, nil).Code(http.StatusForbidden))
			return
		}
	}

	fileStorage := s.Core.Plugins.FileStorage()

	if c.Query("presigned") == "true" {
		presignedURL, err := fileStorage.GenGetObjectPreSignURL(decodedPath)
		if err != nil {
			response.APIError(c, err)
			return
		}
		
		response.APISuccess(c, S3ProxyResponse{
			URL: presignedURL,
		})
		return
	}

	objectResult, err := fileStorage.DownloadFile(c.Request.Context(), decodedPath)
	if err != nil {
		response.APIError(c, err)
		return
	}

	contentType := objectResult.FileType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.Itoa(len(objectResult.File)))

	isImageRequest := strings.HasPrefix(c.Request.URL.Path, "/image/")
	
	if logic.IsPublicResource(decodedPath) || isImageRequest {
		c.Header("Cache-Control", "public, max-age=3600")
		filename := extractFilename(decodedPath)
		if filename != "" {
			c.Header("Content-Disposition", `inline; filename="`+filename+`"`)
		}
	} else {
		filename := extractFilename(decodedPath)
		if filename != "" {
			c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
		}
	}

	c.DataFromReader(http.StatusOK, int64(len(objectResult.File)), contentType, bytes.NewReader(objectResult.File), nil)
}


func extractFilename(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
