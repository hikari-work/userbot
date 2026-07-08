package admins

import (
	"fmt"
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
		Name:        "CleanService",
		Description: "Automatically clean service messages like user join and user left",
		Commands:    []string{"cleanservice", "cleanaction"},
		OnlyOut:     true,
		Handler:     cleanServiceHandler,
		OnMessage:   cleanServiceMessageHook,
	})
}

func cleanServiceHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if uChat.IsAUser() {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize(ctx, "CSOnlyGroupError", nil, nil))
		return nil
	}

	args := update.Args()
	ctxBg := ctx
	key := fmt.Sprintf("userbot:cleanservice:%d", uChat.GetID())

	var status bool
	var err error

	if len(args) > 0 {
		arg := strings.ToLower(args[0])
		if arg == "on" || arg == "yes" || arg == "true" || arg == "1" {
			err = dbClient.Redis.Set(ctxBg, key, "1", 0).Err()
			status = true
		} else if arg == "off" || arg == "no" || arg == "false" || arg == "0" {
			err = dbClient.Redis.Del(ctxBg, key).Err()
			status = false
		} else {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize(ctx, "CSUsage", nil, nil))
			return nil
		}
	} else {
		exists, errExist := dbClient.Redis.Exists(ctxBg, key).Result()
		if errExist != nil {
			err = errExist
		} else if exists > 0 {
			err = dbClient.Redis.Del(ctxBg, key).Err()
			status = false
		} else {
			err = dbClient.Redis.Set(ctxBg, key, "1", 0).Err()
			status = true
		}
	}

	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize(ctx, "CSErrorUpdate", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	var statusKey string
	if status {
		statusKey = "CSEnabled"
	} else {
		statusKey = "CSDisabled"
	}
	statusStr := i18n.Localize(ctx, statusKey, nil, nil)
	msgStr := i18n.Localize(ctx, "CSStatusChange", map[string]interface{}{"Status": statusStr}, nil)
	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, msgStr)
	return nil
}

func cleanServiceMessageHook(ctx *ext.Context, update *ext.Update) error {
	msg := update.EffectiveMessage
	if msg == nil {
		return nil
	}

	if !msg.IsService {
		return nil
	}

	uChat := update.EffectiveChat()
	if uChat.IsAUser() {
		return nil
	}

	isTargetAction := false
	switch msg.Action.(type) {
	case *tg.MessageActionChatAddUser,
		*tg.MessageActionChatJoinedByLink,
		*tg.MessageActionChatJoinedByRequest,
		*tg.MessageActionChatDeleteUser,
		*tg.MessageActionGroupCall,
		*tg.MessageActionInviteToGroupCall,
		*tg.MessageActionGroupCallScheduled,
		*tg.MessageActionPhoneCall:
		isTargetAction = true
	}

	if !isTargetAction {
		return nil
	}

	ctxBg := ctx
	key := fmt.Sprintf("userbot:cleanservice:%d", uChat.GetID())

	enabled, err := dbClient.Redis.Exists(ctxBg, key).Result()
	if err != nil || enabled == 0 {
		return nil
	}

	canDelete, errCanDelete := canDeleteMessages(ctx, uChat.GetID())
	if errCanDelete != nil || !canDelete {
		return nil
	}

	_ = ctx.DeleteMessages(uChat.GetID(), []int{msg.ID})

	return nil
}
