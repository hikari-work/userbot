package quote

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "Quote",
		Description: "Generate a quote sticker from replied message",
		Commands:    []string{"q", "quote"},
		OnlyOut:     true,
		Handler:     quoteHandler,
	})
}

func quoteHandler(ctx *ext.Context, update *ext.Update) error {
	uMsg := update.EffectiveMessage
	uChat := update.EffectiveChat()
	if uMsg == nil || uChat == nil {
		return nil
	}

	reply, ok := uMsg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || reply.ReplyToMsgID == 0 {
		text, entities := utils.ParseHTML(i18n.Localize("QuoteReplyRequired", nil, nil))
		_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		return nil
	}

	loadingText, loadingEntities := utils.ParseHTML(i18n.Localize("QuoteGenerating", nil, nil))
	_, _ = ctx.EditMessage(uChat.GetID(), &tg.MessagesEditMessageRequest{
		ID:       uMsg.ID,
		Message:  loadingText,
		Entities: loadingEntities,
	})

	count := 1
	bgColor := "#1b1429"
	args := update.Args()
	if len(args) > 1 {
		for _, arg := range args[1:] {
			if val, err := strconv.Atoi(arg); err == nil && val > 0 {
				count = val
			} else if strings.HasPrefix(arg, "#") || strings.HasPrefix(arg, "//") {
				bgColor = arg
			}
		}
	}
	if count > 20 {
		count = 20
	}

	bgCtx := *ctx
	bgCtx.Context = context.Background()

	go func() {
		var messageIDs []int
		for i := 0; i < count*3; i++ {
			messageIDs = append(messageIDs, reply.ReplyToMsgID+i)
		}

		msgs, tgUsers, tgChats, err := fetchMessages(&bgCtx, uChat.GetID(), messageIDs)
		if err != nil {
			slog.Error("Quote error fetching messages", "error", err)
			_ = editWithError(&bgCtx, uChat.GetID(), uMsg.ID, err)
			return
		}

		var validMsgs []*tg.Message
		for _, m := range msgs {
			msgObj, ok := m.(*tg.Message)
			if !ok {
				continue
			}
			validMsgs = append(validMsgs, msgObj)
		}

		if len(validMsgs) == 0 {
			slog.Error("Quote: no valid messages found")
			_ = editWithError(&bgCtx, uChat.GetID(), uMsg.ID, fmt.Errorf("no messages found to quote"))
			return
		}

		sort.Slice(validMsgs, func(i, j int) bool {
			return validMsgs[i].ID < validMsgs[j].ID
		})

		if len(validMsgs) > count {
			validMsgs = validMsgs[:count]
		}

		var replyMessage *ReplyMessage
		if len(validMsgs) > 0 {
			if firstReply, ok := validMsgs[0].ReplyTo.(*tg.MessageReplyHeader); ok && firstReply.ReplyToMsgID != 0 {
				rMsgs, rUsers, rChats, rErr := fetchMessages(&bgCtx, uChat.GetID(), []int{firstReply.ReplyToMsgID})
				if rErr == nil && len(rMsgs) > 0 {
					if rMsg, ok := rMsgs[0].(*tg.Message); ok {
						rSender := resolveUserObj(rMsg.FromID, rUsers, rChats)
						senderName := rSender.FirstName
						if rSender.LastName != "" {
							senderName += " " + rSender.LastName
						}
						var rPhoto ReplyPhoto
						if rSender.Photo.Url != "" {
							rPhoto.Url = rSender.Photo.Url
						}
						replyMessage = &ReplyMessage{
							Name:   senderName,
							Text:   getMessageText(rMsg),
							ChatId: int(uChat.GetID()),
							From: ReplyFrom{
								Id:    rSender.Id,
								Name:  senderName,
								Photo: rPhoto,
							},
						}
					}
				}
			}
		}

		var quoteMsgs []Message
		for idx, m := range validMsgs {
			sender := resolveUserObj(m.FromID, tgUsers, tgChats)
			msgText := getMessageText(m)
			mappedEnts := mapEntities(m.Entities)

			var fromObj User
			fromObj.Id = sender.Id
			fromObj.FirstName = sender.FirstName
			fromObj.LastName = sender.LastName
			fromObj.Username = sender.Username
			fromObj.Photo = sender.Photo

			var mediaObj *MessageMedia
			if m.Media != nil {
				shouldUpload := false
				switch m.Media.(type) {
				case *tg.MessageMediaPhoto:
					shouldUpload = true
				case *tg.MessageMediaDocument:
					shouldUpload = true
				}

				if shouldUpload {
					base64Data, err := mediaToBase64(&bgCtx, m.Media)
					if err == nil && base64Data != "" {
						mediaObj = &MessageMedia{Url: base64Data}
					}
				}
			}

			qMsg := Message{
				From:     fromObj,
				Text:     msgText,
				Entities: mappedEnts,
				Avatar:   true,
				Media:    mediaObj,
			}

			if idx == 0 && replyMessage != nil {
				qMsg.ReplyMessage = replyMessage
			}

			quoteMsgs = append(quoteMsgs, qMsg)
		}

		quoteReq := &QuoteRequest{
			BackgroundColor: bgColor,
			Width:           512,
			Height:          768,
			Scale:           2,
			EmojiBrand:      "apple",
			Messages:        quoteMsgs,
		}

		respBytes, err := request(quoteReq)
		if err != nil {
			slog.Error("Quote API request failed", "error", err)
			_ = editWithError(&bgCtx, uChat.GetID(), uMsg.ID, err)
			return
		}

		uploaderHelper := uploader.NewUploader(bgCtx.Raw)
		uploadedFile, err := uploaderHelper.FromBytes(&bgCtx, "sticker.webp", respBytes)
		if err != nil {
			slog.Error("Quote upload failed", "error", err)
			_ = editWithError(&bgCtx, uChat.GetID(), uMsg.ID, fmt.Errorf("failed to upload sticker: %w", err))
			return
		}

		attrs := []tg.DocumentAttributeClass{
			&tg.DocumentAttributeSticker{
				Alt:        "💬",
				Stickerset: &tg.InputStickerSetEmpty{},
			},
			&tg.DocumentAttributeFilename{
				FileName: "sticker.webp",
			},
		}
		reqMediaDoc := &tg.InputMediaUploadedDocument{
			File:       uploadedFile,
			MimeType:   "image/webp",
			Attributes: attrs,
		}

		peer, err := bgCtx.ResolveInputPeerById(uChat.GetID())
		if err != nil {
			slog.Error("Quote peer resolve failed", "error", err)
			_ = editWithError(&bgCtx, uChat.GetID(), uMsg.ID, err)
			return
		}

		_, err = bgCtx.SendMedia(uChat.GetID(), &tg.MessagesSendMediaRequest{
			Peer:     peer,
			Media:    reqMediaDoc,
			RandomID: getRandomID(),
		})
		if err != nil {
			slog.Error("Quote SendMedia failed", "error", err)
			_ = editWithError(&bgCtx, uChat.GetID(), uMsg.ID, fmt.Errorf("failed to send sticker: %w", err))
			return
		}

		_ = bgCtx.DeleteMessages(uChat.GetID(), []int{uMsg.ID})
	}()

	return nil
}
