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
		Name:        "Demote",
		Description: "Demote an admin back to user",
		Commands:    []string{"demote"},
		OnlyOut:     true,
		Handler:     demoteHandler,
	})
}

func demoteHandler(ctx *ext.Context, update *ext.Update) error {
	chat := update.EffectiveChat()
	target, ok := getTargetUser(ctx, update, "demoting")
	if !ok {
		return nil
	}

	// Display progress status
	text, entities := utils.ParseHTML("⏳ <b>Demoting user...</b>")
	_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})

	rights := tg.ChatAdminRights{}
	rights.SetFlags()

	opts := &ext.EditAdminOpts{
		AdminRights: rights,
	}

	_, err := ctx.DemoteChatMember(chat.GetID(), target, opts)
	if err != nil {
		text, entities := utils.ParseHTML(fmt.Sprintf("❌ <b>Failed to demote user:</b> %s", err.Error()))
		_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       update.EffectiveMessage.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	text, entities = utils.ParseHTML(fmt.Sprintf("✅ Admin rights for user <code>%d</code> successfully revoked!", target))
	_, err = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})
	return err
}
