package sqlstore

import (
	"context"

	sq "github.com/Masterminds/squirrel"

	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	register.RegisterFunc[*Provider](RegisterKey{}, func(provider *Provider) {
		provider.stores.SpaceInvitationStore = NewSpaceInvitationStore(provider)
	})
}

// SpaceInvitationStoreImpl 提供 Invitation 表的操作
type SpaceInvitationStoreImpl struct {
	CommonFields
}

// NewInvitationStore 创建新的 InvitationStore
func NewSpaceInvitationStore(provider SqlProviderAchieve) *SpaceInvitationStoreImpl {
	repo := &SpaceInvitationStoreImpl{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_SPACE_INVITATION)
	repo.SetAllColumns(
		"id", "appid", "inviter_id", "invitee_email", "space_id", "role",
		"invite_status", "created_at", "expired_at", "updated_at",
	)
	return repo
}

// Create 创建新的邀请记录
func (s *SpaceInvitationStoreImpl) Create(ctx context.Context, data types.Invitation) error {
	query := sq.Insert(s.GetTable()).
		Columns("appid", "inviter_id", "invitee_email", "space_id", "role", "invite_status", "created_at", "expired_at", "updated_at").
		Values(data.Appid, data.InviterID, data.InviteeEmail, data.SpaceID, data.Role, data.InviteStatus, data.CreatedAt, data.ExpiredAt, data.UpdatedAt)

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

// Get 根据 ID 获取邀请记录
func (s *SpaceInvitationStoreImpl) Get(ctx context.Context, appid, inviterID, inviteeEmail string) (*types.Invitation, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"appid": appid, "inviter_id": inviterID, "invitee_email": inviteeEmail})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.Invitation
	if err := s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *SpaceInvitationStoreImpl) GetByID(ctx context.Context, appid string, id int64) (*types.Invitation, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"appid": appid, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.Invitation
	if err := s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新邀请记录
func (s *SpaceInvitationStoreImpl) UpdateStatus(ctx context.Context, appid string, id int64, status types.SpaceInvitationStatus) error {
	query := sq.Update(s.GetTable()).
		Set("invite_status", status).
		Where(sq.Eq{"id": id, "appid": appid})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除邀请记录
func (s *SpaceInvitationStoreImpl) Delete(ctx context.Context, appid string, id int64) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"id": id, "appid": appid})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// List 分页获取邀请记录列表
func (s *SpaceInvitationStoreImpl) List(ctx context.Context, appid, spaceID string, opts types.ListSpaceInvitationOptions, page, pageSize uint64) ([]types.Invitation, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).Where(sq.Eq{"appid": appid, "space_id": spaceID}).
		Limit(pageSize).
		Offset((page - 1) * pageSize)

	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.Invitation
	if err := s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *SpaceInvitationStoreImpl) Total(ctx context.Context, appid, spaceID string, opts types.ListSpaceInvitationOptions) (int64, error) {
	query := sq.Select("COUNT(*)").
		From(s.GetTable()).Where(sq.Eq{"appid": appid, "space_id": spaceID})

	opts.Apply(&query)

	queryString, args, err := query.ToSql()
	if err != nil {
		return 0, ErrorSqlBuild(err)
	}

	var res int64
	if err := s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return 0, err
	}
	return res, nil
}
