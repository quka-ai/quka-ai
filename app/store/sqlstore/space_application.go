package sqlstore

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc(RegisterKey{}, func(provider *Provider) {
		provider.stores.SpaceApplicationStore = NewSpaceApplicationStore(provider)
	})
}

func NewSpaceApplicationStore(provider SqlProviderAchieve) *SpaceApplicationImpl {
	repo := &SpaceApplicationImpl{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_SPACE_APPLICATION)
	repo.SetAllColumns(
		"id", "space_id", "user_id", "desc", "updated_at", "created_at",
	)
	return repo
}

type SpaceApplicationImpl struct {
	CommonFields
}

// Create 创建新的文章分享链接
func (s *SpaceApplicationImpl) Create(ctx context.Context, data *types.SpaceApplication) error {
	if data.UpdatedAt == 0 {
		data.UpdatedAt = time.Now().Unix()
	}
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "space_id", "user_id", "user_name", "user_email", "desc", "updated_at", "created_at").
		Values(data.ID, data.SpaceID, data.UserID, data.UserName, data.UserEmail, data.Desc, data.UpdatedAt, data.CreatedAt)

	sql, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(sql, args...)
	return err
}

// Get
func (s *SpaceApplicationImpl) Get(ctx context.Context, spaceID, userID string) (*types.SpaceApplication, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"space_id": spaceID, "user_id": userID})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var data types.SpaceApplication
	err = s.GetReplica(ctx).Get(&data, sql, args...)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

// Get
func (s *SpaceApplicationImpl) GetByID(ctx context.Context, id string) (*types.SpaceApplication, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"id": id})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var data types.SpaceApplication
	err = s.GetReplica(ctx).Get(&data, sql, args...)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func (s *SpaceApplicationImpl) List(ctx context.Context, spaceID string, opts types.ListSpaceApplicationOptions, page, pagesize uint64) ([]types.SpaceApplication, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"space_id": spaceID})

	if page != types.NOT_DELETE || pagesize != types.NOT_DELETE {
		query = query.Limit(pagesize).Offset((page - 1) * pagesize)
	}

	opts.Apply(&query)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var data []types.SpaceApplication
	err = s.GetReplica(ctx).Select(&data, sql, args...)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s *SpaceApplicationImpl) Total(ctx context.Context, spaceID string, opts types.ListSpaceApplicationOptions) (int64, error) {
	query := sq.Select("COUNT(*)").
		From(s.GetTable()).
		Where(sq.Eq{"space_id": spaceID})

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, ErrorSqlBuild(err)
	}

	opts.Apply(&query)

	var data int64
	err = s.GetReplica(ctx).Get(&data, sql, args...)
	if err != nil {
		return 0, err
	}

	return data, nil
}

func (s *SpaceApplicationImpl) UpdateStatus(ctx context.Context, id, status string) error {
	query := sq.Update(s.GetTable()).Set("status", status).Where(sq.Eq{"id": id})
	sql, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}
	_, err = s.GetMaster(ctx).Exec(sql, args...)
	return err
}

// Delete
func (s *SpaceApplicationImpl) Delete(ctx context.Context, spaceID, userID string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{"space_id": spaceID, "user_id": userID}).
		PlaceholderFormat(sq.Dollar)

	sql, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(sql, args...)
	return err
}
