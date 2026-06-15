package sqlstore

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.FixedPinStore = NewFixedPinStore(provider)
	})
}

type FixedPinStore struct {
	CommonFields
}

func NewFixedPinStore(provider SqlProviderAchieve) *FixedPinStore {
	repo := &FixedPinStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_FIXED_PIN)
	repo.SetAllColumns("id", "space_id", "user_id", "content", "content_type", "created_at", "updated_at")
	return repo
}

func (s *FixedPinStore) Create(ctx context.Context, data types.FixedPin) error {
	now := time.Now().Unix()
	if data.ID == "" {
		data.ID = utils.GenRandomID()
	}
	if data.CreatedAt == 0 {
		data.CreatedAt = now
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = now
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "space_id", "user_id", "content", "content_type", "created_at", "updated_at").
		Values(data.ID, data.SpaceID, data.UserID, data.Content.String(), data.ContentType, data.CreatedAt, data.UpdatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *FixedPinStore) Get(ctx context.Context, spaceID, userID string) (*types.FixedPin, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{
			"space_id": spaceID,
			"user_id":  userID,
		})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.FixedPin
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *FixedPinStore) Upsert(ctx context.Context, data types.FixedPin) error {
	now := time.Now().Unix()
	if data.ID == "" {
		data.ID = utils.GenRandomID()
	}
	if data.CreatedAt == 0 {
		data.CreatedAt = now
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = now
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "space_id", "user_id", "content", "content_type", "created_at", "updated_at").
		Values(data.ID, data.SpaceID, data.UserID, data.Content.String(), data.ContentType, data.CreatedAt, data.UpdatedAt).
		Suffix("ON CONFLICT (space_id, user_id) DO UPDATE SET content = EXCLUDED.content, content_type = EXCLUDED.content_type, updated_at = EXCLUDED.updated_at")

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *FixedPinStore) Delete(ctx context.Context, spaceID, userID string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{
			"space_id": spaceID,
			"user_id":  userID,
		})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *FixedPinStore) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{
			"space_id": spaceID,
		})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}
