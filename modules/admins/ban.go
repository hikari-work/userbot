package admins

import (
	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "Ban",
		Description: "Ban a user from the group",
		Commands:    []string{"ban"},
		OnlyOut:     true,
		Handler:     banHandler,
	})

	manager.Register(&manager.Module{
		Name:        "Unban",
		Description: "Unban a user from the group",
		Commands:    []string{"unban"},
		OnlyOut:     true,
		Handler:     unbanHandler,
	})

	manager.Register(&manager.Module{
		Name:        "Kick",
		Description: "Kick a user from the group",
		Commands:    []string{"kick"},
		OnlyOut:     true,
		Handler:     kickHandler,
	})
}

func banHandler(ctx *ext.Context, update *ext.Update) error {
	chat := update.EffectiveChat()
	target, ok := getTargetUser(ctx, update, "banning")
	if !ok {
		return nil
	}
	loadingStr := i18n.Localize(ctx, "BANLoading", nil, nil)

	text, entities := utils.ParseHTML(loadingStr)
	_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})

	_, err := ctx.BanChatMember(chat.GetID(), target, 0)
	if err != nil {
		failedStr := i18n.Localize(ctx, "BANFailed", map[string]interface{}{
			"Error": err.Error(),
		}, nil)
		text, entities := utils.ParseHTML(failedStr)
		_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       update.EffectiveMessage.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}
	local := i18n.Localize(ctx, "BANSuccess", map[string]interface{}{
		"UserId": target,
	}, nil)

	text, entities = utils.ParseHTML(local)
	_, err = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})
	return err
}

func unbanHandler(ctx *ext.Context, update *ext.Update) error {
	chat := update.EffectiveChat()
	target, ok := getTargetUser(ctx, update, "unbanning")
	if !ok {
		return nil
	}

	loadingStr := i18n.Localize(ctx, "UNBANLoading", nil, nil)
	text, entities := utils.ParseHTML(loadingStr)
	_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})

	_, err := ctx.UnbanChatMember(chat.GetID(), target)
	if err != nil {
		failedStr := i18n.Localize(ctx, "UNBANFailed", map[string]interface{}{
			"Error": err.Error(),
		}, nil)
		text, entities := utils.ParseHTML(failedStr)
		_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       update.EffectiveMessage.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	successStr := i18n.Localize(ctx, "UNBANSuccess", map[string]interface{}{
		"UserId": target,
	}, nil)
	text, entities = utils.ParseHTML(successStr)
	_, err = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})
	return err
}

func kickHandler(ctx *ext.Context, update *ext.Update) error {
	chat := update.EffectiveChat()
	target, ok := getTargetUser(ctx, update, "kicking")
	if !ok {
		return nil
	}

	loadingStr := i18n.Localize(ctx, "KICKLoading", nil, nil)
	text, entities := utils.ParseHTML(loadingStr)
	_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})

	_, err := ctx.BanChatMember(chat.GetID(), target, 0)
	if err != nil {
		failedStr := i18n.Localize(ctx, "KICKFailed", map[string]interface{}{
			"Error": err.Error(),
		}, nil)
		text, entities := utils.ParseHTML(failedStr)
		_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       update.EffectiveMessage.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	_, _ = ctx.UnbanChatMember(chat.GetID(), target)

	successStr := i18n.Localize(ctx, "KICKSuccess", map[string]interface{}{
		"UserId": target,
	}, nil)
	text, entities = utils.ParseHTML(successStr)
	_, err = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})
	return err
}
