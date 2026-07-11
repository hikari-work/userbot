package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/redis/go-redis/v9"
	"golang.org/x/text/language"

	dbClient "github.com/hikari-work/userbot/connection"
)

//go:embed active.*.json
var translationFS embed.FS

var bundle *i18n.Bundle

func init() {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	_, err := bundle.LoadMessageFileFS(translationFS, "active.en.json")
	if err != nil {
		slog.Error("i18n: failed to load active.en.json", "error", err)
	}
	_, err = bundle.LoadMessageFileFS(translationFS, "active.id.json")
	if err != nil {
		slog.Error("i18n: failed to load active.id.json", "error", err)
	}
}

func GetLanguage() string {
	if dbClient.Redis == nil {
		return "id"
	}
	lang, err := dbClient.Redis.Get(context.Background(), "language").Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			slog.Error("i18n: failed to get language from Redis", "error", err)
		}
		return "id"
	}
	if lang == "" {
		return "id"
	}
	return lang
}

func SetLanguage(ctx context.Context, lang string) error {
	if dbClient.Redis == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	return dbClient.Redis.Set(ctx, "language", lang, 0).Err()
}
func GetAllAvailLanguage() []string {
	var langString []string
	for _, lang := range bundle.LanguageTags() {
		langString = append(langString, lang.String())
	}
	return langString
}

func Localize(messageID string, templateData map[string]interface{}, pluralCount interface{}) string {
	lang := GetLanguage()
	localizer := i18n.NewLocalizer(bundle, lang)

	config := &i18n.LocalizeConfig{
		MessageID: messageID,
	}
	if templateData != nil {
		config.TemplateData = templateData
	}
	if pluralCount != nil {
		config.PluralCount = pluralCount
	}

	result, err := localizer.Localize(config)
	if err != nil {
		fallbackLocalizer := i18n.NewLocalizer(bundle, "en")
		result, err = fallbackLocalizer.Localize(config)
		if err != nil {
			return messageID
		}
	}
	return result
}
