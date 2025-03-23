package sqlstore

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.UserSpaceStore = NewUserSpaceStore(provider)
	})
}

// UserSpaceStore 用于处理用户与空间关系表的操作
type UserSpaceStore struct {
	CommonFields
}

// NewUserSpaceRelationStore 创建新的 UserSpaceRelationStore 实例
func NewUserSpaceStore(provider SqlProviderAchieve) *UserSpaceStore {
	repo := &UserSpaceStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_USER_SPACE)
	repo.SetAllColumns("user_id", "space_id", "role", "created_at")
	return repo
}

// Create 创建新的用户与空间关系
func (s *UserSpaceStore) Create(ctx context.Context, data types.UserSpace) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("user_id", "space_id", "role", "created_at").
		Values(data.UserID, data.SpaceID, data.Role, data.CreatedAt)

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

// Get 根据ID获取用户与空间关系
func (s *UserSpaceStore) GetUserSpaceRole(ctx context.Context, userID, spaceID string) (*types.UserSpace, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"user_id": userID, "space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.UserSpace
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *UserSpaceStore) GetSpaceChief(ctx context.Context, spaceID string) (*types.UserSpace, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "role": srv.RoleChief})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.UserSpace
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新用户与空间关系
func (s *UserSpaceStore) Update(ctx context.Context, userID, spaceID, role string) error {
	query := sq.Update(s.GetTable()).
		Set("role", role).
		Where(sq.Eq{"user_id": userID, "space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除用户与空间关系
func (s *UserSpaceStore) Delete(ctx context.Context, userID, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"user_id": userID, "space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *UserSpaceStore) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// List 分页获取用户与空间关系记录
func (s *UserSpaceStore) List(ctx context.Context, opts types.ListUserSpaceOptions, page, pageSize uint64) ([]types.UserSpace, error) {
	query := sq.Select(s.GetAllColumnsWithPrefix(s.GetTable())...).From(s.GetTable()).OrderBy("created_at DESC")
	if page != 0 && pageSize != 0 {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}
	opts.Apply(&query)
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.UserSpace
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *UserSpaceStore) Total(ctx context.Context, opts types.ListUserSpaceOptions) (int64, error) {
	query := sq.Select("COUNT(*)").From(s.GetTable())
	opts.Apply(&query)
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

func (s *UserSpaceStore) ListSpaceUsers(ctx context.Context, spaceID string) ([]string, error) {
	query := sq.Select("user_id").From(s.GetTable()).Where(sq.Eq{"space_id": spaceID}).GroupBy("user_id").OrderBy("create_at")
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}
	var res []string
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
