package prefix

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/celestix/gotgproto/ext"
	dbClient "github.com/hikari-work/userbot/connection"
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
		_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "Format salah! Gunakan: <code>prefix &lt;simbol&gt;</code> (contoh: <code>prefix .</code> atau <code>prefix !</code>)")
		return err
	}

	newPrefix := strings.TrimSpace(args[1])
	if len(newPrefix) != 1 {
		_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "Prefix harus berupa <b>satu karakter/simbol</b>!")
		return err
	}

	ctxBg := context.Background()
	err := dbClient.Redis.Set(ctxBg, "prefix", newPrefix, 0).Err()
	if err != nil {
		slog.Error("Gagal mengubah prefix di Redis", "error", err)
		_, editErr := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "Gagal mengubah prefix di Redis: <i>"+err.Error()+"</i>")
		return editErr
	}

	if dbClient.UpdatePrefixFunc != nil {
		dbClient.UpdatePrefixFunc(newPrefix)
	}

	slog.Info("Prefix berhasil diubah di Redis", "new_prefix", newPrefix)
	_, err = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("Prefix berhasil diubah menjadi <code>%s</code> secara <b>real-time</b> (tanpa restart)!", newPrefix))
	return err
}
