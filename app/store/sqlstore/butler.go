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
		provider.stores.ButlerTableStore = NewButlerTableStore(provider)
	})
}

// ButlerStore 处理 bw_butler 表的操作
type ButlerStore struct {
	CommonFields // CommonFields 是定义在该代码所在包内的，所以可以直接使用，不用加types.来引用
}

// NewBwButlerStore 创建新的 BwButlerStore 实例
func NewButlerTableStore(provider SqlProviderAchieve) *ButlerStore {
	repo := &ButlerStore{}
	repo.SetProvider(provider)
	repo.SetTable(types.TABLE_BUTLER) // 使用 types 包中的常量
	repo.SetAllColumns("table_id", "user_id", "table_name", "table_description", "table_data", "created_at", "updated_at")
	return repo
}

// Create 创建新的日常事项记录
func (s *ButlerStore) Create(ctx context.Context, data types.ButlerTable) error {
	query := sq.Insert(s.GetTable()).
		Columns("table_id", "user_id", "table_name", "table_description", "table_data", "created_at", "updated_at").
		Values(data.TableID, data.UserID, data.TableName, data.TableDescription, data.TableData, time.Now().Unix(), time.Now().Unix())

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

// GetBwButler 根据ID获取日常事项记录
func (s *ButlerStore) GetTableData(ctx context.Context, id string) (*types.ButlerTable, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"table_id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res types.ButlerTable
	if err = s.GetReplica(ctx).Get(&res, queryString, args...); err != nil {
		return nil, err
	}
	return &res, nil
}

// Update 更新日常事项记录
func (s *ButlerStore) Update(ctx context.Context, id string, data string) error {
	query := sq.Update(s.GetTable()).
		Set("table_data", data).
		Set("updated_at", time.Now().Unix()).
		Where(sq.Eq{"table_id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// Delete 删除日常事项记录
func (s *ButlerStore) Delete(ctx context.Context, id int64) error {
	query := sq.Delete(s.GetTable()).Where(sq.Eq{"table_id": id})

	queryString, args, err := query.ToSql()
	if err != nil {
		return ErrorSqlBuild(err)
	}

	_, err = s.GetMaster(ctx).Exec(queryString, args...)
	return err
}

// ListBwButlers 分页获取日常事项记录列表
func (s *ButlerStore) ListButlerTables(ctx context.Context, userID string) ([]types.ButlerTable, error) {
	query := sq.Select(s.GetAllColumns()...).From(s.GetTable()).Where(sq.Eq{"user_id": userID})

	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, ErrorSqlBuild(err)
	}

	var res []types.ButlerTable
	if err = s.GetReplica(ctx).Select(&res, queryString, args...); err != nil {
		return nil, err
	}
	return res, nil
}
