package download

import (
	"fmt"
	"log/slog"
	"mime"
	"os"
	"path/filepath"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/i18n"
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
		if triggerMsgID > 0 {
			_, _ = utils.EditMessageHTML(ctx, chatID, triggerMsgID, i18n.Localize("DownloadFailedUpload", map[string]interface{}{"Error": err.Error()}, nil))
		}
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

	if triggerMsgID > 0 {
		slog.Info("status: deleting trigger...")
		_ = ctx.DeleteMessages(chatID, []int{triggerMsgID})
	}

	slog.Info("status: running SendMedia...")
	peer, err := ctx.ResolveInputPeerById(chatID)
	if err != nil {
		slog.Error("status: failed to resolve peer chat for media sending", "error", err)
		return err
	}

	htmlText := i18n.Localize("DownloadCompleted", map[string]interface{}{
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

func downloadMediaHelper(ctx *ext.Context, media tg.MessageMediaClass, meta MediaMetadata) (outputPath string, thumbPath string, cleanup func(), err error) {
	err = os.MkdirAll("downloads", 0755)
	if err != nil {
		return "", "", nil, err
	}

	outputPath = filepath.Join("downloads", meta.FileName)
	cleanups := []func(){
		func() {
			slog.Info("status: Deleting local file...", "path", outputPath)
			_ = os.Remove(outputPath)
		},
	}

	if meta.HasThumb && meta.Document != nil {
		tPath := filepath.Join("downloads", fmt.Sprintf("thumb_%d.jpg", meta.Document.ID))
		slog.Info("status: starting download thumbnail...", "thumbSize", meta.ThumbSize, "thumbPath", tPath)
		tErr := downloadThumbnail(ctx, meta.Document, meta.ThumbSize, tPath)
		if tErr != nil {
			slog.Error("status: failed to download thumbnail", "error", tErr)
		} else {
			thumbPath = tPath
			cleanups = append(cleanups, func() {
				slog.Info("status: deleting local thumbnail...", "path", tPath)
				_ = os.Remove(tPath)
			})
		}
	}

	slog.Info("status: Starting DownloadMedia...", "outputPath", outputPath)
	_, err = ctx.DownloadMedia(media, ext.DownloadOutputPath(outputPath), nil)
	if err != nil {
		slog.Error("status: Failed DownloadMedia", "error", err)
		for _, clean := range cleanups {
			clean()
		}
		return "", "", nil, err
	}
	slog.Info("status: DownloadMedia success")

	cleanup = func() {
		for _, clean := range cleanups {
			clean()
		}
	}

	return outputPath, thumbPath, cleanup, nil
}

func isViewOnce(media tg.MessageMediaClass) bool {
	if media == nil {
		return false
	}
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		return int64(m.TTLSeconds) > 0
	case *tg.MessageMediaDocument:
		return int64(m.TTLSeconds) > 0
	}
	return false
}
