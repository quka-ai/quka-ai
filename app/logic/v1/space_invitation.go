package v1

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/samber/lo"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/core/srv"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type SpaceInvitationLogic struct {
	UserInfo
	ctx  context.Context
	core *core.Core
}

func NewSpaceInvitationLogic(ctx context.Context, core *core.Core) *SpaceInvitationLogic {
	l := &SpaceInvitationLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}

	return l
}

func (l *SpaceInvitationLogic) CreateSpaceInvitation(spaceID, invitee, role string) error {
	switch role {
	case srv.PermissionEdit:
	case srv.PermissionView:
	default:
		return errors.New("SpaceLogic.CreateSpaceInvitation", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
	}

	exist, err := l.core.Store().SpaceInvitationStore().Get(l.ctx, l.GetUserInfo().Appid, l.GetUserInfo().User, invitee)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("SpaceLogic.CreateSpaceInvitation.SpaceInvitationStore.Get", i18n.ERROR_INTERNAL, err)
	}

	if exist != nil {
		return errors.New("SpaceLogic.CreateSpaceInvitation.Already", i18n.ERROR_ALREADY_INVITED, nil).Code(http.StatusForbidden)
	}

	// check user already join in the space
	{
		user, err := l.core.Store().UserStore().GetByEmail(l.ctx, l.GetUserInfo().Appid, invitee)
		if err != nil {
			return errors.New("SpaceLogic.CreateSpaceInvitation.UserStore.GetByEmail", i18n.ERROR_INTERNAL, err)
		}
		exist, err := l.core.Store().UserSpaceStore().GetUserSpaceRole(l.ctx, user.ID, spaceID)
		if err != nil && err != sql.ErrNoRows {
			return errors.New("SpaceLogic.CreateSpaceInvitation.UserSpaceStore.GetUserSpaceRole", i18n.ERROR_INTERNAL, err)
		}

		if exist != nil {
			return errors.New("SpaceLogic.CreateSpaceInvitation.AlreadyInvited", i18n.ERROR_ALREADY_INVITED, nil).Code(http.StatusForbidden)
		}
	}

	return l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		err := l.core.Store().SpaceInvitationStore().Create(ctx, types.Invitation{
			Appid:        l.GetUserInfo().Appid,
			SpaceID:      spaceID,
			InviterID:    l.GetUserInfo().User,
			InviteeEmail: invitee,
			Role:         role,
			InviteStatus: types.SPACE_INVITATION_STATUS_PENDING,
			CreatedAt:    time.Now().Unix(),
			UpdatedAt:    time.Now().Unix(),
			ExpiredAt:    time.Now().AddDate(0, 1, 0).Unix(),
		})
		if err != nil {
			return errors.New("SpaceLogic.CreateSpaceInvitation.SpaceInvitationStore.Create", i18n.ERROR_INTERNAL, err)
		}

		// TODO: Add notification creation logic

		return nil
	})
}

type Invitation struct {
	ID           int64                       `json:"id"`
	InviterID    string                      `json:"inviter_id"`
	Inviter      string                      `json:"inviter"`
	InviteeEmail string                      `json:"invitee_email"`
	SpaceID      string                      `json:"space_id"`
	Role         string                      `json:"role"`
	InviteStatus types.SpaceInvitationStatus `json:"invite_status"`
	CreatedAt    int64                       `json:"created_at"`
}

func (l *SpaceInvitationLogic) ListInvitations(spaceID string, opts types.ListSpaceInvitationOptions, page, pagesize uint64) ([]Invitation, int64, error) {
	list, err := l.core.Store().SpaceInvitationStore().List(l.ctx, l.GetUserInfo().Appid, spaceID, opts, page, pagesize)
	if err != nil {
		return nil, 0, errors.New("SpaceLogic.ListInvitations.SpaceInvitationStore.List", i18n.ERROR_INTERNAL, err)
	}

	if len(list) == 0 {
		return nil, 0, nil
	}

	inviterIDs := lo.Uniq(lo.Map(list, func(user types.Invitation, _ int) string {
		return user.InviterID
	}))

	users, err := l.core.Store().UserStore().ListUsers(l.ctx, types.ListUserOptions{
		Appid: l.GetUserInfo().Appid,
		IDs:   inviterIDs,
	}, types.NO_PAGINATION, types.NO_PAGINATION)
	if err != nil {
		return nil, 0, errors.New("SpaceLogic.ListInvitations.UserStore.ListUsers", i18n.ERROR_INTERNAL, err)
	}

	userMap := lo.SliceToMap(users, func(user types.User) (string, string) {
		return user.ID, user.Name
	})

	var (
		result []Invitation
	)
	for _, v := range list {
		result = append(result, Invitation{
			ID:           v.ID,
			InviteStatus: v.InviteStatus,
			InviterID:    v.InviterID,
			Inviter:      userMap[v.InviterID],
			InviteeEmail: v.InviteeEmail,
			SpaceID:      v.SpaceID,
			Role:         v.Role,
			CreatedAt:    v.CreatedAt,
		})
	}

	total, err := l.core.Store().SpaceInvitationStore().Total(l.ctx, l.GetUserInfo().Appid, spaceID, opts)
	if err != nil {
		return nil, 0, errors.New("SpaceLogic.ListInvitations.SpaceInvitationStore.Total", i18n.ERROR_INTERNAL, err)
	}

	return result, total, nil
}

func (l *SpaceInvitationLogic) HandlerInviter(id int64, status types.SpaceInvitationStatus) error {
	invitation, err := l.core.Store().SpaceInvitationStore().GetByID(l.ctx, l.GetUserInfo().Appid, id)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("SpaceLogic.HandlerInviter.SpaceInvitationStore.GetByID", i18n.ERROR_INTERNAL, err)
	}

	if invitation == nil || invitation.InviteStatus != types.SPACE_INVITATION_STATUS_PENDING {
		return errors.New("SpaceLogic.HandlerInviter.SpaceInvitationStore.GetByID.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	return l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
		switch status {
		case types.SPACE_INVITATION_STATUS_ACCEPTED:
			user, err := l.core.Store().UserStore().GetByEmail(l.ctx, invitation.Appid, invitation.InviteeEmail)
			if err != nil {
				return errors.New("SpaceLogic.HandlerInviter.UserStore.GetByEmail", i18n.ERROR_INTERNAL, err)
			}
			// check user already join in the space
			exist, err := l.core.Store().UserSpaceStore().GetUserSpaceRole(l.ctx, user.ID, invitation.SpaceID)
			if err != nil && err != sql.ErrNoRows {
				return errors.New("SpaceLogic.HandlerInviter.UserSpaceStore.GetUserSpaceRole", i18n.ERROR_INTERNAL, err)
			}

			if exist != nil {
				return errors.New("SpaceLogic.HandlerInviter.AlreadyInvited", i18n.ERROR_ALREADY_INVITED, nil).Code(http.StatusForbidden)
			}
			err = l.core.Store().UserSpaceStore().Create(ctx, types.UserSpace{
				UserID:    user.ID,
				SpaceID:   invitation.SpaceID,
				Role:      invitation.Role,
				CreatedAt: time.Now().Unix(),
			})
			if err != nil {
				return errors.New("SpaceLogic.HandlerInviter.UserSpaceStore.Create", i18n.ERROR_INTERNAL, err)
			}
		case types.SPACE_INVITATION_STATUS_EXPIRED:
			return errors.New("SpaceLogic.HandlerInviter.SpaceInvitationStore.Expired", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
		case types.SPACE_INVITATION_STATUS_REJECTED:
		default:
			return errors.New("SpaceLogic.HandlerInviter.SpaceInvitationStore.UnknownStatus", i18n.ERROR_INVALIDARGUMENT, nil).Code(http.StatusBadRequest)
		}

		if err = l.core.Store().SpaceInvitationStore().UpdateStatus(ctx, l.GetUserInfo().Appid, id, status); err != nil {
			return errors.New("SpaceLogic.HandlerInviter.SpaceInvitationStore.UpdateStatus", i18n.ERROR_INTERNAL, err)
		}
		return nil
	})
}
