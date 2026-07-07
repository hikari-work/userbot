package admins

import (
	"fmt"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
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

	text, entities := utils.ParseHTML("⏳ <b>Banning user...</b>")
	_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})
	
	_, err := ctx.BanChatMember(chat.GetID(), target, 0)
	if err != nil {
		text, entities := utils.ParseHTML(fmt.Sprintf("❌ <b>Failed to ban user:</b> %s", err.Error()))
		_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       update.EffectiveMessage.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	text, entities = utils.ParseHTML(fmt.Sprintf("✅ User <code>%d</code> successfully banned from the group!", target))
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

	text, entities := utils.ParseHTML("⏳ <b>Unbanning user...</b>")
	_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})

	_, err := ctx.UnbanChatMember(chat.GetID(), target)
	if err != nil {
		text, entities := utils.ParseHTML(fmt.Sprintf("❌ <b>Failed to unban user:</b> %s", err.Error()))
		_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       update.EffectiveMessage.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	text, entities = utils.ParseHTML(fmt.Sprintf("✅ User <code>%d</code> successfully unbanned!", target))
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

	text, entities := utils.ParseHTML("⏳ <b>Kicking user...</b>")
	_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})

	_, err := ctx.BanChatMember(chat.GetID(), target, 0)
	if err != nil {
		text, entities := utils.ParseHTML(fmt.Sprintf("❌ <b>Failed to kick user:</b> %s", err.Error()))
		_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       update.EffectiveMessage.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	_, _ = ctx.UnbanChatMember(chat.GetID(), target)

	text, entities = utils.ParseHTML(fmt.Sprintf("✅ User <code>%d</code> successfully kicked!", target))
	_, err = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})
	return err
}
