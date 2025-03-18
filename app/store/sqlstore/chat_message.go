package sqlstore

import (
	"context"
	"encoding/json"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.ChatMessageStore = NewChatMessageStore(provider)
	})
}

type ChatMessageStore struct {
	CommonFields
}

func NewChatMessageStore(provider SqlProviderAchieve) *ChatMessageStore {
	repo := &ChatMessageStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_CHAT_MESSAGE)
	repo.SetAllColumns("id", "space_id", "user_id", "role", "message", "msg_type", "send_time", "session_id", "complete", "sequence", "msg_block", "attach", "is_encrypt")
	return repo
}

func (s *ChatMessageStore) Create(ctx context.Context, data *types.ChatMessage) error {
	if data.SendTime == 0 {
		data.SendTime = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("id", "space_id", "user_id", "role", "message", "msg_type", "send_time", "session_id", "complete", "sequence", "msg_block", "attach", "is_encrypt").
		Values(data.ID, data.SpaceID, data.UserID, data.Role, data.Message, data.MsgType, data.SendTime, data.SessionID, data.Complete, data.Sequence, data.MsgBlock, data.Attach.String(), data.IsEncrypt)

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

func (s *ChatMessageStore) GetOne(ctx context.Context, id string) (*types.ChatMessage, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"id": id})
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var msg types.ChatMessage
	if err := s.GetReplica(ctx).Get(&msg, queryString, args...); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *ChatMessageStore) RewriteMessage(ctx context.Context, spaceID, sessionID, id string, message json.RawMessage, complete int32) error {
	query := sq.Update(s.GetTable()).Set("message", message).Set("complete", complete).Where(sq.Eq{"space_id": spaceID, "session_id": sessionID, "id": id})
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

func (s *ChatMessageStore) SaveEncrypt(ctx context.Context, id string, message json.RawMessage) error {
	query := sq.Update(s.GetTable()).Set("message", message).Set("is_encrypt", types.MESSAGE_IS_ENCRYPT).Where(sq.Eq{"id": id})
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

func (s *ChatMessageStore) AppendMessage(ctx context.Context, spaceID, sessionID, id string, message json.RawMessage, complete int32) error {
	query := sq.Update(s.GetTable()).Set("message", sq.Expr("message || ?", message)).Set("complete", complete).Where(sq.Eq{"space_id": spaceID, "session_id": sessionID, "id": id})
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

func (s *ChatMessageStore) UpdateMessageCompleteStatus(ctx context.Context, sessionID, id string, complete int32) error {
	query := sq.Update(s.GetTable()).Set("complete", complete).Where(sq.Eq{"session_id": sessionID, "id": id})
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

func (s *ChatMessageStore) UpdateMessageAttach(ctx context.Context, sessionID, id string, attach types.ChatMessageAttach) error {
	query := sq.Update(s.GetTable()).Set("attach", attach.String()).Where(sq.Eq{"session_id": sessionID, "id": id})
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

func (s *ChatMessageStore) DeleteSessionMessage(ctx context.Context, spaceID, sessionID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "session_id": sessionID})
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

func (s *ChatMessageStore) DeleteMessage(ctx context.Context, id string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"id": id})
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

func (s *ChatMessageStore) DeleteAll(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})
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

func (s *ChatMessageStore) ListSessionMessageUpToGivenID(ctx context.Context, spaceID, sessionID, msgID string, page, pageSize uint64) ([]*types.ChatMessage, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "session_id": sessionID}).Where(sq.LtOrEq{"id": msgID}).OrderBy("id DESC")

	if page != types.NO_PAGING || pageSize != types.NO_PAGING {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var list []*types.ChatMessage
	if err = s.GetReplica(ctx).Select(&list, queryString, args...); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *ChatMessageStore) ListSessionMessage(ctx context.Context, spaceID, sessionID, msgID string, page, pageSize uint64) ([]*types.ChatMessage, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "session_id": sessionID}).OrderBy("send_time DESC, id DESC")
	if msgID != "" {
		query = query.Where(sq.Gt{"id": msgID})
	}
	if page != types.NO_PAGING || pageSize != types.NO_PAGING {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var list []*types.ChatMessage
	if err = s.GetReplica(ctx).Select(&list, queryString, args...); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *ChatMessageStore) ListUnEncryptMessage(ctx context.Context, page, pageSize uint64) ([]*types.ChatMessage, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).
		Where(sq.And{sq.NotEq{"is_encrypt": types.MESSAGE_IS_ENCRYPT}, sq.Eq{"complete": types.MESSAGE_PROGRESS_COMPLETE}})

	if page != types.NO_PAGING || pageSize != types.NO_PAGING {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var list []*types.ChatMessage
	if err = s.GetReplica(ctx).Select(&list, queryString, args...); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *ChatMessageStore) TotalSessionMessage(ctx context.Context, spaceID, sessionID, msgID string) (int64, error) {
	query := sq.Select("COUNT(*)").From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "session_id": sessionID})
	if msgID != "" {
		query = query.Where(sq.Gt{"id": msgID})
	}
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

func (s *ChatMessageStore) Exist(ctx context.Context, spaceID, sessionID, msgID string) (bool, error) {
	query := sq.Select("1").From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "session_id": sessionID, "id": msgID})
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

func (s *ChatMessageStore) GetMessagesByIDs(ctx context.Context, msgIDs []string) ([]*types.ChatMessage, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"id": msgIDs})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var msgs []*types.ChatMessage
	if err = s.GetReplica(ctx).Select(&msgs, queryString, args...); err != nil {
		return nil, err
	}

	return msgs, nil
}

func (s *ChatMessageStore) GetSessionLatestUserMessage(ctx context.Context, spaceID, sessionID string) (*types.ChatMessage, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "session_id": sessionID, "role": types.USER_ROLE_USER}).OrderBy("send_time DESC, id DESC").Limit(1)
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var msg types.ChatMessage
	if err := s.GetReplica(ctx).Get(&msg, queryString, args...); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *ChatMessageStore) GetSessionLatestMessage(ctx context.Context, spaceID, sessionID string) (*types.ChatMessage, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "session_id": sessionID}).OrderBy("send_time DESC, id DESC").Limit(1)
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var msg types.ChatMessage
	if err := s.GetReplica(ctx).Get(&msg, queryString, args...); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *ChatMessageStore) GetSessionLatestUserMsgIDBeforeGivenID(ctx context.Context, spaceID, sessionID, msgID string) (string, error) {
	query := sq.Select("id").From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "session_id": sessionID}).Where(sq.LtOrEq{"id": msgID}).Where(sq.Eq{"role": types.USER_ROLE_USER}).OrderBy("id DESC").Limit(1)
	queryString, args, err := query.ToSql()
	if err != nil {
		return "", ErrorSqlBuild(err)
	}

	var id string
	if err := s.GetReplica(ctx).Get(&id, queryString, args...); err != nil {
		return "", err
	}
	return id, nil
}
