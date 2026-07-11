package admins

import (
	"log/slog"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/utils"
)

func getTargetUser(ctx *ext.Context, update *ext.Update, actionName string) (int64, bool) {
	chat := update.EffectiveChat()
	msg := update.EffectiveMessage

	if chat.IsAUser() {
		html, classes := utils.ParseHTML(i18n.Localize("OnlyGroupError", nil, nil))
		_, err := ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       msg.ID,
			Message:  html,
			Entities: classes,
		})
		if err != nil {
			slog.Error("Failed to send group error message", "error", err)
		}
		return 0, false
	}

	target, err := utils.ExtractUser(ctx, msg, chat)
	if err != nil {
		actionLoc := i18n.Localize("action_"+actionName, nil, nil)
		html, classes := utils.ParseHTML(i18n.Localize("FailedGetTargetUser", map[string]interface{}{
			"Action": actionLoc,
			"Error":  err.Error(),
		}, nil))
		_, editErr := ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       msg.ID,
			Message:  html,
			Entities: classes,
		})
		if editErr != nil {
			slog.Error("Failed to send extract user error message", "error", editErr)
		}
		return 0, false
	}

	return target, true
}

func getAdminTitle(args []string, isReply bool) string {
	if isReply {
		return strings.Join(args, " ")
	}
	if len(args) > 1 {
		return strings.Join(args[1:], " ")
	}
	return ""
}

func canDeleteMessages(ctx *ext.Context, chatID int64) (bool, error) {
	return utils.CheckAdminPermission(ctx, chatID, func(rights tg.ChatAdminRights) bool {
		return rights.DeleteMessages
	})
}
