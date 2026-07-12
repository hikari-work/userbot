package quote

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/utils"
)

func fetchMessages(ctx *ext.Context, chatID int64, messageIDs []int) ([]tg.MessageClass, []tg.UserClass, []tg.ChatClass, error) {
	var toFetch []tg.InputMessageClass
	for _, id := range messageIDs {
		toFetch = append(toFetch, &tg.InputMessageID{ID: id})
	}

	inputPeer, err := ctx.ResolveInputPeerById(chatID)
	if err != nil {
		return nil, nil, nil, err
	}

	var messagesClass tg.MessagesMessagesClass
	var fetchErr error

	switch p := inputPeer.(type) {
	case *tg.InputPeerChannel:
		messagesClass, fetchErr = ctx.Raw.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{
				ChannelID:  p.ChannelID,
				AccessHash: p.AccessHash,
			},
			ID: toFetch,
		})
	default:
		messagesClass, fetchErr = ctx.Raw.MessagesGetMessages(ctx, toFetch)
	}

	if fetchErr != nil {
		return nil, nil, nil, fetchErr
	}

	var msgs []tg.MessageClass
	var users []tg.UserClass
	var chats []tg.ChatClass

	if modified, ok := messagesClass.AsModified(); ok {
		msgs = modified.GetMessages()
		users = modified.GetUsers()
		chats = modified.GetChats()
	} else {
		switch m := messagesClass.(type) {
		case *tg.MessagesMessages:
			msgs = m.Messages
			users = m.Users
			chats = m.Chats
		case *tg.MessagesMessagesSlice:
			msgs = m.Messages
			users = m.Users
			chats = m.Chats
		case *tg.MessagesChannelMessages:
			msgs = m.Messages
			users = m.Users
			chats = m.Chats
		}
	}
	return msgs, users, chats, nil
}

func resolveUserObj(fromID tg.PeerClass, tgUsers []tg.UserClass, tgChats []tg.ChatClass) User {
	var user User
	if fromID == nil {
		user.FirstName = "Anonymous"
		return user
	}

	switch peer := fromID.(type) {
	case *tg.PeerUser:
		for _, uClass := range tgUsers {
			u, ok := uClass.(*tg.User)
			if ok && u.ID == peer.UserID {
				user.Id = int(u.ID)
				user.FirstName = u.FirstName
				user.LastName = u.LastName
				user.Username = u.Username
				if u.Username != "" {
					user.Photo = Photo{
						Url: fmt.Sprintf("https://t.me/i/userpic/320/%s.jpg", u.Username),
					}
				}
				return user
			}
		}
	case *tg.PeerChannel:
		for _, cClass := range tgChats {
			c, ok := cClass.(*tg.Channel)
			if ok && c.ID == peer.ChannelID {
				user.Id = int(c.ID)
				user.FirstName = c.Title
				user.Username = c.Username
				if c.Username != "" {
					user.Photo = Photo{
						Url: fmt.Sprintf("https://t.me/i/userpic/320/%s.jpg", c.Username),
					}
				}
				return user
			}
		}
	case *tg.PeerChat:
		for _, cClass := range tgChats {
			c, ok := cClass.(*tg.Chat)
			if ok && c.ID == peer.ChatID {
				user.Id = int(c.ID)
				user.FirstName = c.Title
				return user
			}
		}
	}
	return user
}

func getMessageText(m *tg.Message) string {
	if m.Message != "" {
		return m.Message
	}
	if m.Media != nil {
		switch media := m.Media.(type) {
		case *tg.MessageMediaPhoto:
			return "📷 Photo"
		case *tg.MessageMediaDocument:
			doc, ok := media.Document.(*tg.Document)
			if ok {
				for _, attr := range doc.Attributes {
					switch attr.(type) {
					case *tg.DocumentAttributeSticker:
						return "🎨 Sticker"
					case *tg.DocumentAttributeAnimated:
						return "📹 GIF"
					case *tg.DocumentAttributeVideo:
						return "🎥 Video"
					case *tg.DocumentAttributeAudio:
						return "🎵 Audio"
					}
				}
			}
			return "📄 Document"
		case *tg.MessageMediaGeo:
			return "📍 Location"
		case *tg.MessageMediaContact:
			return "👤 Contact"
		case *tg.MessageMediaPoll:
			return "📊 Poll"
		case *tg.MessageMediaDice:
			return "🎲 Dice"
		case *tg.MessageMediaWebPage:
			return "🔗 Link"
		default:
			return "📦 Media"
		}
	}
	return ""
}

func mapEntities(tgEntities []tg.MessageEntityClass) []Entity {
	var list []Entity
	for _, e := range tgEntities {
		var ent Entity
		switch v := e.(type) {
		case *tg.MessageEntityBold:
			ent = Entity{Type: "bold", Offset: v.Offset, Length: v.Length}
		case *tg.MessageEntityItalic:
			ent = Entity{Type: "italic", Offset: v.Offset, Length: v.Length}
		case *tg.MessageEntityCode:
			ent = Entity{Type: "code", Offset: v.Offset, Length: v.Length}
		case *tg.MessageEntityPre:
			ent = Entity{Type: "pre", Offset: v.Offset, Length: v.Length}
		case *tg.MessageEntityUnderline:
			ent = Entity{Type: "underline", Offset: v.Offset, Length: v.Length}
		case *tg.MessageEntityStrike:
			ent = Entity{Type: "strikethrough", Offset: v.Offset, Length: v.Length}
		case *tg.MessageEntitySpoiler:
			ent = Entity{Type: "spoiler", Offset: v.Offset, Length: v.Length}
		default:
			continue
		}
		list = append(list, ent)
	}
	return list
}

func getRandomID() int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	return n.Int64()
}

func editWithError(ctx *ext.Context, chatID int64, msgID int, err error) error {
	errText := i18n.Localize("QuoteFailed", map[string]interface{}{"Error": err.Error()}, nil)
	text, entities := utils.ParseHTML(errText)
	_, _ = ctx.EditMessage(chatID, &tg.MessagesEditMessageRequest{
		ID:       msgID,
		Message:  text,
		Entities: entities,
	})
	return nil
}

func mediaToBase64(ctx *ext.Context, media tg.MessageMediaClass) (string, error) {
	var extStr string
	var mimeType string

	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		extStr = ".jpg"
		mimeType = "image/jpeg"
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			for _, attr := range doc.Attributes {
				if fileAttr, ok := attr.(*tg.DocumentAttributeFilename); ok {
					extStr = filepath.Ext(fileAttr.FileName)
				}
			}
			if extStr == "" {
				extStr = ".webp"
			}
		} else {
			extStr = ".webp"
		}
		if extStr == ".png" {
			mimeType = "image/png"
		} else if extStr == ".jpg" || extStr == ".jpeg" {
			mimeType = "image/jpeg"
		} else {
			mimeType = "image/webp"
		}
	default:
		return "", fmt.Errorf("unsupported media type")
	}

	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("quote_media_%d%s", getRandomID(), extStr))
	defer func() {
		_ = os.Remove(tempFile)
	}()

	_, err := ctx.DownloadMedia(media, ext.DownloadOutputPath(tempFile), nil)
	if err != nil {
		return "", fmt.Errorf("failed to download media: %w", err)
	}

	fileBytes, err := os.ReadFile(tempFile)
	if err != nil {
		return "", fmt.Errorf("failed to read downloaded file: %w", err)
	}

	base64Str := base64.StdEncoding.EncodeToString(fileBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Str)

	return dataURL, nil
}
