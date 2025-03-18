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

// UserStore 处理bw_user表的操作
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

// GetUser 根据ID获取用户
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

// Update 更新用户信息
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

// Update 更新用户密码
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

// Update 更新用户计划
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
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable())
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
