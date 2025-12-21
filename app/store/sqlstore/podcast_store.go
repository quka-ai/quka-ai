package sqlstore

import (
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.PodcastStore = NewPodcastStore(provider)
	})
}

// PodcastStore 处理播客表的操作
type PodcastStore struct {
	CommonFields
}

// NewPodcastStore 创建新的 PodcastStore 实例
func NewPodcastStore(provider SqlProviderAchieve) *PodcastStore {
	store := &PodcastStore{}
	store.SetProvider(provider)
	store.SetTable(types.TABLE_PODCASTS)
	store.SetAllColumns(
		"id", "user_id", "space_id",
		"source_type", "source_id",
		"title", "description", "tags",
		"audio_url", "audio_duration", "audio_size", "audio_format",
		"tts_provider", "tts_model",
		"status", "error_message", "retry_times", "generation_last_updated",
		"created_at", "updated_at", "generated_at",
	)
	return store
}

// Create 创建新的播客
func (s *PodcastStore) Create(ctx context.Context, data *types.Podcast) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = time.Now().Unix()
	}

	query := sq.Insert(s.GetTable()).
		Columns(
			"id", "user_id", "space_id",
			"source_type", "source_id",
			"title", "description", "tags",
			"audio_url", "audio_duration", "audio_size", "audio_format",
			"tts_provider", "tts_model",
			"status", "error_message", "retry_times", "generation_last_updated",
			"created_at", "updated_at", "generated_at",
		).
		Values(
			data.ID, data.UserID, data.SpaceID,
			data.SourceType, data.SourceID,
			data.Title, data.Description, pq.Array(data.Tags),
			data.AudioURL, data.AudioDuration, data.AudioSize, data.AudioFormat,
			data.TTSProvider, data.TTSModel,
			data.Status, data.ErrorMessage, data.RetryTimes, data.GenerationLastUpdated,
			data.CreatedAt, data.UpdatedAt, data.GeneratedAt,
		)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Get 根据ID获取播客
func (s *PodcastStore) Get(ctx context.Context, id string) (*types.Podcast, error) {
	var podcast types.Podcast
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&podcast, queryString, args...)
	if err != nil {
		return nil, err
	}

	return &podcast, nil
}

// GetBySource 根据源类型和源ID获取播客
func (s *PodcastStore) GetBySource(ctx context.Context, sourceType types.PodcastSourceType, sourceID string) (*types.Podcast, error) {
	var podcast types.Podcast
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"source_type": sourceType, "source_id": sourceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&podcast, queryString, args...)
	if err != nil {
		return nil, err
	}

	return &podcast, nil
}

// List 获取播客列表
func (s *PodcastStore) List(ctx context.Context, spaceID string, req *types.ListPodcastsRequest) ([]*types.Podcast, int64, error) {
	var podcasts []*types.Podcast

	// 构建查询条件
	where := sq.And{sq.Eq{"space_id": spaceID}}
	if req.SourceType != "" {
		where = append(where, sq.Eq{"source_type": req.SourceType})
	}
	if req.Status != "" {
		where = append(where, sq.Eq{"status": req.Status})
	}

	// 查询总数
	countQuery := sq.Select("COUNT(*)").
		From(s.GetTable()).
		Where(where)

	countQueryString, countArgs, err := countQuery.ToSql()
	if err != nil {
		return nil, 0, ErrorSqlBuild(err)
	}

	var total int64
	err = s.GetReplica(ctx).Get(&total, countQueryString, countArgs...)
	if err != nil {
		return nil, 0, err
	}

	// 查询数据
	offset := (req.Page - 1) * req.PageSize
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(where).
		OrderBy("created_at DESC").
		Limit(uint64(req.PageSize)).
		Offset(uint64(offset))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, 0, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Select(&podcasts, queryString, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*types.Podcast{}, total, nil
		}
		return nil, 0, err
	}

	return podcasts, total, nil
}

// Update 更新播客
func (s *PodcastStore) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// 自动更新 updated_at
	updates["updated_at"] = time.Now().Unix()

	query := sq.Update(s.GetTable()).
		Where(sq.Eq{"id": id})

	for key, value := range updates {
		query = query.Set(key, value)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// UpdateStatus 更新播客状态
func (s *PodcastStore) UpdateStatus(ctx context.Context, id string, status types.PodcastStatus, errorMessage string) error {
	updates := map[string]interface{}{
		"status":        status,
		"error_message": errorMessage,
	}

	if status == types.PODCAST_STATUS_COMPLETED {
		updates["generated_at"] = time.Now().Unix()
	} else if status == types.PODCAST_STATUS_PROCESSING {
		// 初始化进度时间戳
		updates["generation_last_updated"] = time.Now().Unix()
	}

	return s.Update(ctx, id, updates)
}

// UpdateGenerationProgress 更新播客生成进度时间戳
func (s *PodcastStore) UpdateGenerationProgress(ctx context.Context, id string) error {
	updates := map[string]interface{}{
		"generation_last_updated": time.Now().Unix(),
	}

	return s.Update(ctx, id, updates)
}

// IncrementRetry 增加重试次数
func (s *PodcastStore) IncrementRetry(ctx context.Context, id string) error {
	query := sq.Update(s.GetTable()).
		Set("retry_times", sq.Expr("retry_times + 1")).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除播客
func (s *PodcastStore) Delete(ctx context.Context, id string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Exists 检查播客是否存在
func (s *PodcastStore) Exists(ctx context.Context, id string) (bool, error) {
	var count int
	query := sq.Select("COUNT(*)").
		From(s.GetTable()).
		Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return false, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&count, queryString, args...)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// ExistsBySource 检查源内容是否已创建播客
func (s *PodcastStore) ExistsBySource(ctx context.Context, sourceType types.PodcastSourceType, sourceID string) (bool, error) {
	var count int
	query := sq.Select("COUNT(*)").
		From(s.GetTable()).
		Where(sq.Eq{"source_type": sourceType, "source_id": sourceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return false, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Get(&count, queryString, args...)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// ListPendingTasks 获取待处理的播客任务
func (s *PodcastStore) ListPendingTasks(ctx context.Context, limit int) ([]*types.Podcast, error) {
	var podcasts []*types.Podcast
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"status": types.PODCAST_STATUS_PENDING}).
		OrderBy("created_at ASC").
		Limit(uint64(limit))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Select(&podcasts, queryString, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*types.Podcast{}, nil
		}
		return nil, err
	}

	return podcasts, nil
}

// ListFailedTasksForRetry 获取需要重试的失败任务
func (s *PodcastStore) ListFailedTasksForRetry(ctx context.Context, maxRetries int, limit int) ([]*types.Podcast, error) {
	var podcasts []*types.Podcast
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"status": types.PODCAST_STATUS_FAILED}).
		Where(sq.Lt{"retry_times": maxRetries}).
		OrderBy("updated_at ASC").
		Limit(uint64(limit))

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	err = s.GetReplica(ctx).Select(&podcasts, queryString, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*types.Podcast{}, nil
		}
		return nil, err
	}

	return podcasts, nil
}
