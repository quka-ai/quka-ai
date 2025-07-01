package core

import (
	"log/slog"
	"os"
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
	Addr     string   `toml:"addr"`
	Log      Log      `toml:"log"`
	Postgres PGConfig `toml:"postgres"`
	Site     Site     `toml:"site"`

	AI srv.AIConfig `toml:"ai"`

	Security Security `toml:"security"`

	Prompt Prompt `toml:"prompt"`

	bytes []byte `toml:"-"`
}

type Site struct {
	DefaultAvatar string      `toml:"default_avatar"`
	Share         ShareConfig `toml:"share"`
}

type ShareConfig struct {
	Domain          string `toml:"domain"`
	SiteTitle       string `toml:"site_title"`
	SiteDescription string `toml:"site_description"`
}

func (c *CoreConfig) SetConfigBytes(raw []byte) {
	c.bytes = raw
}

type Prompt struct {
	Base         string `toml:"base"`
	Query        string `toml:"query"`
	ChatSummary  string `toml:"chat_summary"`
	EnhanceQuery string `toml:"enhance_query"`
	SessionName  string `toml:"session_name"`
}

type Security struct {
	EncryptKey string `json:"encrypt_key"`
}

func (c *CoreConfig) FromENV() {
	c.Addr = os.Getenv("QUKA_API_SERVICE_ADDRESS")
	c.Log.FromENV()
	c.Postgres.FromENV()
	c.AI.FromENV()
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
