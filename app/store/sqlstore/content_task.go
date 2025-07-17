package sqlstore

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type ContentTaskImpl struct {
	CommonFields
}

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.ContentTaskStore = NewContentTaskStore(provider)
	})
}

func NewContentTaskStore(provider SqlProviderAchieve) *ContentTaskImpl {
	repo := &ContentTaskImpl{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_CONTENT_TASK)
	repo.SetAllColumns(
		"task_id", "space_id", "resource", "meta_info", "user_id", "file_url", "file_name", "ai_file_id", "step", "task_type", "retry_times", "updated_at", "created_at",
	)
	return repo
}

// Create 创建新的任务记录
func (s *ContentTaskImpl) Create(ctx context.Context, data types.ContentTask) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("task_id", "space_id", "resource", "meta_info", "user_id", "file_url", "file_name", "ai_file_id", "step", "task_type", "retry_times", "updated_at", "created_at").
		Values(data.TaskID, data.SpaceID, data.Resource, data.MetaInfo, data.UserID, data.FileURL, data.FileName, data.AIFileID, data.Step, data.TaskType, data.RetryTimes, data.UpdatedAt, data.CreatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	if err != nil {
		return err
	}
	return nil
}

// GetTask 根据 task_id 获取任务记录
func (s *ContentTaskImpl) GetTask(ctx context.Context, taskID string) (*types.ContentTask, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"task_id": taskID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.ContentTask
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *ContentTaskImpl) UpdateStep(ctx context.Context, taskID string, step int) error {
	query := sq.Update(s.GetTable()).
		Set("step", step).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"task_id": taskID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Update 更新任务记录
func (s *ContentTaskImpl) Update(ctx context.Context, taskID string, data types.ContentTask) error {
	query := sq.Update(s.GetTable()).
		Set("space_id", data.SpaceID).
		Set("user_id", data.UserID).
		Set("file_url", data.FileURL).
		Set("file_name", data.FileName).
		Set("step", data.Step).
		Set("task_type", data.TaskType).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"task_id": taskID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Update 更新任务记录
func (s *ContentTaskImpl) UpdateAIFileID(ctx context.Context, taskID, aiFileID string) error {
	query := sq.Update(s.GetTable()).
		Set("ai_file_id", aiFileID).
		Set("step", types.LONG_CONTENT_STEP_CREATE_CHUNK).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"task_id": taskID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除任务记录
func (s *ContentTaskImpl) Delete(ctx context.Context, taskID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"task_id": taskID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *ContentTaskImpl) ListTasksStatus(ctx context.Context, taskIDs []string) ([]types.TaskStatus, error) {
	query := sq.Select("task_id", "step", "retry_times", "updated_at").From(s.GetTable()).Where(sq.Eq{"task_id": taskIDs})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.TaskStatus
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

// ListTasks 分页获取任务记录列表
func (s *ContentTaskImpl) ListTasks(ctx context.Context, spaceID string, page, pageSize uint64) ([]types.ContentTask, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID}).OrderBy("created_at DESC")
	if page != types.NO_PAGINATION || pageSize != types.NO_PAGINATION {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.ContentTask
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *ContentTaskImpl) Total(ctx context.Context, spaceID string) (int64, error) {
	query := sq.Select("COUNT(*)").From(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return 0, ErrorSqlBuild(err)
	}

	var res int64
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return 0, err
	}
	return res, nil
}

// ListTasks 分页获取任务记录列表
func (s *ContentTaskImpl) ListUnprocessedTasks(ctx context.Context, page, pageSize uint64) ([]*types.ContentTask, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.NotEq{"step": types.LONG_CONTENT_STEP_FINISHED}).Where(sq.Lt{"retry_times": 3}).OrderBy("created_at ASC")
	if page != types.NO_PAGINATION || pageSize != types.NO_PAGINATION {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []*types.ContentTask
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

// Delete 删除任务记录
func (s *ContentTaskImpl) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *ContentTaskImpl) SetRetryTimes(ctx context.Context, id string, retryTimes int) error {
	query := sq.Update(s.GetTable()).
		Set("retry_times", retryTimes).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"task_id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}
