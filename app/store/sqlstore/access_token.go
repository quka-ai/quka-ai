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
		provider.stores.AccessTokenStore = NewAccessTokenStore(provider)
	})
}

type AccessTokenStore struct {
	CommonFields
}

// NewBwAccessTokenStore 创建新的 BwAccessTokenStore 实例
func NewAccessTokenStore(provider SqlProviderAchieve) *AccessTokenStore {
	repo := &AccessTokenStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_ACCESS_TOKEN)
	repo.SetAllColumns("id", "appid", "user_id", "token", "version", "created_at", "expires_at", "info")
	return repo
}

// Create 创建新的 access_token 记录
func (s *AccessTokenStore) Create(ctx context.Context, data types.AccessToken) error {
	if data.CreatedAt == 0 {
		data.CreatedAt = time.Now().Unix()
	}
	query := sq.Insert(s.GetTable()).
		Columns("user_id", "appid", "token", "version", "created_at", "expires_at", "info").
		Values(data.UserID, data.Appid, data.Token, data.Version, data.CreatedAt, data.ExpiresAt, data.Info)

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

// GetBwAccessToken 根据ID获取 access_token 记录
func (s *AccessTokenStore) GetAccessToken(ctx context.Context, appid, token string) (*types.AccessToken, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"appid": appid, "token": token})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.AccessToken
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新 access_token 记录
// func (s *AccessTokenStore) Update(ctx context.Context, id int64, data types.AccessToken) error {
// 	query := sq.Update(s.GetTable()).
// 		Set("user_id", data.UserID).
// 		Set("token", data.Token).
// 		Set("detail", data.Detail).
// 		Set("expires_at", data.ExpiresAt).
// 		Set("desc", data.Desc).
// 		Where(sq.Eq{"id": id})

// 	queryString, args, err := query.ToSql()
// 	if err != nil {
// 		return errorSqlBuild(err)
// 	}

// 	_, err = s.GetMaster(ctx).Exec(queryString, args...)
// 	return err
// }

// Delete 删除 access_token 记录
func (s *AccessTokenStore) Delete(ctx context.Context, appid, userID string, id int64) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"appid": appid, "user_id": userID, "id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

func (s *AccessTokenStore) Deletes(ctx context.Context, appid, userID string, ids []int64) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"appid": appid, "user_id": userID, "id": ids})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// ListBwAccessTokens 分页获取 access_token 记录列表
func (s *AccessTokenStore) ListAccessTokens(ctx context.Context, appid, userID string, page, pageSize uint64) ([]types.AccessToken, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).
		Where(sq.Eq{"appid": appid, "user_id": userID}).Limit(pageSize).Offset((page - 1) * pageSize).OrderBy("created_at DESC")

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.AccessToken
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *AccessTokenStore) Total(ctx context.Context, appid, userID string) (int64, error) {
	query := sq.Select("COUNT(*)").From(s.GetTable()).
		Where(sq.Eq{"appid": appid, "user_id": userID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return 0, ErrorSqlBuild(err)
	}

	var res int64
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return 0, err
	}
	return res, nil
}

func (s *AccessTokenStore) ClearUserTokens(ctx context.Context, appid, userID string) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"appid": appid, "user_id": userID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}
