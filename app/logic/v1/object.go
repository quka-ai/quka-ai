package v1

import (
	"context"
	"database/sql"
	"strings"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
)

type ObjectLogic struct {
	ctx  context.Context
	core *core.Core
}

func NewObjectLogic(ctx context.Context, core *core.Core) *ObjectLogic {
	return &ObjectLogic{
		ctx:  ctx,
		core: core,
	}
}

func (l *ObjectLogic) CheckObjectPermission(userID, spaceID, objectPath string) (bool, error) {
	if l.IsPublicResource(objectPath) {
		return true, nil
	}

	userSpaceStore := l.core.Store().UserSpaceStore()

	userSpace, err := userSpaceStore.GetUserSpaceRole(l.ctx, userID, spaceID)
	if err != nil && err != sql.ErrNoRows {
		return false, errors.New("ObjectLogic.CheckObjectPermission.GetUserSpaceRole", i18n.ERROR_INTERNAL, err)
	}

	if userSpace == nil {
		return false, nil
	}

	if !l.isValidObjectPath(objectPath, spaceID) {
		return false, nil
	}

	return true, nil
}

func (l *ObjectLogic) IsPublicResource(objectPath string) bool {
	if objectPath == "" {
		return false
	}

	if strings.Contains(objectPath, "../") {
		return false
	}

	publicPatterns := []string{
		"/avatar/",
		"/public/",
		"/quka/assets/public/",
		"/quka/assets/avatar/",
	}

	for _, pattern := range publicPatterns {
		if strings.Contains(objectPath, pattern) {
			return true
		}
	}

	return false
}

func (l *ObjectLogic) isValidObjectPath(objectPath, spaceID string) bool {
	if objectPath == "" {
		return false
	}

	if strings.Contains(objectPath, "../") {
		return false
	}

	if !strings.HasPrefix(objectPath, spaceID+"/") {
		return false
	}

	return true
}
