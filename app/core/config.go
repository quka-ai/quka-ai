package core

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/quka-ai/quka-ai/app/core/srv"
)

func MustLoadBaseConfig(path string) CoreConfig {
	if path == "" {
		return LoadBaseConfigFromENV()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	conf := &CoreConfig{}
	conf.SetConfigBytes(raw)

	if err = toml.Unmarshal(raw, conf); err != nil {
		panic(err)
	}

	return *conf
}

func (c CoreConfig) LoadCustomConfig(cfg any) error {
	if len(c.bytes) == 0 {
		return nil
	}
	if err := toml.Unmarshal(c.bytes, cfg); err != nil {
		return err
	}
	return nil
}

type CustomConfig[T any] struct {
	CustomConfig T `toml:"custom_config"`
}

func NewCustomConfigPayload[T any]() CustomConfig[T] {
	return CustomConfig[T]{}
}

func LoadBaseConfigFromENV() CoreConfig {
	var c CoreConfig
	c.FromENV()
	return c
}

type CoreConfig struct {
	Addr          string              `toml:"addr"`
	Log           Log                 `toml:"log"`
	Postgres      PGConfig            `toml:"postgres"`
	Redis         RedisConfig         `toml:"redis"`
	Site          Site                `toml:"site"`
	ObjectStorage ObjectStorageDriver `toml:"object_storage"`

	AI         srv.AIConfig     `toml:"ai"`
	Centrifuge CentrifugeConfig `toml:"centrifuge"`

	Security Security `toml:"security"`

	Prompt Prompt `toml:"prompt"`

	Semaphore SemaphoreConfig `toml:"semaphore"`

	bytes []byte `toml:"-"`
}

type ObjectStorageDriver struct {
	StaticDomain string    `toml:"static_domain"`
	Driver       string    `toml:"driver"`
	S3           *S3Config `toml:"s3"`
}

type S3Config struct {
	Bucket       string `toml:"bucket"`
	Region       string `toml:"region"`
	Endpoint     string `toml:"endpoint"`
	AccessKey    string `toml:"access_key"`
	SecretKey    string `toml:"secret_key"`
	UsePathStyle bool   `toml:"use_path_style"`
}

type Site struct {
	DefaultAvatar string      `toml:"default_avatar"`
	Share         ShareConfig `toml:"share"`
}

type ShareConfig struct {
	EmbeddingDomain string `toml:"embedding_domain"`
	Domain          string `toml:"domain"`
	SiteTitle       string `toml:"site_title"`
	SiteDescription string `toml:"site_description"`
}

func (c *CoreConfig) SetConfigBytes(raw []byte) {
	c.bytes = raw
}

// Prompt 配置结构
// 用于自定义系统中各种场景下使用的 prompt
type Prompt struct {
	Base         string `toml:"base"`          // 全局头部 Prompt，为空则使用系统默认
	Query        string `toml:"query"`         // 查询 Prompt（已废弃，保留用于向后兼容）
	ChatSummary  string `toml:"chat_summary"`  // 聊天总结 Prompt，为空则使用系统默认
	EnhanceQuery string `toml:"enhance_query"` // 查询增强 Prompt，为空则使用系统默认
	SessionName  string `toml:"session_name"`  // 会话命名 Prompt，为空则使用系统默认
}

type CentrifugeConfig struct {
	MaxConnections    int      `toml:"max_connections"`
	HeartbeatInterval int      `toml:"heartbeat_interval"`
	DeploymentMode    string   `toml:"deployment_mode"`
	RedisURL          string   `toml:"redis_url"`
	RedisCluster      bool     `toml:"redis_cluster"`
	EnablePresence    bool     `toml:"enable_presence"`
	EnableHistory     bool     `toml:"enable_history"`
	EnableRecovery    bool     `toml:"enable_recovery"`
	AllowedOrigins    []string `toml:"allowed_origins"`
	MaxChannelLength  int      `toml:"max_channel_length"`
	MaxMessageSize    int      `toml:"max_message_size"`
}

type Security struct {
	EncryptKey string `json:"encrypt_key"`
}

type SemaphoreConfig struct {
	Knowledge KnowledgeSemaphoreConfig `toml:"knowledge"`
}

type KnowledgeSemaphoreConfig struct {
	SummaryMaxConcurrency int `toml:"summary_max_concurrency"` // 知识总结最大并发数，默认 10
}

func (c *CoreConfig) FromENV() {
	c.Addr = os.Getenv("QUKA_API_SERVICE_ADDRESS")
	c.Log.FromENV()
	c.Postgres.FromENV()
	c.Redis.FromENV()
}

type PGConfig struct {
	DSN string `toml:"dsn"`
}

func (m *PGConfig) FromENV() {
	m.DSN = os.Getenv("QUKA_API_POSTGRESQL_DSN")
}

func (c PGConfig) FormatDSN() string {
	return c.DSN
}

type RedisConfig struct {
	// 单机模式配置
	Addr     string `toml:"addr"`     // Redis地址，格式: host:port
	Password string `toml:"password"` // Redis密码
	DB       int    `toml:"db"`       // Redis数据库索引 (0-15)

	// 集群模式配置
	Cluster       bool     `toml:"cluster"`        // 是否启用集群模式
	ClusterAddrs  []string `toml:"cluster_addrs"`  // 集群节点地址列表
	ClusterPasswd string   `toml:"cluster_passwd"` // 集群密码

	// 连接池配置
	PoolSize     int `toml:"pool_size"`      // 连接池大小，默认10
	MinIdleConns int `toml:"min_idle_conns"` // 最小空闲连接数，默认0
	MaxRetries   int `toml:"max_retries"`    // 最大重试次数，默认3
	DialTimeout  int `toml:"dial_timeout"`   // 连接超时(秒)，默认5
	ReadTimeout  int `toml:"read_timeout"`   // 读超时(秒)，默认3
	WriteTimeout int `toml:"write_timeout"`  // 写超时(秒)，默认3

	// 队列配置
	KeyPrefix string `toml:"key_prefix"` // Redis键前缀，用于隔离不同环境/应用
}

func (r *RedisConfig) FromENV() {
	r.Addr = os.Getenv("QUKA_REDIS_ADDR")
	r.Password = os.Getenv("QUKA_REDIS_PASSWORD")
	if dbStr := os.Getenv("QUKA_REDIS_DB"); dbStr != "" {
		if db, err := strconv.Atoi(dbStr); err == nil {
			r.DB = db
		}
	}
}

type Log struct {
	Level string `toml:"level"`
	Path  string `toml:"path"`
}

func (l *Log) FromENV() {
	l.Level = os.Getenv("QUKA_API_LOG_LEVEL")
	l.Path = os.Getenv("QUKA_API_LOG_PATH")
}

func (l *Log) SlogLevel() slog.Level {
	switch strings.ToLower(l.Level) {
	case "info":
		return slog.LevelInfo
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelDebug
	}
}
