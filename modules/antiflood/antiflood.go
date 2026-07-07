package antiflood

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "AntifloodSet",
		Description: "Set flood protection settings for the chat",
		Commands:    []string{"setflood"},
		OnlyOut:     true,
		Handler:     setFloodHandler,
		OnMessage:   floodMessageHook,
	})

	manager.Register(&manager.Module{
		Name:        "AntifloodGet",
		Description: "Get current flood protection settings for the chat",
		Commands:    []string{"getflood"},
		OnlyOut:     true,
		Handler:     getFloodHandler,
	})
}

func setFloodHandler(ctx *ext.Context, update *ext.Update) error {
	uMsg := update.EffectiveMessage
	uChat := update.EffectiveChat()
	args := update.Args()

	if uChat.IsAUser() {
		text, entities := utils.ParseHTML("❌ <b>Error:</b> Antiflood can only be used in groups or supergroups.")
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	hasPermission, err := canRestrictMembers(ctx, uChat.GetID())
	if err != nil {
		text, entities := utils.ParseHTML(fmt.Sprintf("❌ <b>Error checking permissions:</b> %s", err.Error()))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}
	if !hasPermission {
		text, entities := utils.ParseHTML("❌ <b>Error:</b> You must be an admin with permission to ban/restrict members to configure Antiflood.")
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	if len(args) == 1 && strings.ToLower(args[0]) == "off" {
		ctxBg := context.Background()
		err := dbClient.Redis.Del(ctxBg, fmt.Sprintf("userbot:flood:cfg:%d", uChat.GetID())).Err()
		if err != nil {
			text, entities := utils.ParseHTML(fmt.Sprintf("❌ <b>Failed to disable Antiflood:</b> %s", err.Error()))
			_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
				ID:       uMsg.ID,
				Message:  text,
				Entities: entities,
			})
			return err
		}

		text, entities := utils.ParseHTML("✅ <b>Antiflood has been disabled for this chat!</b>")
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	if len(args) < 3 {
		text, entities := utils.ParseHTML("❌ <b>Usage:</b> <code>.setflood [ttl_seconds] [max_messages] [action]</code>\n" +
			"Example: <code>.setflood 5 10 ban</code>\n" +
			"Or to disable: <code>.setflood off</code>\n" +
			"Actions: <code>ban</code>, <code>kick</code>, <code>mute</code>")
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	ttlVal, err := strconv.Atoi(args[0])
	if err != nil || ttlVal <= 0 {
		text, entities := utils.ParseHTML("❌ <b>Error:</b> TTL duration must be a positive number.")
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	maxVal, err := strconv.Atoi(args[1])
	if err != nil || maxVal <= 0 || maxVal > 255 {
		text, entities := utils.ParseHTML("❌ <b>Error:</b> Max message count must be between 1 and 255.")
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	action := strings.ToLower(args[2])
	if action != "ban" && action != "kick" && action != "mute" {
		text, entities := utils.ParseHTML("❌ <b>Error:</b> Action must be one of: <code>ban</code>, <code>kick</code>, <code>mute</code>.")
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	ctxBg := context.Background()
	cfgKey := fmt.Sprintf("userbot:flood:cfg:%d", uChat.GetID())
	err = dbClient.Redis.HSet(ctxBg, cfgKey, map[string]interface{}{
		"ttl":    ttlVal,
		"max":    maxVal,
		"action": action,
	}).Err()

	if err != nil {
		text, entities := utils.ParseHTML(fmt.Sprintf("❌ <b>Failed to set Antiflood configuration:</b> %s", err.Error()))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	successMsg := fmt.Sprintf("✅ <b>Antiflood set successfully!</b>\n"+
		"• Limit: <code>%d</code> messages per <code>%d</code> seconds\n"+
		"• Action: <b>%s</b>", maxVal, ttlVal, action)
	text, entities := utils.ParseHTML(successMsg)
	_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       uMsg.ID,
		Message:  text,
		Entities: entities,
	})
	return nil
}

func getFloodHandler(ctx *ext.Context, update *ext.Update) error {
	uMsg := update.EffectiveMessage
	uChat := update.EffectiveChat()

	if uChat.IsAUser() {
		text, entities := utils.ParseHTML("❌ <b>Error:</b> Antiflood is not applicable in private chats.")
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	ctxBg := context.Background()
	cfgKey := fmt.Sprintf("userbot:flood:cfg:%d", uChat.GetID())
	cfg, err := dbClient.Redis.HGetAll(ctxBg, cfgKey).Result()

	if err != nil {
		text, entities := utils.ParseHTML(fmt.Sprintf("❌ <b>Failed to retrieve configuration:</b> %s", err.Error()))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	if len(cfg) == 0 {
		text, entities := utils.ParseHTML("ℹ️ <b>Antiflood is currently not configured for this chat.</b>")
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	infoMsg := fmt.Sprintf("ℹ️ <b>Antiflood configuration for this chat:</b>\n"+
		"• Limit: <code>%s</code> messages per <code>%s</code> seconds\n"+
		"• Action: <b>%s</b>", cfg["max"], cfg["ttl"], cfg["action"])
	text, entities := utils.ParseHTML(infoMsg)
	_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       uMsg.ID,
		Message:  text,
		Entities: entities,
	})
	return nil
}

func floodMessageHook(ctx *ext.Context, update *ext.Update) error {
	msg := update.EffectiveMessage
	if msg == nil {
		return nil
	}
	user := update.EffectiveUser()

	if msg.Out || user == nil || user.ID == ctx.Self.ID {
		return nil
	}

	uChat := update.EffectiveChat()
	userID := user.ID

	ctxBg := context.Background()
	cfgKey := fmt.Sprintf("userbot:flood:cfg:%d", uChat.GetID())
	cfg, err := dbClient.Redis.HGetAll(ctxBg, cfgKey).Result()
	if err != nil || len(cfg) == 0 {
		return nil
	}

	ttlVal, err1 := strconv.Atoi(cfg["ttl"])
	maxValUint, err2 := strconv.ParseUint(cfg["max"], 10, 8)
	action := cfg["action"]
	if err1 != nil || err2 != nil || action == "" {
		return nil
	}
	maxVal := uint8(maxValUint)

	isAdmin, errAdmin := utils.IsAdminOrSelf(ctx, uChat.GetID(), userID)
	if errAdmin != nil {
		slog.Error("Failed to check admin status in floodMessageHook", "error", errAdmin)
		return nil
	}
	if isAdmin {
		return nil
	}

	cntKey := fmt.Sprintf("userbot:flood:cnt:%d:%d", uChat.GetID(), userID)
	count, err := dbClient.Redis.Incr(ctxBg, cntKey).Result()
	if err != nil {
		return nil
	}

	if count == 1 {
		dbClient.Redis.Expire(ctxBg, cntKey, time.Duration(ttlVal)*time.Second)
	}

	if count > int64(maxVal) {
		dbClient.Redis.Del(ctxBg, cntKey)

		var actionResult string
		switch action {
		case "ban":
			_, err = ctx.BanChatMember(uChat.GetID(), userID, 0)
			actionResult = "banned from the group"
		case "kick":
			_, err = ctx.BanChatMember(uChat.GetID(), userID, 0)
			if err == nil {
				_, _ = ctx.UnbanChatMember(uChat.GetID(), userID)
			}
			actionResult = "kicked from the group"
		case "mute":
			err = muteUser(ctx, uChat.GetID(), userID)
			actionResult = "muted in this chat"
		}

		if err != nil {
			slog.Error("Failed to execute flood action", "action", action, "user", userID, "error", err)
			return nil
		}

		warnMsg := fmt.Sprintf("🚨 <b>Antiflood Triggered!</b>\n"+
			"User <code>%d</code> has been <b>%s</b> for flooding messages.", userID, actionResult)
		text, entities := utils.ParseHTML(warnMsg)
		_, _ = ctx.SendMessage(uChat.GetID(), &tg.MessagesSendMessageRequest{
			Message:  text,
			Entities: entities,
		})
	}

	return nil
}


func muteUser(ctx *ext.Context, chatID, userID int64) error {
	inputPeerChat, err := ctx.ResolveInputPeerById(chatID)
	if err != nil {
		return err
	}
	inputPeerUser, err := ctx.ResolveInputPeerById(userID)
	if err != nil {
		return err
	}
	pChannel, ok := inputPeerChat.(*tg.InputPeerChannel)
	if !ok {
		return fmt.Errorf("mute is only supported in supergroups/channels")
	}
	pUser, ok := inputPeerUser.(*tg.InputPeerUser)
	if !ok {
		return fmt.Errorf("invalid user peer")
	}

	rights := tg.ChatBannedRights{
		UntilDate:    0,
		SendMessages: true,
		SendMedia:    true,
		SendStickers: true,
		SendGifs:     true,
		SendGames:    true,
		SendInline:   true,
		EmbedLinks:   true,
	}
	rights.SetFlags()

	_, err = ctx.Raw.ChannelsEditBanned(ctx, &tg.ChannelsEditBannedRequest{
		Channel: &tg.InputChannel{
			ChannelID:  pChannel.ChannelID,
			AccessHash: pChannel.AccessHash,
		},
		Participant: &tg.InputPeerUser{
			UserID:     pUser.UserID,
			AccessHash: pUser.AccessHash,
		},
		BannedRights: rights,
	})
	return err
}

func canRestrictMembers(ctx *ext.Context, chatID int64) (bool, error) {
	return utils.CheckAdminPermission(ctx, chatID, func(rights tg.ChatAdminRights) bool {
		return rights.BanUsers
	})
}
