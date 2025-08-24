package handler

import (
	"github.com/gin-gonic/gin"

	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/app/response"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type CreateFileChunkTaskRequest struct {
	FileName string `json:"file_name" binding:"required"`
	FileUrl  string `json:"file_url" binding:"required"`
	Resource string `json:"resource" binding:"required"`
	MetaInfo string `json:"meta_info"`
}

func (s *HttpSrv) CreateFileChunkTask(c *gin.Context) {
	var (
		err error
		req CreateFileChunkTaskRequest
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	if err = v1.NewAIFileDisposeLogic(c, s.Core).CreateLongContentTask(spaceID, req.Resource, req.MetaInfo, req.FileName, req.FileUrl); err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, nil)
}

type GetFileChunkTaskListRequest struct {
	Page     uint64 `json:"page" form:"page" binding:"required"`
	Pagesize uint64 `json:"pagesize" form:"pagesize" binding:"required"`
}

type GetFileChunkTaskListResponse struct {
	List  []v1.ChunkTaskDetail `json:"list"`
	Total int64                `json:"total"`
}

func (s *HttpSrv) GetFileChunkTaskList(c *gin.Context) {
	var (
		err error
		req GetFileChunkTaskListRequest
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	spaceID, _ := v1.InjectSpaceID(c)
	list, total, err := v1.NewAIFileDisposeLogic(c, s.Core).GetLongContentTaskList(spaceID, req.Page, req.Pagesize)
	if err != nil {
		response.APIError(c, err)
		return
	}

	response.APISuccess(c, GetFileChunkTaskListResponse{
		List:  list,
		Total: total,
	})
}

type DeleteChunkTaskRequest struct {
	TaskID string `json:"task_id" binding:"required"`
}

func (s *HttpSrv) DeleteChunkTask(c *gin.Context) {
	var (
		err error
		req DeleteChunkTaskRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	if err = v1.NewAIFileDisposeLogic(c, s.Core).DeleteTask(req.TaskID); err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, nil)
}

type GetTaskStatusRequest struct {
	TaskIDs []string `json:"task_ids" form:"task_ids[]" binding:"required"`
}

func (s *HttpSrv) GetTaskStatus(c *gin.Context) {
	var (
		err error
		req GetTaskStatusRequest
	)
	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	res, err := v1.NewAIFileDisposeLogic(c, s.Core).GetTasksStatus(req.TaskIDs)
	if err != nil {
		response.APIError(c, err)
		return
	}
	response.APISuccess(c, res)
}
