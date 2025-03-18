package sqlstore

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/quka-ai/quka-ai/pkg/types"
)

func ErrorSqlBuild(err error) error {
	return fmt.Errorf("failed to build sql query, %w", err)
}

type SqlProviderAchieve interface {
	GetMaster() *sqlx.DB
	GetReplica() *sqlx.DB
	GetDBName() (string, error)
	GetTxFromCtx(ctx context.Context) *sqlx.Tx
}

type GetTableFunc func([]interface{}) string

// store 基础设置
type CommonFields struct {
	table        string
	getTableFunc GetTableFunc
	provider     SqlProviderAchieve
	allColumns   []string
	initFunc     func(SqlProviderAchieve) error
}

func (c *CommonFields) GetTable(key ...interface{}) string {
	if c.getTableFunc != nil {
		return c.getTableFunc(key)
	}
	return c.table
}

func (c *CommonFields) SetAllColumns(str ...string) {
	c.allColumns = str
}

func (c *CommonFields) SetInitFunc(f func(SqlProviderAchieve) error) {
	c.initFunc = f
}

func (c *CommonFields) GetAllColumns() []string {
	return c.allColumns
}

func (c *CommonFields) GetAllColumnsWithPrefix(prefix string) []string {
	var newColumns []string
	for _, v := range c.allColumns {
		newColumns = append(newColumns, prefix+"."+v)
	}
	return newColumns
}

func (c *CommonFields) SetTable(table types.TableName) {
	c.table = table.Name()
}

func (c *CommonFields) GetTableFunc(f GetTableFunc) {
	c.getTableFunc = f
}

func (c *CommonFields) SetProvider(p SqlProviderAchieve) {
	c.provider = p
}

type Master interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func (c *CommonFields) GetMaster(ctx context.Context) Master {
	if ctx == nil {
		return c.provider.GetMaster()
	}

	tx := c.provider.GetTxFromCtx(ctx)
	if tx != nil {
		return tx
	}

	return &dbWithContext{
		db:  c.provider.GetMaster(),
		ctx: ctx,
	}
}

type Replica interface {
	Get(dest interface{}, query string, args ...interface{}) error
	Select(dest interface{}, query string, args ...interface{}) error
	Queryx(query string, args ...interface{}) (*sqlx.Rows, error)
	QueryRowx(query string, args ...interface{}) *sqlx.Row
}

type dbWithContext struct {
	db  *sqlx.DB
	ctx context.Context
}

func (d *dbWithContext) Get(dest interface{}, query string, args ...interface{}) error {
	return d.db.GetContext(d.ctx, dest, query, args...)
}

func (d *dbWithContext) Queryx(query string, args ...interface{}) (*sqlx.Rows, error) {
	return d.db.QueryxContext(d.ctx, query, args...)
}

func (d *dbWithContext) QueryRowx(query string, args ...interface{}) *sqlx.Row {
	return d.db.QueryRowxContext(d.ctx, query, args...)
}

func (d *dbWithContext) Select(dest interface{}, query string, args ...interface{}) error {
	return d.db.SelectContext(d.ctx, dest, query, args...)
}

func (d *dbWithContext) Exec(query string, args ...interface{}) (sql.Result, error) {
	return d.db.ExecContext(d.ctx, query, args...)
}

func (c *CommonFields) GetReplica(ctx context.Context) Replica {
	if ctx == nil {
		return c.provider.GetReplica()
	}

	tx := c.provider.GetTxFromCtx(ctx)
	if tx != nil {
		return tx
	}

	return &dbWithContext{
		db:  c.provider.GetReplica(),
		ctx: ctx,
	}
}
