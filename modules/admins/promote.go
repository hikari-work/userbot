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
		Name:        "Promote",
		Description: "Promote a user to admin",
		Commands:    []string{"promote"},
		OnlyOut:     true,
		Handler:     promoteHandler,
		Help:        promoteHelp,
	})
}

func promoteHelp() string {
	return "Format \n<code>.promote &lt;reply/username/id&gt; &lt;title&gt;</code>\n<code>Contoh : .promote @username Admin</code>"
}

func promoteHandler(ctx *ext.Context, update *ext.Update) error {
	chat := update.EffectiveChat()
	target, ok := getTargetUser(ctx, update, "promoting")
	if !ok {
		return nil
	}

	args := update.Args()
	title := getAdminTitle(args, update.EffectiveMessage.ReplyTo != nil)

	text, entities := utils.ParseHTML(i18n.Localize("PromoteLoading", nil, nil))
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
		text, entities := utils.ParseHTML(i18n.Localize("PromoteFailed", map[string]interface{}{
			"Error": err.Error(),
		}, nil))
		_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       update.EffectiveMessage.ID,
			Message:  text,
			Entities: entities,
		})
		return err
	}

	var successMsg string
	if title != "" {
		successMsg = i18n.Localize("PromoteSuccessTitle", map[string]interface{}{
			"UserId": target,
			"Title":  title,
		}, nil)
	} else {
		successMsg = i18n.Localize("PromoteSuccess", map[string]interface{}{
			"UserId": target,
		}, nil)
	}

	text, entities = utils.ParseHTML(successMsg)
	_, err = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       update.EffectiveMessage.ID,
		Message:  text,
		Entities: entities,
	})
	return err
}
