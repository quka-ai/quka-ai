package sqlstore

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.AITokenUsageStore = NewAITokenUsageStore(provider)
	})
}

type AITokenUsageStore struct {
	CommonFields
}

// NewAITokenUsageStore 创建新的 AITokenUsageStore 实例
func NewAITokenUsageStore(provider SqlProviderAchieve) *AITokenUsageStore {
	repo := &AITokenUsageStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_AI_TOKEN_USAGE)
	repo.SetAllColumns("space_id", "user_id", "type", "sub_type", "model", "object_id", "usage_prompt", "usage_output", "created_at")
	return repo
}

// Create 新增一条 AI Token 使用记录
func (s *AITokenUsageStore) Create(ctx context.Context, data types.AITokenUsage) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("space_id", "user_id", "type", "sub_type", "model", "object_id", "usage_prompt", "usage_output", "created_at").
		Values(data.SpaceID, data.UserID, data.Type, data.SubType, data.Model, data.ObjectID, data.UsagePrompt, data.UsageOutput, data.CreatedAt)

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

// Get 根据 ID 获取 AI Token 使用记录
func (s *AITokenUsageStore) Get(ctx context.Context, _type, subType, objectID, userID string) (*types.AITokenUsage, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"object_id": objectID, "user_id": userID})
	if _type != "" {
		query = query.Where(sq.Eq{"type": _type})
	}
	if subType != "" {
		query = query.Where(sq.Eq{"sub_type": subType})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.AITokenUsage
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Delete 删除 AI Token 使用记录
func (s *AITokenUsageStore) Delete(ctx context.Context, spaceID, userID string, st, et time.Time) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "user_id": userID}).
		Where(sq.And{sq.GtOrEq{"created_at": st.Unix()}, sq.LtOrEq{"created_at": et.Unix()}})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// List 分页获取 AI Token 使用记录
func (s *AITokenUsageStore) List(ctx context.Context, spaceID, userID string, page, pageSize uint64) ([]types.AITokenUsage, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "user_id": userID}).
		Limit(pageSize).Offset((page - 1) * pageSize)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.AITokenUsage
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

// ListUserEachModelUsage
func (s *AITokenUsageStore) ListUserEachModelUsage(ctx context.Context, userID string, st, et time.Time) ([]types.AITokenSummary, error) {
	query := sq.Select("sum(usage_prompt) as usage_prompt,sum(usage_output) as usage_output,model").From(s.GetTable()).
		Where(sq.And{sq.Eq{"user_id": userID}, sq.GtOrEq{"created_at": st.Unix()}, sq.LtOrEq{"created_at": et.Unix()}}).GroupBy("model")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.AITokenSummary
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *AITokenUsageStore) SumUserUsageByType(ctx context.Context, userID string, st, et time.Time) ([]types.UserTokenUsageWithType, error) {
	query := sq.Select("SUM(usage_prompt) as usage_prompt", "SUM(usage_output) as usage_output", "type", "sub_type", "user_id").From(s.GetTable()).
		Where(sq.Eq{"user_id": userID}).Where(sq.And{sq.GtOrEq{"created_at": st.Unix()}, sq.LtOrEq{"created_at": et.Unix()}}).GroupBy("type", "sub_type")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.UserTokenUsageWithType
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *AITokenUsageStore) SumUserUsage(ctx context.Context, userID string, st, et time.Time) (types.UserTokenUsage, error) {
	query := sq.Select("SUM(usage_prompt) as usage_prompt", "SUM(usage_output) as usage_output", "user_id").From(s.GetTable()).
		Where(sq.Eq{"user_id": userID}).Where(sq.And{sq.GtOrEq{"created_at": st.Unix()}, sq.LtOrEq{"created_at": et.Unix()}}).
		GroupBy("user_id")

	queryString, args, err := query.ToSql()
	if err != nil {
		return types.UserTokenUsage{}, ErrorSqlBuild(err)
	}

	var res types.UserTokenUsage
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return types.UserTokenUsage{}, err
	}
	return res, nil
}
