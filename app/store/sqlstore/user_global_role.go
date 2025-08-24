package sqlstore

import (
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.UserGlobalRoleStore = NewUserGlobalRoleStore(provider)
	})
}

type UserGlobalRoleStore struct {
	CommonFields
}

// NewUserGlobalRoleStore 创建新的UserGlobalRoleStore实例
func NewUserGlobalRoleStore(provider SqlProviderAchieve) *UserGlobalRoleStore {
	repo := &UserGlobalRoleStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_USER_GLOBAL_ROLE)
	repo.SetAllColumns("id", "user_id", "appid", "role", "created_at", "updated_at")
	return repo
}

// Create 创建用户全局角色记录
func (s *UserGlobalRoleStore) Create(ctx context.Context, data types.UserGlobalRole) error {
	query := sq.Insert(s.GetTable()).
		Columns("user_id", "appid", "role", "created_at", "updated_at").
		Values(data.UserID, data.Appid, data.Role, data.CreatedAt, data.UpdatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// GetUserRole 获取用户的全局角色
func (s *UserGlobalRoleStore) GetUserRole(ctx context.Context, appid, userID string) (*types.UserGlobalRole, error) {
	query := sq.Select("*").
		From(s.GetTable()).
		Where(sq.Eq{"appid": appid, "user_id": userID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var role types.UserGlobalRole
	err = s.GetReplica(ctx).Get(&role, queryString, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &role, nil
}

// UpdateUserRole 更新用户的全局角色
func (s *UserGlobalRoleStore) UpdateUserRole(ctx context.Context, appid, userID, role string) error {
	query := sq.Update(s.GetTable()).
		Set("role", role).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"appid": appid, "user_id": userID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除用户的全局角色记录
func (s *UserGlobalRoleStore) Delete(ctx context.Context, appid, userID string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{"appid": appid, "user_id": userID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// ListUsersByRole 按角色列出用户
func (s *UserGlobalRoleStore) ListUsersByRole(ctx context.Context, opts types.ListUserGlobalRoleOptions, page, pageSize uint64) ([]types.UserGlobalRole, error) {
	query := sq.Select("*").From(s.GetTable())

	opts.Apply(&query)

	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	query = query.OrderBy("created_at DESC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var roles []types.UserGlobalRole
	err = s.GetReplica(ctx).Select(&roles, queryString, args...)
	if err != nil {
		return nil, err
	}

	return roles, nil
}

// Total 获取符合条件的用户总数
func (s *UserGlobalRoleStore) Total(ctx context.Context, opts types.ListUserGlobalRoleOptions) (int64, error) {
	query := sq.Select("COUNT(*)").From(s.GetTable())

	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return 0, ErrorSqlBuild(err)
	}

	var count int64
	err = s.GetReplica(ctx).Get(&count, queryString, args...)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// ListUserIDsByRole 根据角色获取用户ID列表
func (s *UserGlobalRoleStore) ListUserIDsByRole(ctx context.Context, appid, role string) ([]string, error) {
	query := sq.Select("user_id").
		From(s.GetTable()).
		Where(sq.Eq{"appid": appid, "role": role})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var userIDs []string
	err = s.GetReplica(ctx).Select(&userIDs, queryString, args...)
	if err != nil {
		return nil, err
	}

	return userIDs, nil
}

// DeleteAll 删除指定租户下的所有用户全局角色记录
func (s *UserGlobalRoleStore) DeleteAll(ctx context.Context, appid string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{"appid": appid})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}
