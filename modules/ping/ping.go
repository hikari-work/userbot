package ping

import (
	"fmt"
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
	start := time.Now()
	_, err := ctx.Raw.HelpGetConfig(ctx)
	if err != nil {
		return err
	}
	since := time.Since(start)
	
	text := i18n.Localize("PongMessage", map[string]interface{}{
		"Since": fmt.Sprintf("%.2fms", float64(since.Microseconds())/1000.0),
	}, nil)
	_, err = ctx.Reply(update, ext.ReplyTextString(text), nil)
	return err
}
