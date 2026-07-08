package download

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

type MediaMetadata struct {
	FileName     string
	IsPhoto      bool
	IsVideo      bool
	VideoAttr    *tg.DocumentAttributeVideo
	MediaTypeStr string
	ThumbSize    string
	HasThumb     bool
	Document     *tg.Document
}

func init() {
	manager.Register(&manager.Module{
		Name:        "Download",
		Description: "Downloading from telegram and upload it back",
		Commands:    []string{"download", "dl"},
		OnlyOut:     true,
		Handler:     downloadHandler,
	})
}

func getRandomID() int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	return n.Int64()
}

func parseLink(link string) (peer string, isPrivate bool, msgID int, err error) {
	link = strings.TrimSpace(link)

	webRegex := regexp.MustCompile(`(?:t\.me|telegram\.(?:me|dog))/(c/)?([a-zA-Z0-9_]+)/([0-9]+)`)
	if matches := webRegex.FindStringSubmatch(link); len(matches) == 4 {
		isPrivate = matches[1] == "c/"
		peer = matches[2]
		msgID, err = strconv.Atoi(matches[3])
		return peer, isPrivate, msgID, err
	}

	tgResolveRegex := regexp.MustCompile(`tg://resolve\?domain=([a-zA-Z0-9_]+)&post=([0-9]+)`)
	if matches := tgResolveRegex.FindStringSubmatch(link); len(matches) == 3 {
		peer = matches[1]
		msgID, err = strconv.Atoi(matches[2])
		return peer, false, msgID, err
	}

	tgPrivateRegex := regexp.MustCompile(`tg://privatepost\?channel=([0-9\-]+)&post=([0-9]+)`)
	if matches := tgPrivateRegex.FindStringSubmatch(link); len(matches) == 3 {
		peer = matches[1]
		msgID, err = strconv.Atoi(matches[2])
		return peer, true, msgID, err
	}

	return "", false, 0, fmt.Errorf("format link not acceptable: %s", link)
}

func resolvePeer(ctx *ext.Context, peer string, isPrivate bool) (int64, error) {
	if isPrivate {
		parsedChatID, err := strconv.ParseInt(peer, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("chat ID not valid: %w", err)
		}
		if parsedChatID > 0 {
			if parsedChatID < 1000000000000 {
				parsedChatID = -1000000000000 - parsedChatID
			} else {
				parsedChatID = -parsedChatID
			}
		}
		_, err = ctx.ResolveInputPeerById(parsedChatID)
		if err != nil {
			return 0, fmt.Errorf("failed to resolve private chat ID %d: %w", parsedChatID, err)
		}
		return parsedChatID, nil
	}

	resolvedChat, err := ctx.ResolveUsername(peer)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve username '%s': %w", peer, err)
	}
	return resolvedChat.GetID(), nil
}

func getMessage(ctx *ext.Context, chatID int64, msgID int) (*tg.Message, error) {
	msgs, err := ctx.GetMessages(chatID, []tg.InputMessageClass{&tg.InputMessageID{ID: msgID}})
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("message not found")
	}

	msg, ok := msgs[0].(*tg.Message)
	if !ok {
		return nil, fmt.Errorf("message type not identified")
	}
	if msg.Media == nil {
		return nil, fmt.Errorf("message doesn't have media")
	}
	return msg, nil
}

func determineFileInfo(msg *tg.Message) (meta MediaMetadata) {
	meta.MediaTypeStr = "Dokumen/File"

	switch m := msg.Media.(type) {
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			meta.Document = doc
			for _, attr := range doc.Attributes {
				if fileAttr, ok := attr.(*tg.DocumentAttributeFilename); ok {
					meta.FileName = fileAttr.FileName
				}
				if vAttr, ok := attr.(*tg.DocumentAttributeVideo); ok {
					meta.IsVideo = true
					meta.VideoAttr = vAttr
				}
			}

			for _, t := range doc.Thumbs {
				switch thumb := t.(type) {
				case *tg.PhotoSize:
					meta.ThumbSize = thumb.Type
					meta.HasThumb = true
				case *tg.PhotoSizeProgressive:
					meta.ThumbSize = thumb.Type
					meta.HasThumb = true
				}
			}

			if meta.IsVideo {
				if meta.VideoAttr != nil && meta.VideoAttr.RoundMessage {
					meta.MediaTypeStr = "Video Bulat (Telescope)"
				} else {
					meta.MediaTypeStr = "Video"
				}
			} else {
				var isAudio, isVoice, isAnimation bool
				for _, attr := range doc.Attributes {
					if audioAttr, ok := attr.(*tg.DocumentAttributeAudio); ok {
						if audioAttr.Voice {
							isVoice = true
						} else {
							isAudio = true
						}
					}
					if _, ok := attr.(*tg.DocumentAttributeAnimated); ok {
						isAnimation = true
					}
				}
				if isVoice {
					meta.MediaTypeStr = "Voice Note"
				} else if isAudio {
					meta.MediaTypeStr = "Audio/Music"
				} else if isAnimation {
					meta.MediaTypeStr = "GIF/Animation"
				} else {
					meta.MediaTypeStr = "Document/File"
				}
			}

			if meta.FileName == "" {
				exts, _ := mime.ExtensionsByType(doc.MimeType)
				var extension string
				if len(exts) > 0 {
					extension = exts[0]
				} else {
					if meta.IsVideo {
						extension = ".mp4"
					} else {
						extension = ".bin"
					}
				}
				meta.FileName = fmt.Sprintf("doc_%d%s", doc.ID, extension)
			}
		} else {
			meta.FileName = fmt.Sprintf("doc_%d.bin", m.Document.GetID())
			meta.MediaTypeStr = "Document/File"
		}
	case *tg.MessageMediaPhoto:
		meta.IsPhoto = true
		meta.FileName = fmt.Sprintf("photo_%d.jpg", m.Photo.GetID())
		meta.MediaTypeStr = "Photo"
	default:
		meta.FileName = "media_file.bin"
		meta.MediaTypeStr = "Media Not Identified"
	}
	return meta
}

func downloadThumbnail(ctx *ext.Context, doc *tg.Document, thumbSize string, outputPath string) error {
	mediaDownloader := downloader.NewDownloader()
	loc := &tg.InputDocumentFileLocation{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
		ThumbSize:     thumbSize,
	}
	d := mediaDownloader.Download(ctx.Raw, loc)
	_, err := d.ToPath(ctx, outputPath)
	return err
}

func uploadAndSendMedia(ctx *ext.Context, chatID int64, triggerMsgID int, outputPath string, thumbPath string, meta MediaMetadata) error {
	slog.Info("status: running media uploader...")
	uploaderHelper := uploader.NewUploader(ctx.Raw)
	uploadedFile, err := uploaderHelper.FromPath(ctx, outputPath)
	if err != nil {
		slog.Error("status: failed to upload file", "error", err)
		_, _ = utils.EditMessageHTML(ctx, chatID, triggerMsgID, i18n.Localize(ctx, "DownloadFailedUpload", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}
	slog.Info("status: upload file success")

	var uploadedThumb tg.InputFileClass
	if thumbPath != "" {
		slog.Info("status: uploading thumbnail media...")
		tFile, err := uploaderHelper.FromPath(ctx, thumbPath)
		if err == nil {
			uploadedThumb = tFile
			slog.Info("status: Upload thumbnail success")
		} else {
			slog.Error("status: failed to upload thumbnail", "error", err)
		}
	}

	var reqMedia tg.InputMediaClass

	if meta.IsPhoto {
		slog.Info("status: prepare media photo...")
		reqMedia = &tg.InputMediaUploadedPhoto{
			File: uploadedFile,
		}
	} else if meta.IsVideo {
		slog.Info("status: prepare media video...")
		mimeType := mime.TypeByExtension(filepath.Ext(meta.FileName))
		if mimeType == "" {
			mimeType = "video/mp4"
		}
		var attrs []tg.DocumentAttributeClass
		attrs = append(attrs, &tg.DocumentAttributeFilename{
			FileName: meta.FileName,
		})
		if meta.VideoAttr != nil {
			attrs = append(attrs, meta.VideoAttr)
		} else {
			attrs = append(attrs, &tg.DocumentAttributeVideo{
				SupportsStreaming: true,
			})
		}
		reqMediaDoc := &tg.InputMediaUploadedDocument{
			File:       uploadedFile,
			MimeType:   mimeType,
			Attributes: attrs,
		}
		if uploadedThumb != nil {
			reqMediaDoc.SetThumb(uploadedThumb)
		}
		reqMedia = reqMediaDoc
	} else {
		slog.Info("status: prepare media document...")
		mimeType := mime.TypeByExtension(filepath.Ext(meta.FileName))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		reqMediaDoc := &tg.InputMediaUploadedDocument{
			File:      uploadedFile,
			MimeType:  mimeType,
			ForceFile: true,
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeFilename{
					FileName: meta.FileName,
				},
			},
		}
		if uploadedThumb != nil {
			reqMediaDoc.SetThumb(uploadedThumb)
		}
		reqMedia = reqMediaDoc
	}

	slog.Info("status: deleting trigger...")
	_ = ctx.DeleteMessages(chatID, []int{triggerMsgID})

	slog.Info("status: running SendMedia...")
	peer, err := ctx.ResolveInputPeerById(chatID)
	if err != nil {
		slog.Error("status: failed to resolve peer chat for media sending", "error", err)
		return err
	}

	htmlText := i18n.Localize(ctx, "DownloadCompleted", map[string]interface{}{
		"Type": meta.MediaTypeStr,
		"Name": meta.FileName,
	}, nil)
	text, entities := utils.ParseHTML(htmlText)

	_, err = ctx.SendMedia(chatID, &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    reqMedia,
		Message:  text,
		Entities: entities,
		RandomID: getRandomID(),
	})
	if err != nil {
		slog.Error("status: failed SendMedia", "error", err)
		return err
	}
	slog.Info("status: SendMedia success! Proses finished.")
	return nil
}

func downloadHandler(ctx *ext.Context, update *ext.Update) error {
	args := update.Args()
	if len(args) < 2 {
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize(ctx, "DownloadUsage", nil, nil))
		return fmt.Errorf("argument not found")
	}

	link := args[1]
	slog.Info("Starting proses download link", "link", link)

	_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize(ctx, "DownloadAnalyzing", nil, nil))

	peer, isPrivate, msgID, err := parseLink(link)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize(ctx, "DownloadFailedAnalyze", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	chatID, err := resolvePeer(ctx, peer, isPrivate)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize(ctx, "DownloadFailedResolveChat", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize(ctx, "DownloadGettingData", nil, nil))

	msg, err := getMessage(ctx, chatID, msgID)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize(ctx, "DownloadFailedGetMsg", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	meta := determineFileInfo(msg)

	_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize(ctx, "DownloadDownloading", map[string]interface{}{
		"Type": meta.MediaTypeStr,
		"Name": meta.FileName,
	}, nil))

	err = os.MkdirAll("downloads", 0755)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize(ctx, "DownloadFailedMkdir", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	outputPath := filepath.Join("downloads", meta.FileName)
	defer func() {
		slog.Info("status: Deleting local file...", "path", outputPath)
		_ = os.Remove(outputPath)
	}()

	var thumbPath string
	if meta.HasThumb && meta.Document != nil {
		thumbPath = filepath.Join("downloads", fmt.Sprintf("thumb_%d.jpg", meta.Document.ID))
		defer func() {
			slog.Info("status: deleting local thumbnail...", "path", thumbPath)
			_ = os.Remove(thumbPath)
		}()

		slog.Info("status: starting download thumbnail...", "thumbSize", meta.ThumbSize, "thumbPath", thumbPath)
		err = downloadThumbnail(ctx, meta.Document, meta.ThumbSize, thumbPath)
		if err != nil {
			slog.Error("status: failed to download thumbnail", "error", err)
			thumbPath = ""
		} else {
			slog.Info("status: Download thumbnail success")
		}
	}

	slog.Info("status: Starting DownloadMedia...", "outputPath", outputPath)
	_, err = ctx.DownloadMedia(msg.Media, ext.DownloadOutputPath(outputPath), nil)
	if err != nil {
		slog.Error("status: Failed DownloadMedia", "error", err)
		_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize(ctx, "DownloadFailedDownloadMedia", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}
	slog.Info("status: DownloadMedia success")

	_, _ = utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, i18n.Localize(ctx, "DownloadUploading", map[string]interface{}{"Name": meta.FileName}, nil))

	err = uploadAndSendMedia(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, outputPath, thumbPath, meta)
	return err
}
