package utils

import (
	"errors"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/celestix/gotgproto/types"
	"github.com/gotd/td/tg"
)

func ExtractUser(ctx *ext.Context, msg *types.Message, chat types.EffectiveChat) (target int64, err error) {
	if reply, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && reply.ReplyToMsgID != 0 {
		var m []tg.MessageClass
		m, err = ctx.GetMessages(chat.GetID(), []tg.InputMessageClass{&tg.InputMessageID{
			ID: reply.ReplyToMsgID,
		}})
		if err != nil {
			return
		}
		msg, ok := m[0].(*tg.Message)
		if ok {
			target = msg.FromID.(*tg.PeerUser).UserID
		}

	}
	if target == 0 {
		args := strings.Fields(msg.Text)
		if !(len(args) > 1 && strings.HasPrefix(args[1], "@")) {
			err = errors.New("no user provided")
			return
		}
		var c types.EffectiveChat
		c, err = ctx.ResolveUsername(args[1])
		if err != nil {
			return
		}
		target = c.GetID()
	}
	return
}
