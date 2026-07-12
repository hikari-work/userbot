package antiflood

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/utils"
)

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
