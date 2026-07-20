package chat

import (
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "Chat",
		Description: "Manage chat pin and unpin",
		Commands:    []string{"pin", "unpin"},
		OnlyOut:     true,
		Help:        helpString,
		Handler:     pinAndUnpin,
	})
}

func helpString() string {
	return "Format:\n" +
		"<code>.pin</code> [loud] - Reply to a message to pin it\n" +
		"<code>.unpin</code> or <code>.unpinned</code> - Reply to a message to unpin it\n" +
		"<code>.unpin all</code> or <code>.unpinned all</code> - Unpin all pinned messages in the chat"
}

func pinAndUnpin(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if uMsg == nil || uChat == nil {
		return nil
	}

	args := strings.Fields(uMsg.Message.Message)
	if len(args) == 0 {
		return nil
	}

	cmd := strings.ToLower(args[0])
	isUnpin := strings.HasSuffix(cmd, "unpin") || strings.HasSuffix(cmd, "unpinned")

	inputPeer, err := ctx.ResolveInputPeerById(uChat.GetID())
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("PinFailed", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	var replyHeader *tg.MessageReplyHeader
	if uMsg.ReplyTo != nil {
		if r, ok := uMsg.ReplyTo.(*tg.MessageReplyHeader); ok {
			replyHeader = r
		}
	}
	isReply := replyHeader != nil && replyHeader.ReplyToMsgID != 0

	if isUnpin {
		if len(args) > 1 && strings.EqualFold(args[1], "all") {
			_, err := ctx.Raw.MessagesUnpinAllMessages(ctx, &tg.MessagesUnpinAllMessagesRequest{
				Peer: inputPeer,
			})
			if err != nil {
				_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("UnpinAllFailed", map[string]interface{}{"Error": err.Error()}, nil))
				return err
			}
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("UnpinAllSuccess", nil, nil))
			return nil
		}

		if !isReply {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("UnpinReplyRequired", nil, nil))
			return nil
		}

		_, err := ctx.Raw.MessagesUpdatePinnedMessage(ctx, &tg.MessagesUpdatePinnedMessageRequest{
			Unpin: true,
			Peer:  inputPeer,
			ID:    replyHeader.ReplyToMsgID,
		})
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("UnpinFailed", map[string]interface{}{"Error": err.Error()}, nil))
			return err
		}

		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("UnpinSuccess", nil, nil))
		return nil
	}

	if !isReply {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("PinReplyRequired", nil, nil))
		return nil
	}

	silent := true
	if len(args) > 1 && (strings.EqualFold(args[1], "loud") || strings.EqualFold(args[1], "notify")) {
		silent = false
	}

	_, err = ctx.Raw.MessagesUpdatePinnedMessage(ctx, &tg.MessagesUpdatePinnedMessageRequest{
		Silent: silent,
		Unpin:  false,
		Peer:   inputPeer,
		ID:     replyHeader.ReplyToMsgID,
	})
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("PinFailed", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("PinSuccess", nil, nil))
	return nil
}
