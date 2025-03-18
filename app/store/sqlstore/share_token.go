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
		provider.stores.ShareTokenStore = NewShareTokenStore(provider)
	})
}

func NewShareTokenStore(provider SqlProviderAchieve) *ShareTokenStoreImpl {
	repo := &ShareTokenStoreImpl{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_SHARE_TOKEN)
	repo.SetAllColumns(
		"id", "appid", "space_id", "object_id", "share_user_id", "type", "token", "embedding_url", "expire_at", "created_at",
	)
	return repo
}

type ShareTokenStoreImpl struct {
	CommonFields
}

// Create 创建新的文章分享链接
func (s *ShareTokenStoreImpl) Create(ctx context.Context, link *types.ShareToken) error {
	if link.CreatedAt == 0 {
		link.CreatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("appid", "space_id", "object_id", "share_user_id", "embedding_url", "type", "token", "expire_at", "created_at").
		Values(link.Appid, link.SpaceID, link.ObjectID, link.ShareUserID, link.EmbeddingURL, link.Type, link.Token, link.ExpireAt, link.CreatedAt)

	sql, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(sql, args...)
	return err
}

// Get 获取特定的分享链接
func (s *ShareTokenStoreImpl) Get(ctx context.Context, _type, spaceID, objectID string) (*types.ShareToken, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"type": _type, "space_id": spaceID, "object_id": objectID})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var link types.ShareToken
	err = s.GetReplica(ctx).Get(&link, sql, args...)
	if err != nil {
		return nil, err
	}

	return &link, nil
}

// Get 获取特定的分享链接
func (s *ShareTokenStoreImpl) GetByToken(ctx context.Context, token string) (*types.ShareToken, error) {
	query := sq.Select(s.GetAllColumns()...).
		From(s.GetTable()).
		Where(sq.Eq{"token": token})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var link types.ShareToken
	err = s.GetReplica(ctx).Get(&link, sql, args...)
	if err != nil {
		return nil, err
	}

	return &link, nil
}

func (s *ShareTokenStoreImpl) UpdateExpireTime(ctx context.Context, id, expireAt int64) error {
	query := sq.Update(s.GetTable()).Set("expire_at", expireAt).Where(sq.Eq{"id": id})
	sql, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}
	_, err = s.GetMaster(ctx).Exec(sql, args...)
	return err
}

// Delete 删除分享链接
func (s *ShareTokenStoreImpl) Delete(ctx context.Context, token string) error {
	query := sq.Delete(s.GetTable()).
		Where(sq.Eq{"token": token}).
		PlaceholderFormat(sq.Dollar)

	sql, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(sql, args...)
	return err
}
