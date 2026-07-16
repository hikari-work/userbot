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
	"github.com/hikari-work/userbot/utils"
)

// toggleAutoDownload toggles auto-downloading feature on/off in Redis
func toggleAutoDownload(ctx *ext.Context, chatID int64, msgID int, action string) error {
	action = strings.ToLower(action)
	if action == "on" {
		err := dbClient.Redis.Set(ctx, "userbot:autodownload:ttl", "1", 0).Err()
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, chatID, msgID, fmt.Sprintf("❌ <b>Error:</b> %s", html.EscapeString(err.Error())))
			return err
		}
		localize := i18n.Localize("MediaAutoDLActv", nil, nil)
		_, _ = utils.EditMessageHTML(ctx, chatID, msgID, localize)
		return nil
	} else if action == "off" {
		err := dbClient.Redis.Del(ctx, "userbot:autodownload:ttl").Err()
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, chatID, msgID, fmt.Sprintf("❌ <b>Error:</b> %s", html.EscapeString(err.Error())))
			return err
		}
		localize := i18n.Localize("MediaAutoDLDeact", nil, nil)
		_, _ = utils.EditMessageHTML(ctx, chatID, msgID, localize)
		return nil
	}
	return fmt.Errorf("invalid action: %s", action)
}

// downloadBatch downloads multiple messages sequentially (download then upload, one at a time)
func downloadBatch(ctx *ext.Context, triggerChatID int64, triggerMsgID int, sourceChatID int64, startMsgID int, count int, targetChatID int64) {
	_, _ = utils.EditMessageHTML(ctx, triggerChatID, triggerMsgID,
		fmt.Sprintf("⏳ <b>Downloading %d messages</b> (ID %d - %d)...", count, startMsgID, startMsgID+count-1))

	for i := 0; i < count; i++ {
		id := startMsgID + i
		_, _ = utils.EditMessageHTML(ctx, triggerChatID, triggerMsgID,
			fmt.Sprintf("⏳ <b>Processing %d/%d</b> (ID %d)...", i+1, count, id))

		m, err := getMessage(ctx, sourceChatID, id)
		if err != nil {
			slog.Error("Batch download: failed to get message", "msgID", id, "error", err)
			continue
		}
		meta := determineFileInfo(m)
		outputPath, thumbPath, cleanup, err := downloadMediaHelper(ctx, m.Media, meta)
		if err != nil {
			slog.Error("Batch download: failed to download media", "msgID", id, "error", err)
			continue
		}
		err = uploadAndSendMedia(ctx, targetChatID, 0, outputPath, thumbPath, meta)
		if err != nil {
			slog.Error("Batch download: failed to upload media", "msgID", id, "error", err)
		}
		cleanup()
	}
	_, _ = utils.EditMessageHTML(ctx, triggerChatID, triggerMsgID,
		fmt.Sprintf("✅ <b>Batch download selesai</b> (%d messages)", count))
}

// downloadAndSendSingle downloads a single message media and sends/uploads it back
func downloadAndSendSingle(ctx *ext.Context, msg *tg.Message, targetChatID int64, triggerChatID int64, triggerMsgID int, isReply bool) {
	meta := determineFileInfo(msg)

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
			_, _ = utils.EditMessageHTML(ctx, triggerChatID, triggerMsgID, i18n.Localize("DownloadFailedDownloadMedia", map[string]interface{}{"Error": err.Error()}, nil))
		}
		return
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
		_ = uploadAndSendMedia(ctx, targetChatID, triggerMsgID, outputPath, thumbPath, meta)
	}
}
