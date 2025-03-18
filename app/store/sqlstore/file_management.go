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
		provider.stores.FileManagementStore = NewFileManagementStore(provider)
	})
}

type FileManagementStore struct {
	CommonFields
}

func NewFileManagementStore(provider SqlProviderAchieve) *FileManagementStore {
	repo := &FileManagementStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_FILE_MANAGEMENT)
	repo.SetAllColumns("id", "space_id", "user_id", "file", "file_size", "object_type", "kind", "status", "created_at")
	return repo
}

// Create 创建新的文件记录
func (s *FileManagementStore) Create(ctx context.Context, data types.FileManagement) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("space_id", "user_id", "file", "file_size", "object_type", "kind", "status", "created_at").
		Values(data.SpaceID, data.UserID, data.File, data.FileSize, data.ObjectType, data.Kind, data.Status, data.CreatedAt)

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

// GetByID 根据ID获取文件记录
func (s *FileManagementStore) GetByID(ctx context.Context, spaceID, file string) (*types.FileManagement, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "file": file})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.FileManagement
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *FileManagementStore) UpdateStatus(ctx context.Context, spaceID string, files []string, status int) error {
	query := sq.Update(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "file": files}).Set("status", status)

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 根据ID删除文件记录
func (s *FileManagementStore) Delete(ctx context.Context, spaceID, file string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID, "file": file})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}
