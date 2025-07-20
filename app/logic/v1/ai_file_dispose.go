package v1

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"time"

	"github.com/samber/lo"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type AIFileDisposeLogic struct {
	UserInfo
	ctx  context.Context
	core *core.Core
}

func NewAIFileDisposeLogic(ctx context.Context, core *core.Core) *AIFileDisposeLogic {
	l := &AIFileDisposeLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}

	return l
}

func (l *AIFileDisposeLogic) GetTasksStatus(taskIDs []string) ([]types.TaskStatus, error) {
	res, err := l.core.Store().ContentTaskStore().ListTasksStatus(l.ctx, taskIDs)
	if err != nil {
		return nil, errors.New("AIFileDisposeLogic.GetTasksStatus.ContentTaskStore.ListTasksStatus", i18n.ERROR_INTERNAL, err)
	}

	return lo.Map(res, func(item types.TaskStatus, _ int) types.TaskStatus {
		return types.TaskStatus{
			TaskID:     item.TaskID,
			Status:     convertTaskStepToStatus(item.Status, item.RetryTimes),
			RetryTimes: item.RetryTimes,
			UpdatedAt:  item.UpdatedAt,
		}
	}), nil
}

func (l *AIFileDisposeLogic) DeleteTask(taskID string) error {
	task, err := l.core.Store().ContentTaskStore().GetTask(l.ctx, taskID)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("AIFileDisposeLogic.DeleteTask.ContentTaskStore.GetTask", i18n.ERROR_INTERNAL, err)
	}

	if task == nil {
		return nil
	}

	return l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		if err := l.core.Store().ContentTaskStore().Delete(ctx, taskID); err != nil {
			return errors.New("AIFileDisposeLogic.DeleteTask.ContentTaskStore.Delete", i18n.ERROR_INTERNAL, err)
		}

		if err = l.core.Store().FileManagementStore().UpdateStatus(ctx, task.SpaceID, []string{task.FileURL}, types.FILE_UPLOAD_STATUS_NEED_TO_DELETE); err != nil {
			return errors.New("AIFileDisposeLogic.DeleteTask.FileManagementStore.UpdateStatus", i18n.ERROR_INTERNAL, err)
		}
		return nil
	})
}

func (l *AIFileDisposeLogic) CreateLongContentTask(spaceID, resource, meta, fileName, fileURL string) error {
	taskID := utils.GenUniqIDStr()

	parsedUrl, err := url.Parse(fileURL)
	if err != nil {
		return errors.New("AIFileDisposeLogic.CreateLongContentTask.url.Parse", i18n.ERROR_INVALIDARGUMENT, err).Code(http.StatusBadRequest)
	}

	if meta == "" {
		meta = fileName
	}

	err = l.core.Store().ContentTaskStore().Create(l.ctx, types.ContentTask{
		TaskID:    taskID,
		SpaceID:   spaceID,
		Resource:  resource,
		MetaInfo:  meta,
		UserID:    l.GetUserInfo().User,
		FileURL:   parsedUrl.RequestURI(),
		FileName:  fileName,
		Step:      types.LONG_CONTENT_STEP_CREATE_CHUNK,
		TaskType:  "chunk",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	})
	if err != nil {
		return errors.New("AIFileDisposeLogic.CreateLongContentTask.ContentTaskStore.Create", i18n.ERROR_INTERNAL, err)
	}

	{
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := l.core.Store().FileManagementStore().UpdateStatus(ctx, spaceID, []string{parsedUrl.RequestURI()}, types.FILE_UPLOAD_STATUS_UPLOADED); err != nil {
			return errors.New("UpdateFilesUploaded.FileManagementStore.UpdateStatus", i18n.ERROR_INTERNAL, err)
		}
	}

	// async dispose
	return nil
}

func (l *AIFileDisposeLogic) GetLongContentTaskInfo(taskID string) (*types.ContentTask, error) {
	task, err := l.core.Store().ContentTaskStore().GetTask(l.ctx, taskID)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.New("AIFileDisposeLogic.GetLongContentTaskInfo.ContentTaskStore.GetTask", i18n.ERROR_INTERNAL, err)
	}

	if task == nil {
		return nil, errors.New("AIFileDisposeLogic.GetLongContentTaskInfo.ContentTaskStore.GetTask.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	return task, nil
}

type ChunkTaskDetail struct {
	TaskID        string `json:"task_id" db:"task_id"`   // 任务ID，32字符字符串类型，唯一标识任务
	SpaceID       string `json:"space_id" db:"space_id"` // 空间ID，标识任务归属的空间
	UserID        string `json:"user_id" db:"user_id"`   // 用户ID，标识发起任务的用户
	Resource      string `json:"resource" db:"resource"` // 资源类型
	ResourceTitle string `json:"resource_title"`
	FileURL       string `json:"file_url" db:"file_url"`   // 文件URL，任务需要处理的文件路径
	FileName      string `json:"file_name" db:"file_name"` // 文件名，任务需要处理的文件名称
	UserName      string `json:"user_name"`
	UserAvatar    string `json:"user_avatar"`
	UserEmail     string `json:"user_email"`
	Status        int    `json:"status" db:"status"`
	TaskType      string `json:"task_type" db:"task_type"`   // 任务类型，表示任务的目的或用途
	CreatedAt     int64  `json:"created_at" db:"created_at"` // 任务创建时间，时间戳格式
	UpdatedAt     int64  `json:"updated_at" db:"updated_at"` // 任务创建时间，时间戳格式
	RetryTimes    int    `json:"retry_times" db:"retry_times"`
}

func (l *AIFileDisposeLogic) GetLongContentTaskList(spaceID string, page, pagesize uint64) ([]ChunkTaskDetail, int64, error) {
	list, err := l.core.Store().ContentTaskStore().ListTasks(l.ctx, spaceID, page, pagesize)
	if err != nil && err != sql.ErrNoRows {
		return nil, 0, errors.New("AIFileDisposeLogic.GetLongContentTaskList.ContentTaskStore.ListTasks", i18n.ERROR_INTERNAL, err)
	}

	total, err := l.core.Store().ContentTaskStore().Total(l.ctx, spaceID)
	if err != nil {
		return nil, 0, errors.New("AIFileDisposeLogic.GetLongContentTaskList.ContentTaskStore.Total", i18n.ERROR_INTERNAL, err)
	}

	userIDs := lo.Map(list, func(item types.ContentTask, _ int) string {
		return item.UserID
	})
	userIDs = lo.Union(userIDs)

	user, err := l.core.Store().UserStore().ListUsers(l.ctx, types.ListUserOptions{
		IDs: userIDs,
	}, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		return nil, 0, errors.New("AIFileDisposeLogic.GetLongContentTaskList.UserStore.ListUsers", i18n.ERROR_INTERNAL, err)
	}

	userMap := lo.SliceToMap(user, func(item types.User) (string, types.User) {
		return item.ID, item
	})

	spaceResources, err := l.core.Store().ResourceStore().ListResources(l.ctx, spaceID, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		return nil, 0, errors.New("AIFileDisposeLogic.GetLongContentTaskList.ResourceStore.ListResources", i18n.ERROR_INTERNAL, err)
	}

	resourceMap := lo.SliceToMap(spaceResources, func(item types.Resource) (string, types.Resource) {
		return item.ID, item
	})

	results := lo.Map(list, func(item types.ContentTask, _ int) ChunkTaskDetail {
		user, exist := userMap[item.UserID]
		if !exist {
			user = types.User{}
		}
		resource, exist := resourceMap[item.Resource]
		if !exist {
			resource = types.Resource{}
		}
		return ChunkTaskDetail{
			TaskID:        item.TaskID,
			SpaceID:       item.SpaceID,
			UserID:        item.UserID,
			FileURL:       item.FileURL,
			FileName:      item.FileName,
			Resource:      item.Resource,
			ResourceTitle: lo.If(resource.Title != "", resource.Title).Else(item.Resource),
			UserName:      user.Name,
			UserAvatar:    user.Avatar,
			UserEmail:     user.Email,
			Status:        convertTaskStepToStatus(item.Step, item.RetryTimes),
			RetryTimes:    item.RetryTimes,
			TaskType:      item.TaskType,
			CreatedAt:     item.CreatedAt,
			UpdatedAt:     item.UpdatedAt,
		}
	})

	return results, total, nil
}

func convertTaskStepToStatus(step, retryTimes int) int {
	return lo.If(step == types.LONG_CONTENT_STEP_FINISHED, types.TASK_STATUS_FINISHED).ElseIf(retryTimes == 3, types.TASK_STATUS_FAILED).Else(types.TASK_STATUS_IN_PROGRESS)
}
