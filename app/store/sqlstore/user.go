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
		provider.stores.UserStore = NewUserStore(provider)
	})
}

type UserStore struct {
	CommonFields // CommonFields 是定义在该代码所在包内的，所以可以直接使用
}

// NewUserStore 创建新的UserStore实例
func NewUserStore(provider SqlProviderAchieve) *UserStore {
	repo := &UserStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_USER) // 设置表名
	repo.SetAllColumns("id", "appid", "name", "avatar", "email", "password", "salt", "source", "plan_id", "updated_at", "created_at")
	return repo
}

// Create 创建新的用户
func (s *UserStore) Create(ctx context.Context, data types.User) error {
	query := sq.Insert(s.GetTable()).
		Columns("id", "appid", "name", "avatar", "email", "password", "salt", "source", "plan_id", "updated_at", "created_at").
		Values(data.ID, data.Appid, data.Name, data.Avatar, data.Email, data.Password, data.Salt, data.Source, data.PlanID, data.UpdatedAt, data.CreatedAt)

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

// GetUser 根据ID获取用户
func (s *UserStore) GetUser(ctx context.Context, appid, id string) (*types.User, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"appid": appid, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.User
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// GetByEmail 根据邮箱获取用户
func (s *UserStore) GetByEmail(ctx context.Context, appid, email string) (*types.User, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"appid": appid, "email": email})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.User
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// UpdateUserProfile 更新用户信息
func (s *UserStore) UpdateUserProfile(ctx context.Context, appid, id, userName, email, avatar string) error {
	query := sq.Update(s.GetTable()).
		Set("name", userName).
		Set("email", email).
		Set("avatar", avatar).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"appid": appid, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// UpdateUserPassword 更新用户密码
func (s *UserStore) UpdateUserPassword(ctx context.Context, appid, id, salt, password string) error {
	query := sq.Update(s.GetTable()).
		Set("salt", salt).
		Set("password", password).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"appid": appid, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// UpdateUserPlan 更新用户计划
func (s *UserStore) UpdateUserPlan(ctx context.Context, appid, id, planID string) error {
	query := sq.Update(s.GetTable()).
		Set("plan_id", planID).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"appid": appid, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// BatchUpdateUserPlan 批量更新用户计划
func (s *UserStore) BatchUpdateUserPlan(ctx context.Context, appid string, ids []string, planID string) error {
	query := sq.Update(s.GetTable()).
		Set("plan_id", planID).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"appid": appid, "id": ids})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除用户
func (s *UserStore) Delete(ctx context.Context, appid, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"appid": appid, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// ListUsers 分页获取用户列表
func (s *UserStore) ListUsers(ctx context.Context, opts types.ListUserOptions, page, pageSize uint64) ([]types.User, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).OrderBy("created_at DESC, id DESC")
	if page != 0 || pageSize != 0 {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}

	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.User
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

// Total 获取符合条件的用户总数
func (s *UserStore) Total(ctx context.Context, opts types.ListUserOptions) (int64, error) {
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

// ListUsersWithGlobalRole 获取用户列表，支持全局角色过滤（使用JOIN查询避免SQL长度限制）
func (s *UserStore) ListUsersWithGlobalRole(ctx context.Context, opts types.ListUserOptions, globalRole string, page, pageSize uint64) ([]types.UserWithRole, error) {
	// 基础查询：用户表 + 全局角色表
	query := sq.Select(
		"u.id", "u.appid", "u.name", "u.avatar", "u.email", "u.plan_id", "u.updated_at", "u.created_at",
		"COALESCE(r.role, $1) as global_role",
	).From(s.GetTable() + " u").
		LeftJoin(types.TABLE_USER_GLOBAL_ROLE.Name() + " r ON u.id = r.user_id AND u.appid = r.appid").
		OrderBy("u.created_at DESC, u.id DESC").PlaceholderFormat(sq.Dollar)

	// 应用全局角色过滤
	if globalRole != "" {
		if globalRole == types.DefaultGlobalRole {
			// 对于默认角色，查找没有角色记录的用户
			query = query.Where("r.user_id IS NULL")
		} else {
			// 对于指定角色，查找匹配的角色记录
			query = query.Where("r.role = $2", globalRole)
		}
	}

	// 应用其他过滤条件
	if opts.Appid != "" {
		query = query.Where(sq.Eq{"u.appid": opts.Appid})
	}
	if len(opts.IDs) > 0 {
		query = query.Where(sq.Eq{"u.id": opts.IDs})
	}
	if opts.Email != "" {
		query = query.Where(sq.Eq{"u.email": opts.Email})
	}
	if opts.Name != "" {
		query = query.Where(sq.Like{"u.name": "%" + opts.Name + "%"})
	}
	if opts.EmailLike != "" {
		query = query.Where(sq.Like{"u.email": "%" + opts.EmailLike + "%"})
	}

	// 分页
	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	// 将默认角色值添加到参数中
	args = append([]interface{}{types.DefaultGlobalRole}, args...)

	var res []types.UserWithRole
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}

	return res, nil
}

// TotalWithGlobalRole 获取符合条件的用户总数（支持全局角色过滤）
func (s *UserStore) TotalWithGlobalRole(ctx context.Context, opts types.ListUserOptions, globalRole string) (int64, error) {
	// 基础计数查询：用户表 + 全局角色表
	query := sq.Select("COUNT(u.id)").
		From(s.GetTable() + " u").
		LeftJoin(types.TABLE_USER_GLOBAL_ROLE.Name() + " r ON u.id = r.user_id AND u.appid = r.appid")

	// 应用全局角色过滤
	if globalRole != "" {
		if globalRole == types.DefaultGlobalRole {
			// 对于默认角色，查找没有角色记录的用户
			query = query.Where("r.user_id IS NULL")
		} else {
			// 对于指定角色，查找匹配的角色记录
			query = query.Where("r.role = $1", globalRole)
		}
	}

	// 应用其他过滤条件
	if opts.Appid != "" {
		query = query.Where(sq.Eq{"u.appid": opts.Appid})
	}
	if len(opts.IDs) > 0 {
		query = query.Where(sq.Eq{"u.id": opts.IDs})
	}
	if opts.Email != "" {
		query = query.Where(sq.Eq{"u.email": opts.Email})
	}
	if opts.Name != "" {
		query = query.Where(sq.Like{"u.name": "%" + opts.Name + "%"})
	}
	if opts.EmailLike != "" {
		query = query.Where(sq.Like{"u.email": "%" + opts.EmailLike + "%"})
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return 0, ErrorSqlBuild(err)
	}

	var count int64
	if err = s.GetReplica(ctx).Get(&count, queryString, args...); err != nil {
		return 0, err
	}

	return count, nil
}
