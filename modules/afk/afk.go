package afk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	dbClient "github.com/hikari-work/userbot/connection"
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
	})
}

func afkCommandHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage
	afkReason := strings.Join(update.Args(), " ")
	if afkReason == "" {
		afkReason = "I'll be back!"
	}
	afkTimeStr := time.Now().Format(time.RFC3339)

	ctxBg := context.Background()
	err := dbClient.Redis.HSet(ctxBg, "userbot:afk", map[string]interface{}{
		"active": "true",
		"reason": afkReason,
		"time":   afkTimeStr,
	}).Err()
	if err != nil {

		_, err := ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			Message: "Failed activate AFK : " + err.Error(),
			ID:      uMsg.ID,
		})
		return err
	}

	_, err = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
		Message: "AFK Successfully Activated ",
		ID:      uMsg.ID,
	})
	return err
}

func afkMessageHook(ctx *ext.Context, update *ext.Update) error {
	m := update.EffectiveMessage
	if m == nil {
		return nil
	}

	ctxBg := context.Background()

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

		_, _ = ctx.Reply(update, ext.ReplyTextString(fmt.Sprintf("I'm back! I've been AFK for %s", durationStr)), nil)
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

		_, _ = ctx.Reply(update, ext.ReplyTextString(fmt.Sprintf("Sorry, i'm AFK (%s). Since %s", afkData["reason"], timeStr)), nil)
	}

	return nil
}
