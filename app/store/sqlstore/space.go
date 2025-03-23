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
		provider.stores.SpaceStore = NewSpaceStore(provider)
	})
}

// UserSpaceStore 用于处理用户与空间关系表的操作
type SpaceStore struct {
	CommonFields
}

// NewUserSpaceRelationStore 创建新的 UserSpaceRelationStore 实例
func NewSpaceStore(provider SqlProviderAchieve) *SpaceStore {
	repo := &SpaceStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_SPACE)
	repo.SetAllColumns("space_id", "title", "base_prompt", "chat_prompt", "description", "created_at")
	return repo
}

// Create 创建新的用户与空间关系
func (s *SpaceStore) Create(ctx context.Context, data types.Space) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("space_id", "title", "base_prompt", "chat_prompt", "description", "created_at").
		Values(data.SpaceID, data.Title, data.BasePrompt, data.ChatPrompt, data.Description, data.CreatedAt)

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

func (s *SpaceStore) GetSpace(ctx context.Context, spaceID string) (*types.Space, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.Space
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *SpaceStore) Update(ctx context.Context, spaceID, title, desc, basePrompt, chatPrompt string) error {
	if title == "" && desc == "" {
		return nil
	}
	query := sq.Update(s.GetTable()).
		Where(sq.Eq{"space_id": spaceID})
	if title != "" {
		query = query.Set("title", title)
	}
	if desc != "" {
		query = query.Set("description", desc)
	}

	query = query.Set("base_prompt", basePrompt).Set("chat_prompt", chatPrompt)
	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *SpaceStore) Delete(ctx context.Context, spaceID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"space_id": spaceID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// List 分页获取用户与空间关系记录
func (s *SpaceStore) List(ctx context.Context, spaceIDs []string, page, pageSize uint64) ([]types.Space, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"space_id": spaceIDs}).OrderBy("created_at")
	if page != 0 && pageSize != 0 {
		query = query.Limit(pageSize).Offset((page - 1) * pageSize)
	}

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.Space
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
