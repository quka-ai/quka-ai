package sqlstore

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.ChatMessageExtStore = NewChatMessageExtStore(provider)
	})
}

// ChatMessageExtStore 处理 ChatMessageExt 表的 CRUD 操作
type ChatMessageExtStore struct {
	CommonFields // 包含基本的数据库操作字段和方法
}

// NewChatMessageExtStore 创建 ChatMessageExtStore 实例
func NewChatMessageExtStore(provider SqlProviderAchieve) *ChatMessageExtStore {
	store := &ChatMessageExtStore{}
	store.SetProvider(provider)
	store.SetTable(types.TABLE_CHAT_MESSAGE_EXT)
	store.SetAllColumns("message_id", "space_id", "session_id", "evaluate", "generation_status", "rel_docs", "created_at", "updated_at")
	return store
}

// Create 创建新的 ChatMessageExt 记录
func (s *ChatMessageExtStore) Create(ctx context.Context, data types.ChatMessageExt) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}

	if data.UpdatedAt == 0 {
		data.UpdatedAt = time.Now().Unix()
	}

	query := sq.Insert(s.GetTable()).
		Columns("message_id", "space_id", "session_id", "evaluate", "generation_status", "rel_docs", "created_at", "updated_at").
		Values(data.MessageID, data.SpaceID, data.SessionID, data.Evaluate, data.GenerationStatus, pq.Array(data.RelDocs), data.CreatedAt, data.UpdatedAt)

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

// GetChatMessageExt 根据ID获取 ChatMessageExt 记录
func (s *ChatMessageExtStore) GetChatMessageExt(ctx context.Context, spaceID, sessionID, messageID string) (*types.ChatMessageExt, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "session_id": sessionID, "message_id": messageID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.ChatMessageExt
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新 ChatMessageExt 记录
func (s *ChatMessageExtStore) Update(ctx context.Context, messageID string, data types.ChatMessageExt) error {
	query := sq.Update(s.GetTable()).
		Set("session_id", data.SessionID).
		Set("evaluate", data.Evaluate).
		Set("generation_status", data.GenerationStatus).
		Set("rel_docs", data.RelDocs).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"message_id": messageID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除 ChatMessageExt 记录
func (s *ChatMessageExtStore) Delete(ctx context.Context, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *ChatMessageExtStore) DeleteSessionMessageExt(ctx context.Context, spaceID, sessionID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "session_id": sessionID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *ChatMessageExtStore) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// ListChatMessageExts 分页获取 ChatMessageExt 记录列表
func (s *ChatMessageExtStore) ListChatMessageExts(ctx context.Context, messageIDs []string) ([]types.ChatMessageExt, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"message_id": messageIDs})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.ChatMessageExt
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
