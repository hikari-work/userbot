package antiflood

import (
	"fmt"
	"strconv"
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
		Name:        "AntifloodSet",
		Description: "Set flood protection settings for the chat",
		Commands:    []string{"setflood"},
		OnlyOut:     true,
		Handler:     setFloodHandler,
		OnMessage:   floodMessageHook,
		Help:        setfloodHelp,
	})

	manager.Register(&manager.Module{
		Name:        "AntifloodGet",
		Description: "Get current flood protection settings for the chat",
		Commands:    []string{"getflood"},
		OnlyOut:     true,
		Handler:     getFloodHandler,
		Help:        getfloodHelp,
	})
}

func setfloodHelp() string {
	return "Format \n<code>.setflood &lt;jumlah_pesan&gt;</code>\n<code>Contoh : .setflood 5</code>"
}

func getfloodHelp() string {
	return "Format \n<code>.getflood</code>\n<code>Contoh : .getflood</code>"
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

