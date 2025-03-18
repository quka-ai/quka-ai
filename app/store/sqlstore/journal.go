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
		provider.stores.JournalStore = NewJournalStore(provider)
	})
}

type JournalStore struct {
	CommonFields
}

// NewJournal
func NewJournalStore(provider SqlProviderAchieve) *JournalStore {
	repo := &JournalStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_JOURNAL)
	repo.SetAllColumns("id", "space_id", "user_id", "date", "content", "updated_at", "created_at")
	return repo
}

// Create
func (s *JournalStore) Create(ctx context.Context, data types.Journal) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("id", "space_id", "user_id", "date", "content", "updated_at", "created_at").
		Values(data.ID, data.SpaceID, data.UserID, data.Date, data.Content, data.UpdatedAt, data.CreatedAt)

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

// Get
func (s *JournalStore) Get(ctx context.Context, spaceID, userID, date string) (*types.Journal, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "user_id": userID, "date": date})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.Journal
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *JournalStore) Exist(ctx context.Context, spaceID, userID, date string) (bool, error) {
	query := sq.Select("1").From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "user_id": userID, "date": date})

	queryString, args, err := query.ToSql()
	if err != nil {
		return false, ErrorSqlBuild(err)
	}

	var exist bool
	if err = s.GetReplica(ctx).Get(&exist, queryString, args...); err != nil {
		return false, err
	}
	return exist, nil
}

func (s *JournalStore) Update(ctx context.Context, id int64, content types.KnowledgeContent) error {
	query := sq.Update(s.GetTable()).SetMap(map[string]interface{}{
		"content":    content.String(),
		"updated_at": time.Now().Unix(),
	}).Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	if _, err = s.GetMaster(ctx).Exec(queryString, args...); err != nil {
		return err
	}
	return nil
}

// Delete
func (s *JournalStore) Delete(ctx context.Context, id int64) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// List
func (s *JournalStore) List(ctx context.Context, spaceID, userID string, page, pageSize uint64) ([]types.Journal, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "user_id": userID}).
		Limit(pageSize).Offset((page - 1) * pageSize).OrderBy("date DESC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.Journal
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *JournalStore) ListWithDate(ctx context.Context, spaceID, userID, startDate, endDate string) ([]types.Journal, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.And{sq.Eq{"space_id": spaceID, "user_id": userID},
		sq.GtOrEq{"date": startDate}, sq.LtOrEq{"date": endDate}}).OrderBy("date DESC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.Journal
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *JournalStore) DeleteByDate(ctx context.Context, date string) error {
	query := sq.Delete(s.GetTable()).Where(sq.LtOrEq{"date": date})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}
