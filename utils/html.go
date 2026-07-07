package utils

import (
	"github.com/celestix/gotgproto/ext"
	"github.com/celestix/gotgproto/types"
	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/html"
	"github.com/gotd/td/telegram/message/styling"
	"github.com/gotd/td/tg"
)

func ParseHTML(s string) (string, []tg.MessageEntityClass) {
	b := &entity.Builder{}
	opt := html.String(nil, s)
	_ = styling.Perform(b, opt)
	text, entities := b.Complete()
	return text, entities
}

func EditMessageHTML(ctx *ext.Context, chatID int64, messageID int, htmlContent string) (*types.Message, error) {
	text, entities := ParseHTML(htmlContent)
	return ctx.EditMessage(chatID, &tg.MessagesEditMessageRequest{
		ID:       messageID,
		Message:  text,
		Entities: entities,
	})
}
