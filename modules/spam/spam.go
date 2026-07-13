package spam

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "Spam",
		Description: "Sending message continuously at several time and period",
		Commands:    []string{"spam", "delayspam", "dspam", "wspam", "sspam"},
		OnlyOut:     true,
		Help:        spamHelp,
		Handler:     handler,
	})
}

func spamHelp() string {
	return i18n.Localize("SpamHelpText", nil, nil)
}

func handler(ctx *ext.Context, update *ext.Update) error {
	args := update.Args()
	if len(args) == 0 {
		return nil
	}

	cmd := strings.ToLower(args[0])
	if strings.HasSuffix(cmd, "delayspam") || strings.HasSuffix(cmd, "dspam") {
		return handleDelaySpam(ctx, update)
	} else if strings.HasSuffix(cmd, "wspam") {
		return handleWSpam(ctx, update)
	} else if strings.HasSuffix(cmd, "sspam") {
		return handleSSpam(ctx, update)
	} else if strings.HasSuffix(cmd, "spam") {
		return handleSpam(ctx, update)
	}

	return nil
}

func getRepliedMessage(ctx *ext.Context, update *ext.Update) (*tg.Message, error) {
	uMsg := update.EffectiveMessage
	if uMsg == nil {
		return nil, fmt.Errorf("tidak ada pesan")
	}
	reply, ok := uMsg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || reply.ReplyToMsgID == 0 {
		return nil, fmt.Errorf("tidak membalas ke pesan manapun")
	}
	msgs, err := ctx.GetMessages(update.EffectiveChat().GetID(), []tg.InputMessageClass{&tg.InputMessageID{ID: reply.ReplyToMsgID}})
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("pesan tidak ditemukan")
	}
	msg, ok := msgs[0].(*tg.Message)
	if !ok {
		return nil, fmt.Errorf("pesan bukan tipe regular message")
	}
	return msg, nil
}

func getMediaInput(media tg.MessageMediaClass) tg.InputMediaClass {
	if media == nil {
		return nil
	}
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		if photo, ok := m.Photo.(*tg.Photo); ok {
			return &tg.InputMediaPhoto{
				ID: photo.AsInput(),
			}
		}
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			return &tg.InputMediaDocument{
				ID: doc.AsInput(),
			}
		}
	case *tg.MessageMediaDice:
		return &tg.InputMediaDice{
			Emoticon: m.Emoticon,
		}
	case *tg.MessageMediaGeo:
		if geo, ok := m.Geo.(*tg.GeoPoint); ok {
			return &tg.InputMediaGeoPoint{
				GeoPoint: &tg.InputGeoPoint{
					Lat:            geo.Lat,
					Long:           geo.Long,
					AccuracyRadius: geo.AccuracyRadius,
				},
			}
		}
	case *tg.MessageMediaContact:
		return &tg.InputMediaContact{
			PhoneNumber: m.PhoneNumber,
			FirstName:   m.FirstName,
			LastName:    m.LastName,
			Vcard:       m.Vcard,
		}
	}
	return nil
}

func getSpamTextByArgs(fullText string, skipCount int) string {
	words := strings.Fields(fullText)
	if len(words) <= skipCount {
		return ""
	}

	pos := 0
	for i := 0; i < skipCount; i++ {
		word := words[i]
		idx := strings.Index(fullText[pos:], word)
		if idx == -1 {
			return ""
		}
		pos += idx + len(word)
	}

	return strings.TrimSpace(fullText[pos:])
}

func getRandomID() int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	return n.Int64()
}

func showError(ctx *ext.Context, update *ext.Update, errMsg string) {
	chat := update.EffectiveChat()
	msg := update.EffectiveMessage
	if chat == nil || msg == nil {
		return
	}
	text, entities := utils.ParseHTML("❌ <b>Error:</b> " + errMsg)
	_, _ = ctx.EditMessage(chat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       msg.ID,
		Message:  text,
		Entities: entities,
	})
}
