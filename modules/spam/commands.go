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

func handleSpam(ctx *ext.Context, update *ext.Update) error {
	uMsg := update.EffectiveMessage
	uChat := update.EffectiveChat()
	if uMsg == nil || uChat == nil {
		return nil
	}

	args := update.Args()
	if len(args) < 2 {
		showError(ctx, update, i18n.Localize("SpamHelpError", nil, nil))
		return nil
	}

	count, err := strconv.Atoi(args[1])
	if err != nil || count <= 0 {
		showError(ctx, update, i18n.Localize("SpamInvalidCount", nil, nil))
		return nil
	}

	repliedMsg, replyErr := getRepliedMessage(ctx, update)

	var textToSpam string
	var entitiesToSpam []tg.MessageEntityClass
	var inputMedia tg.InputMediaClass
	hasMedia := false

	if replyErr == nil {
		if repliedMsg.Media != nil {
			inputMedia = getMediaInput(repliedMsg.Media)
			if inputMedia != nil {
				hasMedia = true
			}
		}
		textToSpam = repliedMsg.Message
		entitiesToSpam = repliedMsg.Entities
	} else {
		textToSpam = getSpamTextByArgs(uMsg.Message.Message, 2)
		if textToSpam == "" {
			showError(ctx, update, i18n.Localize("SpamMissingText", nil, nil))
			return nil
		}
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
			var sendErr error
			if hasMedia {
				_, sendErr = bgCtx.SendMedia(uChat.GetID(), &tg.MessagesSendMediaRequest{
					Peer:     peer,
					Media:    inputMedia,
					Message:  textToSpam,
					Entities: entitiesToSpam,
					RandomID: getRandomID(),
				})
			} else {
				_, sendErr = bgCtx.SendMessage(uChat.GetID(), &tg.MessagesSendMessageRequest{
					Peer:     peer,
					Message:  textToSpam,
					Entities: entitiesToSpam,
					RandomID: getRandomID(),
				})
			}
			if sendErr != nil {
				slog.Error("Failed to send spam message", "error", sendErr)
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	return nil
}

func handleDelaySpam(ctx *ext.Context, update *ext.Update) error {
	uMsg := update.EffectiveMessage
	uChat := update.EffectiveChat()
	if uMsg == nil || uChat == nil {
		return nil
	}

	args := update.Args()
	if len(args) < 3 {
		showError(ctx, update, i18n.Localize("DelaySpamHelpError", nil, nil))
		return nil
	}

	delaySec, err := strconv.ParseFloat(args[1], 64)
	if err != nil || delaySec < 0.1 {
		showError(ctx, update, i18n.Localize("DelaySpamInvalidDelay", nil, nil))
		return nil
	}

	count, err := strconv.Atoi(args[2])
	if err != nil || count <= 0 {
		showError(ctx, update, i18n.Localize("SpamInvalidCount", nil, nil))
		return nil
	}

	repliedMsg, replyErr := getRepliedMessage(ctx, update)

	var textToSpam string
	var entitiesToSpam []tg.MessageEntityClass
	var inputMedia tg.InputMediaClass
	hasMedia := false

	if replyErr == nil {
		if repliedMsg.Media != nil {
			inputMedia = getMediaInput(repliedMsg.Media)
			if inputMedia != nil {
				hasMedia = true
			}
		}
		textToSpam = repliedMsg.Message
		entitiesToSpam = repliedMsg.Entities
	} else {
		textToSpam = getSpamTextByArgs(uMsg.Message.Message, 3)
		if textToSpam == "" {
			showError(ctx, update, i18n.Localize("SpamMissingText", nil, nil))
			return nil
		}
	}

	_ = ctx.DeleteMessages(uChat.GetID(), []int{uMsg.ID})

	bgCtx := *ctx
	bgCtx.Context = context.Background()
	peer, peerErr := bgCtx.ResolveInputPeerById(uChat.GetID())
	if peerErr != nil {
		slog.Error("Failed to resolve peer", "error", peerErr)
		return nil
	}

	delayDuration := time.Duration(delaySec * float64(time.Second))

	go func() {
		for i := 0; i < count; i++ {
			var sendErr error
			if hasMedia {
				_, sendErr = bgCtx.SendMedia(uChat.GetID(), &tg.MessagesSendMediaRequest{
					Peer:     peer,
					Media:    inputMedia,
					Message:  textToSpam,
					Entities: entitiesToSpam,
					RandomID: getRandomID(),
				})
			} else {
				_, sendErr = bgCtx.SendMessage(uChat.GetID(), &tg.MessagesSendMessageRequest{
					Peer:     peer,
					Message:  textToSpam,
					Entities: entitiesToSpam,
					RandomID: getRandomID(),
				})
			}
			if sendErr != nil {
				slog.Error("Failed to send delayspam message", "error", sendErr)
				break
			}
			if i < count-1 {
				time.Sleep(delayDuration)
			}
		}
	}()

	return nil
}
