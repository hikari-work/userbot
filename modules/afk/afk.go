package afk

import (
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "afk",
		Description: "Activate Away From Keyboard (AFK) mode",
		Commands:    []string{"afk"},
		OnlyOut:     true,
		Handler:     afkCommandHandler,
		OnMessage:   afkMessageHook,
		Help:        afkHelp,
	})
}

func afkHelp() string {
	return "Format \n<code>.afk &lt;alasan&gt;</code>\n<code>Contoh : .afk makan</code>"
}

func afkCommandHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage
	afkReason := strings.Join(update.Args(), " ")
	if afkReason == "" {
		afkReason = i18n.Localize("AFKDefaultReason", nil, nil)
	}
	afkTimeStr := time.Now().Format(time.RFC3339)

	ctxBg := ctx
	err := dbClient.Redis.HSet(ctxBg, "userbot:afk", map[string]interface{}{
		"active": "true",
		"reason": afkReason,
		"time":   afkTimeStr,
	}).Err()
	if err != nil {

		_, err := ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			Message: i18n.Localize("AFKFailed", map[string]interface{}{"Error": err.Error()}, nil),
			ID:      uMsg.ID,
		})
		return err
	}

	_, err = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
		Message: i18n.Localize("AFKSuccess", nil, nil),
		ID:      uMsg.ID,
	})
	return err
}

func afkMessageHook(ctx *ext.Context, update *ext.Update) error {
	m := update.EffectiveMessage
	if m == nil {
		return nil
	}

	ctxBg := ctx

	afkData, err := dbClient.Redis.HGetAll(ctxBg, "userbot:afk").Result()
	if err != nil || len(afkData) == 0 || afkData["active"] != "true" {
		return nil
	}

	if m.Out {
		dbClient.Redis.Del(ctxBg, "userbot:afk")

		parsedTime, parseErr := time.Parse(time.RFC3339, afkData["time"])
		var durationStr string
		if parseErr == nil {
			durationStr = time.Since(parsedTime).Round(time.Second).String()
		} else {
			durationStr = "a moment"
		}

		_, _ = ctx.Reply(update, ext.ReplyTextString(i18n.Localize("AFKBack", map[string]interface{}{"Duration": durationStr}, nil)), nil)
		return nil
	}

	_, isPrivate := m.PeerID.(*tg.PeerUser)

	if m.Mentioned || isPrivate {
		parsedTime, parseErr := time.Parse(time.RFC3339, afkData["time"])
		var timeStr string
		if parseErr == nil {
			timeStr = parsedTime.Format("15:04:05")
		} else {
			timeStr = "a moment ago"
		}

		_, _ = ctx.Reply(update, ext.ReplyTextString(i18n.Localize("AFKStatus", map[string]interface{}{"Reason": afkData["reason"], "Time": timeStr}, nil)), nil)
	}

	return nil
}
