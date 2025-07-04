package sqlstore

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/lib/pq"

	"github.com/quka-ai/quka-ai/app/store"
	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/sqlstore"
)

func init() {
	sq.StatementBuilder = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
}

var provider = &Provider{
	stores: &Stores{},
}

func GetProvider() *Provider {
	return provider
}

type Provider struct {
	*sqlstore.SqlProvider
	stores *Stores
}

type Stores struct {
	store.KnowledgeStore
	store.KnowledgeChunkStore
	store.VectorStore
	store.AccessTokenStore
	store.UserSpaceStore
	store.SpaceStore
	store.ResourceStore
	store.UserStore
	store.ChatSessionStore
	store.ChatSessionPinStore
	store.ChatMessageStore
	store.ChatSummaryStore
	store.ChatMessageExtStore
	store.FileManagementStore
	store.AITokenUsageStore
	store.ShareTokenStore
	store.SpaceApplicationStore
	store.JournalStore
	store.ButlerTableStore
	store.ModelProviderStore
	store.ModelConfigStore
	store.CustomConfigStore
}

func (s *Provider) batchExecStoreFuncs(fname string) {
	val := reflect.ValueOf(s.stores)
	num := val.NumField()
	for i := 0; i < num; i++ {
		val.Field(i).MethodByName(fname).Call([]reflect.Value{})
	}
}

type RegisterKey struct{}

func MustSetup(m sqlstore.ConnectConfig, s ...sqlstore.ConnectConfig) func() *Provider {

	provider.SqlProvider = sqlstore.MustSetupProvider(m, s...)

	for _, f := range register.ResolveFuncHandlers[*Provider](RegisterKey{}) {
		f(provider)
	}

	return func() *Provider {
		return provider
	}
}

// Install 初始化所有数据表
func (p *Provider) Install() error {
	// 首先启用必要的数据库扩展
	if err := p.enableExtensions(); err != nil {
		return err
	}

	// 确保迁移记录表存在
	if err := p.ensureMigrationTable(); err != nil {
		return err
	}

	// 获取所有SQL文件
	files, err := CreateTableFiles.ReadDir(".")
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
			// 检查文件是否已经执行过
			if executed, err := p.isFileExecuted(file.Name()); err != nil {
				return err
			} else if executed {
				continue // 跳过已执行的文件
			}

			sql, err := CreateTableFiles.ReadFile(file.Name())
			if err != nil {
				return err
			}

			// 执行SQL文件内容
			if err = p.executeSQLFile(string(sql), file.Name()); err != nil {
				return err
			}

			// 记录文件已执行
			if err = p.markFileExecuted(file.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}

// enableExtensions 启用必要的数据库扩展
// 如需添加更多扩展，只需在 extensions 切片中添加相应的 SQL 语句
func (p *Provider) enableExtensions() error {
	extensions := []string{
		"CREATE EXTENSION IF NOT EXISTS vector;", // pgvector 扩展，用于向量操作
		// 可以在这里添加更多扩展，例如：
		// "CREATE EXTENSION IF NOT EXISTS uuid-ossp;", // UUID 生成功能
		// "CREATE EXTENSION IF NOT EXISTS pg_trgm;",   // 模糊字符串匹配
	}

	for _, ext := range extensions {
		if _, err := p.SqlProvider.GetMaster().Exec(ext); err != nil {
			return fmt.Errorf("failed to enable extension: %w\nSQL: %s", err, ext)
		}
	}
	return nil
}

// ensureMigrationTable 确保迁移记录表存在
func (p *Provider) ensureMigrationTable() error {
	createTableSQL := `
CREATE TABLE IF NOT EXISTS bw_schema_migrations (
    filename VARCHAR(255) PRIMARY KEY,
    executed_at BIGINT NOT NULL
);`
	_, err := p.SqlProvider.GetMaster().Exec(createTableSQL)
	return err
}

// isFileExecuted 检查文件是否已经执行过
func (p *Provider) isFileExecuted(filename string) (bool, error) {
	var count int
	err := p.SqlProvider.GetReplica().Get(&count,
		"SELECT COUNT(*) FROM bw_schema_migrations WHERE filename = $1", filename)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// markFileExecuted 标记文件为已执行
func (p *Provider) markFileExecuted(filename string) error {
	_, err := p.SqlProvider.GetMaster().Exec(
		"INSERT INTO bw_schema_migrations (filename, executed_at) VALUES ($1, $2) ON CONFLICT (filename) DO NOTHING",
		filename, time.Now().Unix())
	return err
}

// executeSQLFile 执行SQL文件内容，分割语句并逐个执行
func (p *Provider) executeSQLFile(content, filename string) error {
	// 分割SQL语句（以分号分隔）
	statements := strings.Split(content, ";")

	for i, stmt := range statements {
		// 清理语句，去除空白和注释
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		// 执行语句
		if _, err := p.SqlProvider.GetMaster().Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute statement %d in file %s: %w\nSQL: %s", i+1, filename, err, stmt)
		}
	}
	return nil
}

func (p *Provider) store() *Stores {
	return p.stores
}

func (p *Provider) KnowledgeStore() store.KnowledgeStore {
	return p.stores.KnowledgeStore
}

func (p *Provider) VectorStore() store.VectorStore {
	return p.stores.VectorStore
}

func (p *Provider) AccessTokenStore() store.AccessTokenStore {
	return p.stores.AccessTokenStore
}

func (p *Provider) UserSpaceStore() store.UserSpaceStore {
	return p.stores.UserSpaceStore
}

func (p *Provider) SpaceStore() store.SpaceStore {
	return p.stores.SpaceStore
}

func (p *Provider) ResourceStore() store.ResourceStore {
	return p.stores.ResourceStore
}

func (p *Provider) UserStore() store.UserStore {
	return p.stores.UserStore
}

func (p *Provider) KnowledgeChunkStore() store.KnowledgeChunkStore {
	return p.stores.KnowledgeChunkStore
}

func (p *Provider) ChatSessionStore() store.ChatSessionStore {
	return p.stores.ChatSessionStore
}

func (p *Provider) ChatMessageStore() store.ChatMessageStore {
	return p.stores.ChatMessageStore
}

func (p *Provider) ChatSummaryStore() store.ChatSummaryStore {
	return p.stores.ChatSummaryStore
}

func (p *Provider) ChatMessageExtStore() store.ChatMessageExtStore {
	return p.stores.ChatMessageExtStore
}

func (p *Provider) FileManagementStore() store.FileManagementStore {
	return p.stores.FileManagementStore
}

func (p *Provider) AITokenUsageStore() store.AITokenUsageStore {
	return p.stores.AITokenUsageStore
}

func (p *Provider) ShareTokenStore() store.ShareTokenStore {
	return p.stores.ShareTokenStore
}

func (p *Provider) JournalStore() store.JournalStore {
	return p.stores.JournalStore
}

func (p *Provider) ChatSessionPinStore() store.ChatSessionPinStore {
	return p.stores.ChatSessionPinStore
}

func (p *Provider) BulterTableStore() store.ButlerTableStore {
	return p.stores.ButlerTableStore
}

func (p *Provider) SpaceApplicationStore() store.SpaceApplicationStore {
	return p.stores.SpaceApplicationStore
}

func (p *Provider) ModelProviderStore() store.ModelProviderStore {
	return p.stores.ModelProviderStore
}

func (p *Provider) ModelConfigStore() store.ModelConfigStore {
	return p.stores.ModelConfigStore
}

func (p *Provider) CustomConfigStore() store.CustomConfigStore {
	return p.stores.CustomConfigStore
}
