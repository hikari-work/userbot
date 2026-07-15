package download

import (
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "Download",
		Description: "Downloading from telegram and upload it back",
		Commands:    []string{"download", "dl"},
		OnlyOut:     true,
		Handler:     downloadHandler,
		OnMessage:   autoForward,
		Help:        downloadHelp,
	})
}

func downloadHelp() string {
	return "Format \n<code>.dl &lt;link_pesan&gt;</code>\n<code>Contoh : .dl https://t.me/c/123456789/123</code>"
}

func downloadHandler(ctx *ext.Context, update *ext.Update) error {
	args := update.Args()
	message := update.EffectiveMessage
	uChat := update.EffectiveChat()

	var replyHeader *tg.MessageReplyHeader
	if message.ReplyTo != nil {
		if r, ok := message.ReplyTo.(*tg.MessageReplyHeader); ok {
			replyHeader = r
		}
	}
	isReply := replyHeader != nil && replyHeader.ReplyToMsgID != 0

	if !isReply && len(args) < 2 {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), message.ID, i18n.Localize("DownloadUsage", nil, nil))
		return fmt.Errorf("argument not found")
	}

	if isReply {
		defer func() {
			_ = ctx.DeleteMessages(uChat.GetID(), []int{message.ID})
		}()
	}

	var targetChatID int64
	if isReply {
		targetChatID = ctx.Self.ID
	} else {
		targetChatID = uChat.GetID()
	}

	var msg *tg.Message
	var err error

	if isReply {
		msg, err = getMessage(ctx, uChat.GetID(), replyHeader.ReplyToMsgID)
		if err != nil {
			peer, pErr := ctx.ResolveInputPeerById(ctx.Self.ID)
			if pErr == nil {
				_, _ = ctx.SendMessage(ctx.Self.ID, &tg.MessagesSendMessageRequest{
					Peer:     peer,
					Message:  i18n.Localize("DownloadFailedGetMsg", map[string]interface{}{"Error": err.Error()}, nil),
					RandomID: getRandomID(),
				})
			}
			return err
		}
	} else {
		link := args[1]
		slog.Info("Starting proses download link", "link", link)
		if "on" == strings.ToLower(link) {
			err := dbClient.Redis.Set(ctx, "userbot:autodownload:ttl", "1", 0).Err()
			if err != nil {
				_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), message.ID, fmt.Sprintf("❌ <b>Error:</b> %s", html.EscapeString(err.Error())))
				return err
			}
			localize := i18n.Localize("MediaAutoDLActv", nil, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), message.ID, localize)
			return nil
		}
		if "off" == strings.ToLower(link) {
			err := dbClient.Redis.Del(ctx, "userbot:autodownload:ttl").Err()
			if err != nil {
				_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), message.ID, fmt.Sprintf("❌ <b>Error:</b> %s", html.EscapeString(err.Error())))
				return err
			}
			localize := i18n.Localize("MediaAutoDLDeact", nil, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), message.ID, localize)
			return nil
		}

		peer, isPrivate, msgID, err := parseLink(link)
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), message.ID, i18n.Localize("DownloadFailedAnalyze", map[string]interface{}{"Error": err.Error()}, nil))
			return err
		}

		chatID, err := resolvePeer(ctx, peer, isPrivate)
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), message.ID, i18n.Localize("DownloadFailedResolveChat", map[string]interface{}{"Error": err.Error()}, nil))
			return err
		}

		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), message.ID, i18n.Localize("DownloadGettingData", nil, nil))

		msg, err = getMessage(ctx, chatID, msgID)
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), message.ID, i18n.Localize("DownloadFailedGetMsg", map[string]interface{}{"Error": err.Error()}, nil))
			return err
		}
	}

	meta := determineFileInfo(msg)

	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), message.ID, i18n.Localize("DownloadDownloading", map[string]interface{}{
		"Type": meta.MediaTypeStr,
		"Name": meta.FileName,
	}, nil))

	outputPath, thumbPath, cleanup, err := downloadMediaHelper(ctx, msg.Media, meta)
	if err != nil {
		if isReply {
			peer, pErr := ctx.ResolveInputPeerById(ctx.Self.ID)
			if pErr == nil {
				_, _ = ctx.SendMessage(ctx.Self.ID, &tg.MessagesSendMessageRequest{
					Peer:     peer,
					Message:  i18n.Localize("DownloadFailedDownloadMedia", map[string]interface{}{"Error": err.Error()}, nil),
					RandomID: getRandomID(),
				})
			}
		} else {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), message.ID, i18n.Localize("DownloadFailedDownloadMedia", map[string]interface{}{"Error": err.Error()}, nil))
		}
		return err
	}
	defer cleanup()

	if isReply {
		err = uploadAndSendMedia(ctx, targetChatID, 0, outputPath, thumbPath, meta)
		if err != nil {
			peer, pErr := ctx.ResolveInputPeerById(ctx.Self.ID)
			if pErr == nil {
				_, _ = ctx.SendMessage(ctx.Self.ID, &tg.MessagesSendMessageRequest{
					Peer:     peer,
					Message:  i18n.Localize("DownloadFailedUpload", map[string]interface{}{"Error": err.Error()}, nil),
					RandomID: getRandomID(),
				})
			}
		}
	} else {
		err = uploadAndSendMedia(ctx, targetChatID, message.ID, outputPath, thumbPath, meta)
	}

	return err
}

func autoForward(ctx *ext.Context, update *ext.Update) error {
	msg := update.EffectiveMessage
	if msg == nil || msg.Media == nil {
		return nil
	}

	enabled, err := dbClient.Redis.Exists(ctx, "userbot:autodownload:ttl").Result()
	if err != nil || enabled == 0 {
		return nil
	}

	if !isViewOnce(msg.Media) {
		return nil
	}

	slog.Info("Deteksi media sekali lihat (TTLSeconds > 0) dan autodownload aktif. Memulai pengunduhan...")

	meta := determineFileInfo(msg.Message)

	outputPath, thumbPath, cleanup, err := downloadMediaHelper(ctx, msg.Media, meta)
	if err != nil {
		slog.Error("status: Failed to download media", "error", err)
		return err
	}
	defer cleanup()

	err = uploadAndSendMedia(ctx, ctx.Self.ID, 0, outputPath, thumbPath, meta)
	if err != nil {
		slog.Error("status: Failed to upload and send auto-downloaded media", "error", err)
		return err
	}

	return nil
}
