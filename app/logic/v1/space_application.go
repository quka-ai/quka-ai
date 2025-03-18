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

func NewSpaceApplicationLogic(ctx context.Context, core *core.Core) *SpaceApplicationLogic {
	l := &SpaceApplicationLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}

	return l
}

type SpaceApplicationLogic struct {
	ctx  context.Context
	core *core.Core
	UserInfo
}

func (l *SpaceApplicationLogic) Apply(spaceToken, desc string) (string, error) {
	invite, err := l.core.Store().ShareTokenStore().GetByToken(l.ctx, spaceToken)
	if err != nil && err != sql.ErrNoRows {
		return "", errors.New("SpaceApplicationLogic.Apply.ShareTokenStore.GetByToken", i18n.ERROR_INTERNAL, err)
	}

	if invite == nil || invite.Type != types.SHARE_TYPE_SPACE_INVITE {
		return "", errors.New("SpaceApplicationLogic.Apply.ShareTokenStore.GetByToken.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	space, err := l.core.Store().SpaceStore().GetSpace(l.ctx, invite.SpaceID)
	if err != nil {
		return "", errors.New("SpaceApplicationLogic.Apply.SpaceStore.GetSpace", i18n.ERROR_INTERNAL, err)
	}

	application, err := l.core.Store().SpaceApplicationStore().Get(l.ctx, invite.SpaceID, l.GetUserInfo().User)
	if err != nil && err != sql.ErrNoRows {
		return "", errors.New("SpaceApplicationLogic.Apply.SpaceApplicationStore.Get", i18n.ERROR_INTERNAL, err)
	}

	if application != nil {
		return "", errors.New("SpaceApplicationLogic.Apply.SpaceApplicationStore.Get.not.nil", i18n.ERROR_ALREADY_APPLIED, err)
	}

	applicant, err := l.core.Store().UserStore().GetUser(l.ctx, l.GetUserInfo().Appid, l.GetUserInfo().User)
	if err != nil {
		return "", errors.New("SpaceApplicationLogic.Apply.UserStore.GetUser", i18n.ERROR_INTERNAL, err)
	}

	err = l.core.Store().SpaceApplicationStore().Create(l.ctx, &types.SpaceApplication{
		SpaceID:   space.SpaceID,
		UserID:    applicant.ID,
		UserName:  applicant.Name,
		UserEmail: applicant.Email,
		Desc:      desc,
		UpdatedAt: time.Now().Unix(),
		CreatedAt: time.Now().Unix(),
	})

	if err != nil {
		return "", errors.New("SpaceApplicationLogic.Application.SpaceApplicationStore.Create", i18n.ERROR_INTERNAL, err)
	}

	// TODO：check user's leaves is more than space join leaves condition
	return types.SPACE_APPLICATION_WAITING, nil
}

type SpaceApplicationWaitingItem struct {
	User   SpaceApplicationUser `json:"user"`
	Desc   string               `json:"desc"`
	UserID string               `json:"user_id"`
	Status string               `json:"status"`
}

type SpaceApplicationUser struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatar"`
}

func (l *SpaceApplicationLogic) WaitingList(spaceID string, opts types.ListSpaceApplicationOptions, page, pagesize uint64) ([]SpaceApplicationWaitingItem, int64, error) {
	list, err := l.core.Store().SpaceApplicationStore().List(l.ctx, spaceID, opts, page, pagesize)
	if err != nil && err != sql.ErrNoRows {
		return nil, 0, errors.New("SpaceApplicationLogic.WaitingList.SpaceApplicationStore.List", i18n.ERROR_NOT_FOUND, err)
	}

	total, err := l.core.Store().SpaceApplicationStore().Total(l.ctx, spaceID, opts)
	if err != nil {
		return nil, 0, errors.New("SpaceApplicationLogic.WaitingList.SpaceApplicationStore.Total", i18n.ERROR_NOT_FOUND, err)
	}

	userIDs := lo.Map(list, func(item types.SpaceApplication, _ int) string {
		return item.UserID
	})

	userList, err := l.core.Store().UserStore().ListUsers(l.ctx, types.ListUserOptions{
		IDs: userIDs,
	}, types.NO_PAGING, types.NO_PAGING)

	if err != nil && err != sql.ErrNoRows {
		return nil, 0, errors.New("SpaceApplicationLogic.WaitingList.UserStore.ListUsers", i18n.ERROR_INTERNAL, err)
	}

	userIndex := lo.SliceToMap(userList, func(item types.User) (string, types.User) {
		return item.ID, item
	})

	var result []SpaceApplicationWaitingItem
	for _, v := range list {
		user := userIndex[v.UserID]
		result = append(result, SpaceApplicationWaitingItem{
			UserID: v.UserID,
			Desc:   v.Desc,
			User: SpaceApplicationUser{
				Avatar: user.Avatar,
				Email:  user.Email,
				Name:   user.Name,
				ID:     user.ID,
			},
		})
	}

	return result, total, nil
}

func (l *SpaceApplicationLogic) HandlerApplication(id, status string) error {
	user := l.GetUserInfo()
	if !l.core.Srv().RBAC().CheckPermission(user.GetRole(), srv.PermissionAdmin) {
		return errors.New("SpaceApplicationLogic.HandlerApplication.RBAC.CheckPermission", i18n.ERROR_PERMISSION_DENIED, nil).Code(http.StatusForbidden)
	}

	data, err := l.core.Store().SpaceApplicationStore().GetByID(l.ctx, id)
	if err != nil && err != sql.ErrNoRows {
		return errors.New("SpaceApplicationLogic.HandlerApplication.SpaceApplicationStore.GetByID", i18n.ERROR_INTERNAL, err)
	}

	if data == nil {
		return errors.New("SpaceApplicationLogic.HandlerApplication.SpaceApplicationStore.GetByID.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	if status == types.SPACE_APPLICATION_ACCESS {
		return l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {
			err = l.core.Store().UserSpaceStore().Create(l.ctx, types.UserSpace{
				UserID:    data.UserID,
				SpaceID:   data.SpaceID,
				Role:      srv.RoleMember,
				CreatedAt: time.Now().Unix(),
			})
			if err != nil {
				return errors.New("SpaceApplicationLogic.HandlerApplication.UserSpaceStore.Create", i18n.ERROR_INTERNAL, err)
			}
			if err = l.core.Store().SpaceApplicationStore().UpdateStatus(l.ctx, id, status); err != nil {
				return errors.New("SpaceApplicationLogic.HandlerApplication.SpaceApplicationStore.UpdateStatus", i18n.ERROR_INTERNAL, err)
			}
			return nil
		})
	}
	if err = l.core.Store().SpaceApplicationStore().UpdateStatus(l.ctx, id, status); err != nil {
		return errors.New("SpaceApplicationLogic.HandlerApplication.SpaceApplicationStore.UpdateStatus", i18n.ERROR_INTERNAL, err)
	}
	return nil
}

type SpaceApplicationLandingDetail struct {
	ID                string          `json:"id"`
	Title             string          `json:"title"`
	Desc              string          `json:"desc"`
	Maintainer        SpaceMaintainer `json:"user"`
	ApplicationStatus string          `json:"application_status"`
}

type SpaceMaintainer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

func (l *SpaceApplicationLogic) LandingDetail(spaceToken string) (SpaceApplicationLandingDetail, error) {
	data, err := l.core.Store().ShareTokenStore().GetByToken(l.ctx, spaceToken)
	if err != nil && err != sql.ErrNoRows {
		return SpaceApplicationLandingDetail{}, errors.New("SpaceApplicationLogic.LandingDetail.ShareTokenStore.GetByToken", i18n.ERROR_INTERNAL, err)
	}

	if data == nil || data.Type != types.SHARE_TYPE_SPACE_INVITE {
		return SpaceApplicationLandingDetail{}, errors.New("SpaceApplicationLogic.LandingDetail.ShareTokenStore.GetByToken.nil", i18n.ERROR_NOT_FOUND, nil).Code(http.StatusNoContent)
	}

	space, err := l.core.Store().SpaceStore().GetSpace(l.ctx, data.SpaceID)
	if err != nil {
		return SpaceApplicationLandingDetail{}, errors.New("SpaceApplicationLogic.LandingDetail.SpaceStore.GetSpace", i18n.ERROR_INTERNAL, err)
	}

	application, err := l.core.Store().SpaceApplicationStore().Get(l.ctx, data.SpaceID, l.GetUserInfo().User)
	if err != nil && err != sql.ErrNoRows {
		return SpaceApplicationLandingDetail{}, errors.New("SpaceApplicationLogic.LandingDetail.SpaceApplicationStore.Get", i18n.ERROR_INTERNAL, err)
	}

	spaceMaintainer, err := l.core.Store().UserStore().GetUser(l.ctx, l.GetUserInfo().Appid, space.SpaceID)
	if err != nil {
		return SpaceApplicationLandingDetail{}, errors.New("SpaceApplicationLogic.LandingDetail.UserStore.GetUser", i18n.ERROR_INTERNAL, err)
	}

	return SpaceApplicationLandingDetail{
		ID:    space.SpaceID,
		Title: space.Title,
		Desc:  space.Description,
		Maintainer: SpaceMaintainer{
			ID:     spaceMaintainer.ID,
			Name:   spaceMaintainer.Name,
			Avatar: spaceMaintainer.Avatar,
		},
		ApplicationStatus: lo.If(application == nil, types.SPACE_APPLICATION_NONE).Else(application.Status),
	}, nil
}
