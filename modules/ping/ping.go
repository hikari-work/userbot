package ping

import (
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/hikari-work/userbot/i18n"
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
	since := time.Since(now)
	text := i18n.Localize("PongMessage", map[string]interface{}{
		"Since": since.String(),
	}, nil)
	_, err := ctx.Reply(update, ext.ReplyTextString(text), nil)
	return err
}
