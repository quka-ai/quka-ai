package v1

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/errors"
	"github.com/quka-ai/quka-ai/pkg/i18n"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type UploadLogic struct {
	ctx  context.Context
	core *core.Core
	UserInfo
}

func NewUploadLogic(ctx context.Context, core *core.Core) *UploadLogic {
	l := &UploadLogic{
		ctx:      ctx,
		core:     core,
		UserInfo: SetupUserInfo(ctx, core),
	}

	return l
}

type UploadKey struct {
	Key          string `json:"key"`
	FullPath     string `json:"full_path"`
	StaticDomain string `json:"static_domain"`
	Status       string `json:"status"`
}

const (
	UPLOAD_STATUS_EXIST = "exist"
)

func hashFileName(fileName string) string {
	result := strings.Split(fileName, ".")
	var suffix string
	if len(result) > 1 {
		suffix = "." + result[len(result)-1]
		fileName = strings.TrimSuffix(fileName, suffix)
	}

	return utils.MD5(fileName) + suffix
}

func randomFileName(fileName string) string {
	result := strings.Split(fileName, ".")
	var suffix string
	if len(result) > 1 {
		suffix = "." + result[len(result)-1]
		fileName = strings.TrimSuffix(fileName, suffix)
	}
	return utils.MD5(fmt.Sprintf("%s%d", fileName, time.Now().Truncate(time.Second*10).Unix())) + suffix
}

func (l *UploadLogic) GenClientUploadKey(objectType, kind, fileName string, size int64) (UploadKey, error) {
	userID := l.UserInfo.GetUserInfo().User
	spaceID, _ := InjectSpaceID(l.ctx)
	fileName = randomFileName(fileName)
	fullPath := types.GenS3FilePath(spaceID, objectType, fileName)
	exist, err := l.core.Store().FileManagementStore().GetByID(l.ctx, spaceID, fullPath)
	if err != nil && err != sql.ErrNoRows {
		return UploadKey{}, errors.New("UploadLogic.FileManagementStore.GetById", i18n.ERROR_INTERNAL, err)
	}

	if exist != nil {
		return UploadKey{
			Status:       UPLOAD_STATUS_EXIST,
			StaticDomain: l.core.Plugins.FileStorage().GetStaticDomain(),
			FullPath:     fullPath,
		}, nil
	}

	if size > 1024*1024*30 {
		return UploadKey{}, errors.New("UploadLogic.FileManagementStore.GreateThanMaxSize", i18n.ERROR_MORE_TAHN_MAX, nil).Code(http.StatusForbidden)
	}

	var meta core.UploadFileMeta
	err = l.core.Store().Transaction(l.ctx, func(ctx context.Context) error {

		err := l.core.Store().FileManagementStore().Create(l.ctx, types.FileManagement{
			SpaceID:    spaceID,
			UserID:     userID,
			File:       fullPath,
			FileSize:   size,
			Status:     types.FILE_UPLOAD_STATUS_UNKNOWN,
			Kind:       kind,
			ObjectType: objectType,
			CreatedAt:  time.Now().Unix(),
		})
		if err != nil {
			return errors.New("UploadLogic.GenClientUploadKey.FileManagementStore.Create", i18n.ERROR_INTERNAL, err)
		}

		meta, err = l.core.Plugins.FileStorage().GenUploadFileMeta(fullPath, size)
		if err != nil {
			return errors.New("UploadLogic.GenClientUploadKey.FileUploader.GenUploadFileMeta", i18n.ERROR_INTERNAL, err)
		}
		return nil
	})
	if err != nil {
		return UploadKey{}, err
	}

	return UploadKey{
		Key:          meta.UploadEndpoint,
		FullPath:     meta.FullPath,
		StaticDomain: l.core.FileStorage().GetStaticDomain(),
	}, nil
}
