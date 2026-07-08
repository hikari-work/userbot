package prefix

import (
	"log/slog"
	"strings"

	"github.com/celestix/gotgproto/ext"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "Prefix",
		Description: "Mengubah prefix command userbot secara dinamis",
		Commands:    []string{"prefix", "pre"},
		OnlyOut:     true,
		Handler:     handlerPrefix,
	})
}

func handlerPrefix(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage
	if uMsg == nil || uChat == nil {
		return nil
	}

	args := update.Args()
	if len(args) < 2 {
		_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize(ctx, "PrefixUsage", nil, nil))
		return err
	}

	newPrefix := strings.TrimSpace(args[1])
	if len(newPrefix) != 1 {
		_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize(ctx, "PrefixLengthError", nil, nil))
		return err
	}

	ctxBg := ctx
	err := dbClient.Redis.Set(ctxBg, "prefix", newPrefix, 0).Err()
	if err != nil {
		slog.Error("Gagal mengubah prefix di Redis", "error", err)
		_, editErr := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize(ctx, "PrefixFailedRedis", map[string]interface{}{"Error": err.Error()}, nil))
		return editErr
	}

	if dbClient.UpdatePrefixFunc != nil {
		dbClient.UpdatePrefixFunc(newPrefix)
	}

	slog.Info("Prefix berhasil diubah di Redis", "new_prefix", newPrefix)
	_, err = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize(ctx, "PrefixSuccess", map[string]interface{}{"Prefix": newPrefix}, nil))
	return err
}
