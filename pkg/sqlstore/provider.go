package sqlstore

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"github.com/quka-ai/quka-ai/pkg/utils"
)

type SqlCommons interface {
	GetTable(...interface{}) string
}

type ConnectConfig interface {
	FormatDSN() string
}

type SqlProvider struct {
	master   *sqlx.DB
	replicas []*sqlx.DB
	dbname   string
}

func (s *SqlProvider) GetTxFromCtx(ctx context.Context) *sqlx.Tx {
	if driver, ok := ctx.Value(TransactionKey{}).(*sqlx.Tx); ok {
		return driver
	}
	return nil
}

func (s *SqlProvider) GetMaster() *sqlx.DB {
	return s.master
}

func (s *SqlProvider) GetReplica() *sqlx.DB {
	return s.replicas[utils.Random(0, len(s.replicas)-1)]
}

type TransactionKey struct{}

func (s *SqlProvider) Transaction(ctx context.Context, next func(ctx context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if _, ok := ctx.Value(TransactionKey{}).(*sql.Tx); ok {
		return next(ctx)
	}

	var (
		tx  *sqlx.Tx
		err error
	)
	if tx, err = s.GetMaster().BeginTxx(ctx, nil); err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil || err != nil {
			slog.Error("Transaction rollbacked", slog.Any("recover", r), slog.String("error", err.Error()))
			_ = tx.Rollback()
			return
		}
	}()

	if err = next(context.WithValue(ctx, TransactionKey{}, tx)); err != nil {
		return err
	}

	return tx.Commit()
}

type Stores struct{}

// 建立数据库连接
func (s *SqlProvider) initConnection(conf ConnectConfig) (*sqlx.DB, error) {
	var (
		engine *sqlx.DB
	)

	engine = sqlx.MustOpen("postgres", conf.FormatDSN())

	return engine, nil
}

func MustSetupProvider(m ConnectConfig, s ...ConnectConfig) *SqlProvider {
	var (
		err      error
		engine   *sqlx.DB
		slaves   []*sqlx.DB
		provider = &SqlProvider{}
	)

	if engine, err = provider.initConnection(m); err != nil {
		panic(err)
	}

	if len(s) == 0 {
		s = append(s, m)
	}

	for _, v := range s {
		slave, err := provider.initConnection(v)
		if err != nil {
			panic(err)
		}
		slaves = append(slaves, slave)
	}

	provider.master = engine

	if len(slaves) == 0 {
		slaves = append(slaves, engine)
	}
	provider.replicas = append(provider.replicas, slaves...)

	return provider
}

func (s *SqlProvider) GetTx() (*sqlx.Tx, error) {
	return s.GetMaster().Beginx()
}

func (s *SqlProvider) GetDBName() (string, error) {
	if s.dbname == "" {
		// 获取当前使用的数据库名
		var dbName string
		err := s.GetMaster().QueryRow("SELECT DATABASE()").Scan(&dbName)
		if err != nil {
			return "", err
		}
		s.dbname = dbName
	}

	return s.dbname, nil
}
