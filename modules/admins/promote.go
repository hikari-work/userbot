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
		Name:        "Promote",
		Description: "Promote a user to admin",
		Commands:    []string{"promote"},
		OnlyOut:     true,
		Handler:     promoteHandler,
	})
}

func promoteHandler(ctx *ext.Context, update *ext.Update) error {
	chat := update.EffectiveChat()
	target, ok := getTargetUser(ctx, update, "promoting")
	if !ok {
		return nil
	}

	args := update.Args()
	title := getAdminTitle(args, update.EffectiveMessage.ReplyTo != nil)

	text, entities := utils.ParseHTML("⏳ <b>Promoting user...</b>")
	_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})

	rights := tg.ChatAdminRights{
		ChangeInfo:           true,
		PostMessages:         true,
		EditMessages:         true,
		DeleteMessages:       true,
		BanUsers:             true,
		InviteUsers:          true,
		PinMessages:          true,
		AddAdmins:            false,
		Anonymous:            false,
		ManageCall:           true,
		Other:                true,
		ManageTopics:         false,
		PostStories:          true,
		EditStories:          true,
		DeleteStories:        true,
		ManageDirectMessages: true,
	}
	rights.SetFlags()

	opts := &ext.EditAdminOpts{
		AdminRights: rights,
		AdminTitle:  title,
	}

	_, err := ctx.PromoteChatMember(chat.GetID(), target, opts)
	if err != nil {
		text, entities := utils.ParseHTML(fmt.Sprintf("❌ <b>Failed to promote user:</b> %s", err.Error()))
		_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       update.EffectiveMessage.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	successMsg := fmt.Sprintf("✅ User <code>%d</code> successfully promoted to Admin!", target)
	if title != "" {
		successMsg = fmt.Sprintf("✅ User <code>%d</code> successfully promoted with title <b>%s</b>!", target, title)
	}

	text, entities = utils.ParseHTML(successMsg)
	_, err = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})
	return err
}
