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

	text, entities := utils.ParseHTML(i18n.Localize("DemoteLoading", nil, nil))
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
		text, entities := utils.ParseHTML(i18n.Localize("DemoteFailed", map[string]interface{}{
			"Error": err.Error(),
		}, nil))
		_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       update.EffectiveMessage.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	text, entities = utils.ParseHTML(i18n.Localize("DemoteSuccess", map[string]interface{}{
		"UserId": target,
	}, nil))
	_, err = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})
	return err
}
