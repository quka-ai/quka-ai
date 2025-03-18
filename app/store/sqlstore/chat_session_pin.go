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
		provider.stores.ChatSessionPinStore = NewChatSessionPinStore(provider)
	})
}

// ChatSessionContentStore 处理会话内容表的操作
type ChatSessionPinStore struct {
	CommonFields
}

// NewChatSessionContentStore 创建新的实例
func NewChatSessionPinStore(provider SqlProviderAchieve) *ChatSessionPinStore {
	repo := &ChatSessionPinStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_CHAT_SESSION_PIN)
	repo.SetAllColumns("session_id", "space_id", "user_id", "content", "version", "created_at", "updated_at")
	return repo
}

// Create 新增会话内容记录
func (s *ChatSessionPinStore) Create(ctx context.Context, data types.ChatSessionPin) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	if data.UpdatedAt == 0 {
		data.UpdatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("session_id", "space_id", "user_id", "content", "version", "created_at", "updated_at").
		Values(data.SessionID, data.SpaceID, data.UserID, data.Content, data.Version, data.CreatedAt, data.UpdatedAt)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// GetBySessionID 根据 session_id 获取记录
func (s *ChatSessionPinStore) GetBySessionID(ctx context.Context, sessionID string) (*types.ChatSessionPin, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"session_id": sessionID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.ChatSessionPin
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新会话内容记录
func (s *ChatSessionPinStore) Update(ctx context.Context, spaceID, sessionID string, content types.RawMessage, version string) error {
	query := sq.Update(s.GetTable()).
		Set("content", content).
		Set("version", version).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{
			"session_id": sessionID,
			"space_id":   spaceID,
		})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除会话内容记录
func (s *ChatSessionPinStore) Delete(ctx context.Context, spaceID, sessionID string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{
			"session_id": sessionID,
			"space_id":   spaceID,
		})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *ChatSessionPinStore) DeleteAll(ctx context.Context, spaceID string) error {
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

// List 分页获取会话内容记录列表
func (s *ChatSessionPinStore) List(ctx context.Context, page, pageSize uint64) ([]types.ChatSessionPin, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Limit(pageSize).
		Offset((page - 1) * pageSize)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.ChatSessionPin
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
