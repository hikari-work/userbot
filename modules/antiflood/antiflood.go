package antiflood

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/i18n"
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
		text, entities := utils.ParseHTML(i18n.Localize("FloodErrorNotGroup", nil, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	hasPermission, err := canRestrictMembers(ctx, uChat.GetID())
	if err != nil {
		text, entities := utils.ParseHTML(i18n.Localize("FloodErrorCheckPermission", map[string]interface{}{"Error": err.Error()}, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}
	if !hasPermission {
		text, entities := utils.ParseHTML(i18n.Localize("FloodErrorNoPermission", nil, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	if len(args) == 1 && strings.ToLower(args[0]) == "off" {
		ctxBg := ctx
		err := dbClient.Redis.Del(ctxBg, fmt.Sprintf("userbot:flood:cfg:%d", uChat.GetID())).Err()
		if err != nil {
			text, entities := utils.ParseHTML(i18n.Localize("FloodErrorDisable", map[string]interface{}{"Error": err.Error()}, nil))
			_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
				ID:       uMsg.ID,
				Message:  text,
				Entities: entities,
			})
			return err
		}

		text, entities := utils.ParseHTML(i18n.Localize("FloodDisabled", nil, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	if len(args) < 3 {
		text, entities := utils.ParseHTML(i18n.Localize("FloodUsage", nil, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	ttlVal, err := strconv.Atoi(args[0])
	if err != nil || ttlVal <= 0 {
		text, entities := utils.ParseHTML(i18n.Localize("FloodErrorTTL", nil, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	maxVal, err := strconv.Atoi(args[1])
	if err != nil || maxVal <= 0 || maxVal > 255 {
		text, entities := utils.ParseHTML(i18n.Localize("FloodErrorMax", nil, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	action := strings.ToLower(args[2])
	if action != "ban" && action != "kick" && action != "mute" {
		text, entities := utils.ParseHTML(i18n.Localize("FloodErrorAction", nil, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	ctxBg := ctx
	cfgKey := fmt.Sprintf("userbot:flood:cfg:%d", uChat.GetID())
	err = dbClient.Redis.HSet(ctxBg, cfgKey, map[string]interface{}{
		"ttl":    ttlVal,
		"max":    maxVal,
		"action": action,
	}).Err()

	if err != nil {
		text, entities := utils.ParseHTML(i18n.Localize("FloodFailedConfig", map[string]interface{}{"Error": err.Error()}, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	actionLoc := i18n.Localize("action_"+action, nil, nil)
	successMsg := i18n.Localize("FloodSuccessConfig", map[string]interface{}{
		"Max":    maxVal,
		"TTL":    ttlVal,
		"Action": actionLoc,
	}, nil)
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
		text, entities := utils.ParseHTML(i18n.Localize("FloodErrorPrivate", nil, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	ctxBg := ctx
	cfgKey := fmt.Sprintf("userbot:flood:cfg:%d", uChat.GetID())
	cfg, err := dbClient.Redis.HGetAll(ctxBg, cfgKey).Result()

	if err != nil {
		text, entities := utils.ParseHTML(i18n.Localize("FloodFailedRetrieve", map[string]interface{}{"Error": err.Error()}, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	if len(cfg) == 0 {
		text, entities := utils.ParseHTML(i18n.Localize("FloodNotConfigured", nil, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	actionLoc := i18n.Localize("action_"+cfg["action"], nil, nil)
	infoMsg := i18n.Localize("FloodConfigInfo", map[string]interface{}{
		"Max":    cfg["max"],
		"TTL":    cfg["ttl"],
		"Action": actionLoc,
	}, nil)
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

	ctxBg := ctx
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

		switch action {
		case "ban":
			_, err = ctx.BanChatMember(uChat.GetID(), userID, 0)
		case "kick":
			_, err = ctx.BanChatMember(uChat.GetID(), userID, 0)
			if err == nil {
				_, _ = ctx.UnbanChatMember(uChat.GetID(), userID)
			}
		case "mute":
			err = muteUser(ctx, uChat.GetID(), userID)
		}

		if err != nil {
			slog.Error("Failed to execute flood action", "action", action, "user", userID, "error", err)
			return nil
		}

		actionResultLoc := i18n.Localize("result_"+action, nil, nil)
		warnMsg := i18n.Localize("FloodTriggered", map[string]interface{}{
			"UserId": userID,
			"Result": actionResultLoc,
		}, nil)
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
