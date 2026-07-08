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
		Name:        "Purge",
		Description: "Delete all messages from the replied message to this message",
		Commands:    []string{"purge"},
		OnlyOut:     true,
		Handler:     purgeHandler,
	})

	manager.Register(&manager.Module{
		Name:        "Purgeme",
		Description: "Delete your own messages from the replied message to this message",
		Commands:    []string{"purgeme"},
		OnlyOut:     true,
		Handler:     purgeMeHandler,
	})
}

func purgeHandler(ctx *ext.Context, update *ext.Update) error {
	uMsg := update.EffectiveMessage
	uChat := update.EffectiveChat()

	canDelete, err := canDeleteMessages(ctx, uChat.GetID())
	if err != nil || !canDelete {
		errText := i18n.Localize(ctx, "PurgeNoPermission", nil, nil)
		if err != nil {
			errText = i18n.Localize(ctx, "PurgeCheckPermissionError", map[string]interface{}{"Error": err.Error()}, nil)
		}
		text, entities := utils.ParseHTML(errText)
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	reply, ok := uMsg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || reply.ReplyToMsgID == 0 {
		text, entities := utils.ParseHTML(i18n.Localize(ctx, "PurgeReplyRequired", nil, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	toDel := []int{uMsg.ID}
	for i := reply.ReplyToMsgID; i < uMsg.ID; i++ {
		toDel = append(toDel, i)
	}

	const chunkSize = 100
	for i := 0; i < len(toDel); i += chunkSize {
		end := i + chunkSize
		if end > len(toDel) {
			end = len(toDel)
		}
		_ = ctx.DeleteMessages(uChat.GetID(), toDel[i:end])
	}

	return nil
}

func purgeMeHandler(ctx *ext.Context, update *ext.Update) error {
	uMsg := update.EffectiveMessage
	uChat := update.EffectiveChat()

	reply, ok := uMsg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || reply.ReplyToMsgID == 0 {
		text, entities := utils.ParseHTML(i18n.Localize(ctx, "PurgemeReplyRequired", nil, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	var toFetch []tg.InputMessageClass
	for i := reply.ReplyToMsgID; i <= uMsg.ID; i++ {
		toFetch = append(toFetch, &tg.InputMessageID{ID: i})
	}

	inputPeer, errPeer := ctx.ResolveInputPeerById(uChat.GetID())
	if errPeer != nil {
		text, entities := utils.ParseHTML(i18n.Localize(ctx, "PurgeResolveChatFailed", map[string]interface{}{"Error": errPeer.Error()}, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return errPeer
	}

	var toDel []int

	const chunkSize = 100
	for i := 0; i < len(toFetch); i += chunkSize {
		end := i + chunkSize
		if end > len(toFetch) {
			end = len(toFetch)
		}

		var messagesClass tg.MessagesMessagesClass
		var fetchErr error

		switch p := inputPeer.(type) {
		case *tg.InputPeerChannel:
			messagesClass, fetchErr = ctx.Raw.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
				Channel: &tg.InputChannel{
					ChannelID:  p.ChannelID,
					AccessHash: p.AccessHash,
				},
				ID: toFetch[i:end],
			})
		default:
			messagesClass, fetchErr = ctx.Raw.MessagesGetMessages(ctx, toFetch[i:end])
		}

		if fetchErr != nil {
			continue
		}

		var msgs []tg.MessageClass
		switch m := messagesClass.(type) {
		case *tg.MessagesMessages:
			msgs = m.Messages
		case *tg.MessagesMessagesSlice:
			msgs = m.Messages
		case *tg.MessagesChannelMessages:
			msgs = m.Messages
		}

		for _, m := range msgs {
			msgObj, ok := m.(*tg.Message)
			if !ok {
				continue
			}
			if msgObj.Out {
				toDel = append(toDel, msgObj.ID)
			}
		}
	}

	for i := 0; i < len(toDel); i += chunkSize {
		end := i + chunkSize
		if end > len(toDel) {
			end = len(toDel)
		}
		_ = ctx.DeleteMessages(uChat.GetID(), toDel[i:end])
	}

	return nil
}
