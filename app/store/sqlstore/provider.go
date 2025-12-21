package sqlstore

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/lib/pq"

	"github.com/quka-ai/quka-ai/app/store"
	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/quka-ai/quka-ai/pkg/sqlstore"
	"github.com/quka-ai/quka-ai/pkg/types"
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
	stores  *Stores
	coreRef *CoreRef
}

// CoreRef 用于延迟获取 core 实例，避免循环依赖
type CoreRef struct {
	getCacheFunc func() types.Cache
}

type Stores struct {
	store.KnowledgeStore
	store.KnowledgeChunkStore
	store.VectorStore
	store.AccessTokenStore
	store.UserSpaceStore
	store.UserGlobalRoleStore
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
	store.SpaceInvitationStore
	store.ContentTaskStore
	store.KnowledgeMetaStore
	store.KnowledgeRelMetaStore
	store.RSSSubscriptionStore
	store.RSSArticleStore
	store.RSSUserInterestStore
	store.RSSDailyDigestStore
	store.PodcastStore
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
CREATE TABLE IF NOT EXISTS ` + types.TABLE_PREFIX + `schema_migrations (
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
		"SELECT COUNT(*) FROM "+types.TABLE_PREFIX+"schema_migrations WHERE filename = $1", filename)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// markFileExecuted 标记文件为已执行
func (p *Provider) markFileExecuted(filename string) error {
	_, err := p.SqlProvider.GetMaster().Exec(
		"INSERT INTO "+types.TABLE_PREFIX+"schema_migrations (filename, executed_at) VALUES ($1, $2) ON CONFLICT (filename) DO NOTHING",
		filename, time.Now().Unix())
	return err
}

// executeSQLFile 执行SQL文件内容，分割语句并逐个执行
func (p *Provider) executeSQLFile(content, filename string) error {
	fmt.Println("executeSQLFile", content)
	// 执行语句
	if _, err := p.SqlProvider.GetMaster().Exec(content); err != nil {
		return err
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

func (p *Provider) UserGlobalRoleStore() store.UserGlobalRoleStore {
	return p.stores.UserGlobalRoleStore
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

func (p *Provider) SpaceInvitationStore() store.SpaceInvitationStore {
	return p.stores.SpaceInvitationStore
}

func (p *Provider) ContentTaskStore() store.ContentTaskStore {
	return p.stores.ContentTaskStore
}

func (p *Provider) KnowledgeMetaStore() store.KnowledgeMetaStore {
	return p.stores.KnowledgeMetaStore
}

func (p *Provider) KnowledgeRelMetaStore() store.KnowledgeRelMetaStore {
	return p.stores.KnowledgeRelMetaStore
}

func (p *Provider) RSSSubscriptionStore() store.RSSSubscriptionStore {
	return p.stores.RSSSubscriptionStore
}

func (p *Provider) RSSArticleStore() store.RSSArticleStore {
	return p.stores.RSSArticleStore
}

func (p *Provider) RSSUserInterestStore() store.RSSUserInterestStore {
	return p.stores.RSSUserInterestStore
}

func (p *Provider) RSSDailyDigestStore() store.RSSDailyDigestStore {
	return p.stores.RSSDailyDigestStore
}

func (p *Provider) PodcastStore() store.PodcastStore {
	return p.stores.PodcastStore
}

// Cache 实现 Author 接口的 Cache 方法
func (p *Provider) Cache() types.Cache {
	if p.coreRef != nil && p.coreRef.getCacheFunc != nil {
		return p.coreRef.getCacheFunc()
	}
	// 返回一个空的 cache 实现作为fallback
	return &EmptyCache{}
}

// SetCacheFunc 设置获取 cache 的函数
func (p *Provider) SetCacheFunc(getCacheFunc func() types.Cache) {
	if p.coreRef == nil {
		p.coreRef = &CoreRef{}
	}
	p.coreRef.getCacheFunc = getCacheFunc
}

// EmptyCache 空的 cache 实现，用作 fallback
type EmptyCache struct{}

func (c *EmptyCache) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (c *EmptyCache) SetEx(ctx context.Context, key, value string, expiresAt time.Duration) error {
	return nil
}

func (c *EmptyCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return nil
}
