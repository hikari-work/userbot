package filter

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:            "Filter",
		Description:     "Auto reply to specified keywords in chat",
		Commands:        []string{"filter", "stop", "filters"},
		OnlyOut:         true,
		Handler:         filterCommandHandler,
		OnMessage:       filterMessageHook,
		CallbackPrefix:  "",
		CallbackHandler: nil,
		InlineHandler:   nil,
	})
}

func filterCommandHandler(ctx *ext.Context, update *ext.Update) error {
	uMsg := update.EffectiveMessage
	uChat := update.EffectiveChat()
	if uMsg == nil || uChat == nil {
		return nil
	}

	if uChat.IsAUser() {
		localize := i18n.Localize("FilterInPrivate", nil, nil)
		_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
		return err
	}

	args := update.Args()
	if len(args) == 0 {
		_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("FilterUsage", nil, nil))
		return err
	}

	cmdName := ""
	for _, c := range []string{"filter", "stop", "filters"} {
		if strings.HasSuffix(strings.ToLower(args[0]), c) {
			cmdName = c
			break
		}
	}

	switch cmdName {
	case "filter":
		if len(args) < 2 {
			_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("FilterUsage", nil, nil))
			return err
		}
		trigger := strings.ToLower(args[1])
		var response string

		if len(args) <= 2 {
			replyHeader, ok := uMsg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || replyHeader.ReplyToMsgID == 0 {
				_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("FilterUsage", nil, nil))
				return err
			}
			msgs, err := ctx.GetMessages(uChat.GetID(), []tg.InputMessageClass{&tg.InputMessageID{
				ID: replyHeader.ReplyToMsgID,
			}})
			if err != nil || len(msgs) == 0 {
				_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("FilterUsage", nil, nil))
				return err
			}
			repMsg, ok := msgs[0].(*tg.Message)
			if !ok || repMsg.Message == "" {
				_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "❌ <b>Error:</b> Pesan balasan harus memiliki teks atau caption.")
				return err
			}
			response = repMsg.Message
		} else {
			response = strings.Join(args[2:], " ")
		}

		err := setFilter(ctx, uChat.GetID(), trigger, response)
		if err != nil {
			localize := i18n.Localize("FilterFailedSave", map[string]interface{}{"Error": err.Error()}, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return err
		}

		localize := i18n.Localize("FilterSuccess", map[string]interface{}{"Trigger": trigger}, nil)
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
		return nil

	case "stop":
		if len(args) < 2 {
			_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("FilterStopUsage", nil, nil))
			return err
		}
		trigger := strings.ToLower(args[1])
		deleted, err := deleteFilter(ctx, uChat.GetID(), trigger)
		if err != nil {
			localize := i18n.Localize("FilterStopFailed", map[string]interface{}{"Error": err.Error()}, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return err
		}
		if deleted == 0 {
			localize := i18n.Localize("FilterStopNotFound", map[string]interface{}{"Trigger": trigger}, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return nil
		}
		localize := i18n.Localize("FilterStopSuccess", map[string]interface{}{"Trigger": trigger}, nil)
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
		return nil

	case "filters":
		filters, err := getFilters(ctx, uChat.GetID())
		if err != nil {
			localize := i18n.Localize("FiltersFailedRetrieve", map[string]interface{}{"Error": err.Error()}, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return err
		}
		if len(filters) == 0 {
			localize := i18n.Localize("FiltersEmpty", nil, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return nil
		}
		var sb strings.Builder
		sb.WriteString(i18n.Localize("FiltersListHeader", nil, nil))
		i := 1
		for trig := range filters {
			sb.WriteString(fmt.Sprintf("%d. <code>%s</code>\n", i, html.EscapeString(trig)))
			i++
		}
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, sb.String())
		return nil
	}

	return nil
}

func filterMessageHook(ctx *ext.Context, update *ext.Update) error {
	msg := update.EffectiveMessage
	if msg == nil {
		return nil
	}

	user := update.EffectiveUser()
	if msg.Out || (user != nil && user.ID == ctx.Self.ID) {
		return nil
	}

	if msg.Text == "" {
		return nil
	}

	uChat := update.EffectiveChat()
	if uChat.IsAUser() {
		return nil
	}

	key := fmt.Sprintf("userbot:filters:%d", uChat.GetID())
	filters, err := dbClient.Redis.HGetAll(ctx, key).Result()
	if err != nil || len(filters) == 0 {
		return nil
	}

	msgText := strings.TrimSpace(msg.Text)
	var matchedResponse string
	var found bool

	for trigger, resp := range filters {
		if strings.EqualFold(msgText, trigger) {
			matchedResponse = resp
			found = true
			break
		}

		escapedTrigger := regexp.QuoteMeta(trigger)
		re, err := regexp.Compile(`(?i)\b` + escapedTrigger + `\b`)
		if err == nil && re.MatchString(msgText) {
			matchedResponse = resp
			found = true
			break
		}
	}

	if found {
		text, entities := utils.ParseHTML(matchedResponse)
		req := &tg.MessagesSendMessageRequest{
			Message:  text,
			Entities: entities,
		}
		req.SetReplyTo(&tg.InputReplyToMessage{
			ReplyToMsgID: msg.ID,
		})
		_, err = ctx.SendMessage(uChat.GetID(), req)
		if err != nil {
			slog.Error("Failed to send filter reply", "error", err)
			return err
		}
	}

	return nil
}

func setFilter(ctx context.Context, chatId int64, trigger string, response string) error {
	key := fmt.Sprintf("userbot:filters:%d", chatId)
	return dbClient.Redis.HSet(ctx, key, strings.ToLower(trigger), response).Err()
}

func deleteFilter(ctx context.Context, chatId int64, trigger string) (int64, error) {
	key := fmt.Sprintf("userbot:filters:%d", chatId)
	return dbClient.Redis.HDel(ctx, key, strings.ToLower(trigger)).Result()
}

func getFilters(ctx context.Context, chatId int64) (map[string]string, error) {
	key := fmt.Sprintf("userbot:filters:%d", chatId)
	return dbClient.Redis.HGetAll(ctx, key).Result()
}
