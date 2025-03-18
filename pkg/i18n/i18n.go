package i18n

import (
	"embed"
	"log/slog"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	//go:embed *.toml
	f embed.FS
)

type Localizer struct {
	bundle   *i18n.Bundle
	registry map[string]*i18n.Localizer
}

var localizer Localizer

func NewLocalizer(languages ...string) Localizer {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	for _, lang := range languages {
		path := lang + ".toml"
		if _, err := bundle.LoadMessageFileFS(f, path); err != nil {
			slog.Error("Failed to load i18n message config", slog.String("error", err.Error()), slog.String("lang", lang), slog.String("file", path))
		}
	}

	l := Localizer{
		bundle:   bundle,
		registry: make(map[string]*i18n.Localizer),
	}
	for _, lang := range languages {
		l.registry[lang] = i18n.NewLocalizer(l.bundle, lang)
	}
	return l
}

func (l Localizer) Get(lang string, id string) string {
	localizer := l.registry[lang]
	if localizer == nil {
		return id
	}

	cfg := &i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    id,
			Other: id,
			One:   id,
		},
	}

	str, err := localizer.Localize(cfg)
	if err != nil {
		slog.Info("failed to get localizer message", slog.String("message", "Get"), slog.String("id", id), slog.String("error", err.Error()))
		return id
	}

	return str
}

func (l Localizer) GetWithData(lang, id string, data map[string]interface{}) string {
	localizer := l.registry[lang]
	if localizer == nil {
		return id
	}
	cfg := &i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    id,
			Other: id,
		},
		TemplateData: data,
	}
	str, err := localizer.Localize(cfg)
	if err != nil {
		slog.Info("failed to get localizer message", slog.String("message", "GetWithData"), slog.String("id", id), slog.String("error", err.Error()))
		return id
	}

	return str
}
