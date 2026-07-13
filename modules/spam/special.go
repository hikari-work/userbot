package spam

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/i18n"
)

func handleWSpam(ctx *ext.Context, update *ext.Update) error {
	uMsg := update.EffectiveMessage
	uChat := update.EffectiveChat()
	if uMsg == nil || uChat == nil {
		return nil
	}

	repliedMsg, replyErr := getRepliedMessage(ctx, update)
	if replyErr != nil {
		showError(ctx, update, i18n.Localize("WSpamReplyRequired", nil, nil))
		return nil
	}

	if repliedMsg.Message == "" {
		showError(ctx, update, i18n.Localize("WSpamNoText", nil, nil))
		return nil
	}

	runes := []rune(repliedMsg.Message)

	_ = ctx.DeleteMessages(uChat.GetID(), []int{uMsg.ID})

	bgCtx := *ctx
	bgCtx.Context = context.Background()
	peer, peerErr := bgCtx.ResolveInputPeerById(uChat.GetID())
	if peerErr != nil {
		slog.Error("Failed to resolve peer", "error", peerErr)
		return nil
	}

	go func() {
		for _, r := range runes {
			charStr := string(r)
			_, sendErr := bgCtx.SendMessage(uChat.GetID(), &tg.MessagesSendMessageRequest{
				Peer:     peer,
				Message:  charStr,
				RandomID: getRandomID(),
			})
			if sendErr != nil {
				slog.Error("Failed to send wspam character", "error", sendErr)
				break
			}
			time.Sleep(150 * time.Millisecond)
		}
	}()

	return nil
}

func handleSSpam(ctx *ext.Context, update *ext.Update) error {
	uMsg := update.EffectiveMessage
	uChat := update.EffectiveChat()
	if uMsg == nil || uChat == nil {
		return nil
	}

	args := update.Args()
	if len(args) < 2 {
		showError(ctx, update, i18n.Localize("SSpamHelpError", nil, nil))
		return nil
	}

	count, err := strconv.Atoi(args[1])
	if err != nil || count <= 0 {
		showError(ctx, update, i18n.Localize("SSpamInvalidCount", nil, nil))
		return nil
	}

	repliedMsg, replyErr := getRepliedMessage(ctx, update)
	if replyErr != nil {
		showError(ctx, update, i18n.Localize("SSpamReplyRequired", nil, nil))
		return nil
	}

	mediaDoc, ok := repliedMsg.Media.(*tg.MessageMediaDocument)
	if !ok {
		showError(ctx, update, i18n.Localize("SSpamNotSticker", nil, nil))
		return nil
	}

	doc, ok := mediaDoc.Document.(*tg.Document)
	if !ok {
		showError(ctx, update, i18n.Localize("SSpamNotSticker", nil, nil))
		return nil
	}

	isSticker := false
	for _, attr := range doc.Attributes {
		if _, ok := attr.(*tg.DocumentAttributeSticker); ok {
			isSticker = true
			break
		}
	}

	if !isSticker {
		showError(ctx, update, i18n.Localize("SSpamNotSticker", nil, nil))
		return nil
	}

	inputMedia := &tg.InputMediaDocument{
		ID: doc.AsInput(),
	}

	_ = ctx.DeleteMessages(uChat.GetID(), []int{uMsg.ID})

	bgCtx := *ctx
	bgCtx.Context = context.Background()
	peer, peerErr := bgCtx.ResolveInputPeerById(uChat.GetID())
	if peerErr != nil {
		slog.Error("Failed to resolve peer", "error", peerErr)
		return nil
	}

	go func() {
		for i := 0; i < count; i++ {
			_, sendErr := bgCtx.SendMedia(uChat.GetID(), &tg.MessagesSendMediaRequest{
				Peer:     peer,
				Media:    inputMedia,
				RandomID: getRandomID(),
			})
			if sendErr != nil {
				slog.Error("Failed to send sticker spam", "error", sendErr)
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	return nil
}
