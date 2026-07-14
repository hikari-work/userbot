package download

import (
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/celestix/gotgproto/ext"
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
	if len(args) < 2 {
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize("DownloadUsage", nil, nil))
		return fmt.Errorf("argument not found")
	}

	link := args[1]
	slog.Info("Starting proses download link", "link", link)
	if "on" == strings.ToLower(link) {
		err := dbClient.Redis.Set(ctx, "userbot:autodownload:ttl", "1", 0).Err()
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, fmt.Sprintf("❌ <b>Error:</b> %s", html.EscapeString(err.Error())))
			return err
		}
		localize := i18n.Localize("MediaAutoDLActv", nil, nil)
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, localize)
		return nil
	}
	if "off" == strings.ToLower(link) {
		err := dbClient.Redis.Del(ctx, "userbot:autodownload:ttl").Err()
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, fmt.Sprintf("❌ <b>Error:</b> %s", html.EscapeString(err.Error())))
			return err
		}
		localize := i18n.Localize("MediaAutoDLDeact", nil, nil)
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, localize)
		return nil
	}

	_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize("DownloadAnalyzing", nil, nil))

	peer, isPrivate, msgID, err := parseLink(link)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize("DownloadFailedAnalyze", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	chatID, err := resolvePeer(ctx, peer, isPrivate)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize("DownloadFailedResolveChat", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize("DownloadGettingData", nil, nil))

	msg, err := getMessage(ctx, chatID, msgID)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize("DownloadFailedGetMsg", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	meta := determineFileInfo(msg)

	_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize("DownloadDownloading", map[string]interface{}{
		"Type": meta.MediaTypeStr,
		"Name": meta.FileName,
	}, nil))

	outputPath, thumbPath, cleanup, err := downloadMediaHelper(ctx, msg.Media, meta)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize("DownloadFailedDownloadMedia", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}
	defer cleanup()

	_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize("DownloadUploading", map[string]interface{}{"Name": meta.FileName}, nil))

	err = uploadAndSendMedia(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, outputPath, thumbPath, meta)
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

