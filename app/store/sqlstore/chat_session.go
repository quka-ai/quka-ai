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
		provider.stores.ChatSessionStore = NewChatSessionStore(provider)
	})
}

type ChatSessionStore struct {
	CommonFields
}

func NewChatSessionStore(provider SqlProviderAchieve) *ChatSessionStore {
	repo := &ChatSessionStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_CHAT_SESSION)
	repo.SetAllColumns("id", "space_id", "user_id", "title", "session_type", "status", "created_at", "latest_access_time")
	return repo
}

func (s *ChatSessionStore) Create(ctx context.Context, data types.ChatSession) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}

	if data.LatestAccessTime == 0 {
		data.LatestAccessTime = time.Now().Unix()
	}

	query := sq.Insert(s.GetTable()).
		Columns("id", "space_id", "user_id", "title", "session_type", "status", "created_at", "latest_access_time").
		Values(data.ID, data.SpaceID, data.UserID, data.Title, data.Type, data.Status, data.CreatedAt, data.LatestAccessTime)

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

func (s *ChatSessionStore) UpdateSessionTitle(ctx context.Context, sessionID string, title string) error {
	query := sq.Update(s.GetTable()).Where(sq.Eq{"id": sessionID}).Set("title", title)
	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	if _, err = s.GetMaster(ctx).Exec(queryString, args...); err != nil {
		return err
	}
	return nil
}

func (s *ChatSessionStore) UpdateSessionStatus(ctx context.Context, sessionID string, status types.ChatSessionStatus) error {
	query := sq.Update(s.GetTable()).Where(sq.Eq{"id": sessionID}).Set("status", status)
	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	if _, err = s.GetMaster(ctx).Exec(queryString, args...); err != nil {
		return err
	}
	return nil
}

func (s *ChatSessionStore) GetByUserID(ctx context.Context, userID string) ([]*types.ChatSession, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"user_id": userID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []*types.ChatSession
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *ChatSessionStore) UpdateChatSessionLatestAccessTime(ctx context.Context, spaceID, sessionID string) error {
	query := sq.Update(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "id": sessionID}).Set("latest_access_time", time.Now().Unix())

	queryString, args, err := query.ToSql()
	if err != nil {
		return err
	}

	if _, err = s.GetMaster(ctx).Exec(queryString, args...); err != nil {
		return err
	}
	return nil
}

func (s *ChatSessionStore) GetChatSession(ctx context.Context, spaceID, sessionID string) (*types.ChatSession, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "id": sessionID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.ChatSession
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *ChatSessionStore) Delete(ctx context.Context, spaceID, sessionID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "id": sessionID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	if _, err = s.GetMaster(ctx).Exec(queryString, args...); err != nil {
		return err
	}
	return nil
}

func (s *ChatSessionStore) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	if _, err = s.GetMaster(ctx).Exec(queryString, args...); err != nil {
		return err
	}
	return nil
}

func (s *ChatSessionStore) List(ctx context.Context, spaceID, userID string, page, pageSize uint64) ([]types.ChatSession, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).
		Where(sq.Eq{"space_id": spaceID, "user_id": userID}).
		OrderBy("latest_access_time DESC")

	if page != types.NO_PAGING || pageSize != types.NO_PAGING {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var list []types.ChatSession
	if err = s.GetReplica(ctx).Select(&list, queryString, args...); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *ChatSessionStore) ListBeforeTime(ctx context.Context, t time.Time, page, pageSize uint64) ([]types.ChatSession, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).
		Where(sq.Lt{"created_at": t.Unix()})

	if page != types.NO_PAGING || pageSize != types.NO_PAGING {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var list []types.ChatSession
	if err = s.GetReplica(ctx).Select(&list, queryString, args...); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *ChatSessionStore) Total(ctx context.Context, spaceID, userID string) (int64, error) {
	query := sq.Select("COUNT(*)").From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "user_id": userID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return 0, ErrorSqlBuild(err)
	}

	var total int64
	if err = s.GetReplica(ctx).Get(&total, queryString, args...); err != nil {
		return 0, err
	}
	return total, nil
}
