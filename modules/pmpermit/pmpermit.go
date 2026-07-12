package pmpermit

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
		Name:        "PMPermit",
		Description: "PM Security to protect DM from spam",
		Commands:    []string{"pmpermit", "pmp"},
		OnlyOut:     true,
		Handler:     pmpermitToggleHandler,
		OnMessage:   pmpermitMessageHook,
	})

	manager.Register(&manager.Module{
		Name:        "PMApprove",
		Description: "Approve a user to PM you",
		Commands:    []string{"approve", "a"},
		OnlyOut:     true,
		Handler:     approveHandler,
	})

	manager.Register(&manager.Module{
		Name:        "PMDisapprove",
		Description: "Disapprove a user from PMing you",
		Commands:    []string{"disapprove", "da"},
		OnlyOut:     true,
		Handler:     disapproveHandler,
	})

	manager.Register(&manager.Module{
		Name:        "PMBlock",
		Description: "Block a user from messaging you",
		Commands:    []string{"block"},
		OnlyOut:     true,
		Handler:     blockHandler,
	})

	manager.Register(&manager.Module{
		Name:        "PMUnblock",
		Description: "Unblock a user",
		Commands:    []string{"unblock"},
		OnlyOut:     true,
		Handler:     unblockHandler,
	})
}

func getTargetUser(ctx *ext.Context, update *ext.Update) (int64, bool) {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	target, err := utils.ExtractUser(ctx, uMsg, uChat)
	if err != nil {
		if uChat.IsAUser() {
			return uChat.GetID(), true
		}
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "❌ <b>Error:</b> Tentukan pengguna.")
		return 0, false
	}
	return target, true
}

func pmpermitToggleHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage
	args := update.Args()

	ctxBg := ctx

	if len(args) == 0 {
		enabled, err := dbClient.Redis.Get(ctxBg, "userbot:pmpermit:enabled").Result()
		status := "enabled"
		if err == nil && enabled == "false" {
			status = "disabled"
		}
		htmlStr := fmt.Sprintf("ℹ️ <b>PM Security Status:</b> <code>%s</code>\n\n%s", status, i18n.Localize("PMPermitUsage", nil, nil))
		_, err = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, htmlStr)
		return err
	}

	cmd := strings.ToLower(args[0])
	if cmd == "on" || cmd == "enable" || cmd == "true" {
		err := dbClient.Redis.Set(ctxBg, "userbot:pmpermit:enabled", "true", 0).Err()
		if err != nil {
			return err
		}
		_, err = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("PMPermitEnabled", nil, nil))
		return err
	} else if cmd == "off" || cmd == "disable" || cmd == "false" {
		err := dbClient.Redis.Set(ctxBg, "userbot:pmpermit:enabled", "false", 0).Err()
		if err != nil {
			return err
		}
		_, err = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("PMPermitDisabled", nil, nil))
		return err
	}

	_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("PMPermitUsage", nil, nil))
	return err
}

func approveHandler(ctx *ext.Context, update *ext.Update) error {
	target, ok := getTargetUser(ctx, update)
	if !ok {
		return nil
	}

	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	ctxBg := ctx
	dbClient.Redis.SAdd(ctxBg, "userbot:pmpermit:approved", target)
	dbClient.Redis.Del(ctxBg, fmt.Sprintf("userbot:pmpermit:warns:%d", target))

	lastMsgKey := fmt.Sprintf("userbot:pmpermit:lastmsg:%d", target)
	lastMsgIDStr, err := dbClient.Redis.Get(ctxBg, lastMsgKey).Result()
	if err == nil && lastMsgIDStr != "" {
		if id, err := strconv.Atoi(lastMsgIDStr); err == nil {
			_ = ctx.DeleteMessages(uChat.GetID(), []int{id})
		}
	}
	dbClient.Redis.Del(ctxBg, lastMsgKey)

	_, err = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("PMPermitApproved", nil, nil))
	return err
}

func disapproveHandler(ctx *ext.Context, update *ext.Update) error {
	target, ok := getTargetUser(ctx, update)
	if !ok {
		return nil
	}

	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	ctxBg := ctx
	dbClient.Redis.SRem(ctxBg, "userbot:pmpermit:approved", target)

	_, err := utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("PMPermitDisapproved", nil, nil))
	return err
}

func blockHandler(ctx *ext.Context, update *ext.Update) error {
	target, ok := getTargetUser(ctx, update)
	if !ok {
		return nil
	}

	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	inputPeer, err := ctx.ResolveInputPeerById(target)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Error:</b> %s", err.Error()))
		return nil
	}

	_, err = ctx.Raw.ContactsBlock(ctx, &tg.ContactsBlockRequest{ID: inputPeer})
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Error:</b> %s", err.Error()))
		return nil
	}

	ctxBg := ctx
	dbClient.Redis.SRem(ctxBg, "userbot:pmpermit:approved", target)
	dbClient.Redis.Del(ctxBg, fmt.Sprintf("userbot:pmpermit:warns:%d", target))

	_, err = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("PMPermitBlocked", nil, nil))
	return err
}

func unblockHandler(ctx *ext.Context, update *ext.Update) error {
	target, ok := getTargetUser(ctx, update)
	if !ok {
		return nil
	}

	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	inputPeer, err := ctx.ResolveInputPeerById(target)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Error:</b> %s", err.Error()))
		return nil
	}

	_, err = ctx.Raw.ContactsUnblock(ctx, &tg.ContactsUnblockRequest{ID: inputPeer})
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Error:</b> %s", err.Error()))
		return nil
	}

	_, err = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("PMPermitUnblocked", nil, nil))
	return err
}
