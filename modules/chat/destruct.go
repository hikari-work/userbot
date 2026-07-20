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
		Name:        "Destructive",
		Description: "Delete a chat or purge messages",
		Commands:    []string{"delchat", "delmsg"},
		OnlyOut:     true,
		Help:        helpDestruct,
		Handler:     destructHandler,
	})
}

func helpDestruct() string {
	return "Format:\n" +
		"<code>.delchat</code> - Delete private chat for both sides or leave group\n" +
		"<code>.delmsg</code> [me/you] - Delete messages in chat (use 'me' for your messages, 'you' for target user messages, or reply to a message)"
}

func destructHandler(ctx *ext.Context, update *ext.Update) error {
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
	if strings.HasSuffix(cmd, "delchat") {
		return delChatHandler(ctx, update)
	} else if strings.HasSuffix(cmd, "delmsg") {
		return delMsgHandler(ctx, update)
	}

	return nil
}

func delChatHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	inputPeer, err := ctx.ResolveInputPeerById(uChat.GetID())
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("DelchatFailed", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	if uChat.IsAUser() {
		_, err = ctx.Raw.MessagesDeleteHistory(ctx, &tg.MessagesDeleteHistoryRequest{
			JustClear: false,
			Revoke:    true,
			Peer:      inputPeer,
		})
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("DelchatFailed", map[string]interface{}{"Error": err.Error()}, nil))
			return err
		}
		return nil
	}

	switch p := inputPeer.(type) {
	case *tg.InputPeerChannel:
		_, err = ctx.Raw.ChannelsLeaveChannel(ctx, &tg.InputChannel{
			ChannelID:  p.ChannelID,
			AccessHash: p.AccessHash,
		})
	case *tg.InputPeerChat:
		selfPeer, resolveErr := ctx.ResolveInputPeerById(ctx.Self.ID)
		if resolveErr != nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("DelchatFailed", map[string]interface{}{"Error": resolveErr.Error()}, nil))
			return resolveErr
		}
		if selfUser, ok := selfPeer.(*tg.InputPeerUser); ok {
			_, err = ctx.Raw.MessagesDeleteChatUser(ctx, &tg.MessagesDeleteChatUserRequest{
				ChatID: p.ChatID,
				UserID: &tg.InputUser{
					UserID:     selfUser.UserID,
					AccessHash: selfUser.AccessHash,
				},
			})
		}
	}

	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("DelchatFailed", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	return nil
}

func delMsgHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	cmdArgs := update.Args()
	filterArg := ""
	if len(cmdArgs) > 0 {
		filterArg = strings.ToLower(cmdArgs[0])
	}

	inputPeer, err := ctx.ResolveInputPeerById(uChat.GetID())
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("DelmsgFailed", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	var replyHeader *tg.MessageReplyHeader
	if uMsg.ReplyTo != nil {
		if r, ok := uMsg.ReplyTo.(*tg.MessageReplyHeader); ok {
			replyHeader = r
		}
	}
	isReply := replyHeader != nil && replyHeader.ReplyToMsgID != 0

	var targetSenderID int64
	var toDel []int

	if isReply {
		var toFetch []tg.InputMessageClass
		for i := replyHeader.ReplyToMsgID; i <= uMsg.ID; i++ {
			toFetch = append(toFetch, &tg.InputMessageID{ID: i})
		}

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

			if filterArg == "you" {
				for _, m := range msgs {
					if msgObj, ok := m.(*tg.Message); ok && msgObj.ID == replyHeader.ReplyToMsgID {
						if pUser, ok := msgObj.FromID.(*tg.PeerUser); ok {
							targetSenderID = pUser.UserID
						}
						break
					}
				}
			}

			for _, m := range msgs {
				msgObj, ok := m.(*tg.Message)
				if !ok {
					continue
				}

				if shouldDelete(msgObj, filterArg, ctx.Self.ID, targetSenderID) {
					toDel = append(toDel, msgObj.ID)
				}
			}
		}
	} else {
		res, fetchErr := ctx.Raw.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:  inputPeer,
			Limit: 100,
		})
		if fetchErr != nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("DelmsgFailed", map[string]interface{}{"Error": fetchErr.Error()}, nil))
			return fetchErr
		}

		var msgs []tg.MessageClass
		switch m := res.(type) {
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

			if shouldDelete(msgObj, filterArg, ctx.Self.ID, 0) {
				toDel = append(toDel, msgObj.ID)
			}
		}
	}

	if len(toDel) == 0 {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("DelmsgNoMessages", nil, nil))
		return nil
	}

	const chunkSize = 100
	for i := 0; i < len(toDel); i += chunkSize {
		end := i + chunkSize
		if end > len(toDel) {
			end = len(toDel)
		}
		_ = ctx.DeleteMessages(uChat.GetID(), toDel[i:end])
	}

	if filterArg == "you" {
		_ = ctx.DeleteMessages(uChat.GetID(), []int{uMsg.ID})
	}

	return nil
}

func shouldDelete(msgObj *tg.Message, filterArg string, selfID int64, targetSenderID int64) bool {
	switch filterArg {
	case "me":
		if msgObj.Out {
			return true
		}
		if pUser, ok := msgObj.FromID.(*tg.PeerUser); ok && pUser.UserID == selfID {
			return true
		}
		return false
	case "you":
		if msgObj.Out {
			return false
		}
		if targetSenderID != 0 {
			if pUser, ok := msgObj.FromID.(*tg.PeerUser); ok && pUser.UserID == targetSenderID {
				return true
			}
			return false
		}
		return true
	default:
		return true
	}
}
