package ping

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/hikari-work/userbot/modules/manager"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "Ping",
		Description: "Mengecek Apakah Userbot Merespon",
		Commands:    []string{"ping", "p"},
		OnlyOut:     true,
		Handler:     pingHandler,
	})
}
func pingHandler(ctx *ext.Context, update *ext.Update) error {
	now := time.Now()
	slog.Info("Getting Ping")
	slog.Info("Sending Ping Reposnse",
		"sender_id", update.EffectiveUser().ID,
		"chat_id", update.EffectiveChat().GetID(),
	)
	since := time.Since(now)
	_, err := ctx.Reply(update, ext.ReplyTextString(fmt.Sprintf("Pong %s", since)), nil)
	return err
}
