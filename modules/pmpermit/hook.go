package pmpermit

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/utils"
)

func pmpermitMessageHook(ctx *ext.Context, update *ext.Update) error {
	msg := update.EffectiveMessage
	if msg == nil {
		return nil
	}

	uChat := update.EffectiveChat()
	if uChat == nil || uChat.IsAUser() == false {
		return nil
	}

	sender := update.EffectiveUser()
	if sender == nil || sender.Bot || sender.ID == ctx.Self.ID {
		return nil
	}

	ctxBg := ctx

	// If it is an outgoing message (sent by us), auto-approve the user!
	if msg.Out {
		prefixVal, err := dbClient.Redis.Get(ctxBg, "prefix").Result()
		if err == nil && strings.HasPrefix(msg.Text, prefixVal) {
			return nil
		}

		dbClient.Redis.SAdd(ctxBg, "userbot:pmpermit:approved", sender.ID)
		dbClient.Redis.Del(ctxBg, fmt.Sprintf("userbot:pmpermit:warns:%d", sender.ID))
		return nil
	}

	// For incoming messages:
	// 1. Check if pmpermit is enabled
	enabled, err := dbClient.Redis.Get(ctxBg, "userbot:pmpermit:enabled").Result()
	if err == nil && enabled == "false" {
		return nil
	}

	// 2. Check if user is approved
	isApproved, err := dbClient.Redis.SIsMember(ctxBg, "userbot:pmpermit:approved", sender.ID).Result()
	if err == nil && isApproved {
		return nil
	}

	// 3. Ignore commands
	prefixVal, err := dbClient.Redis.Get(ctxBg, "prefix").Result()
	if err == nil && strings.HasPrefix(msg.Text, prefixVal) {
		return nil
	}

	// 4. Increment warns
	warnsKey := fmt.Sprintf("userbot:pmpermit:warns:%d", sender.ID)
	warns, err := dbClient.Redis.Incr(ctxBg, warnsKey).Result()
	if err != nil {
		return nil
	}
	dbClient.Redis.Expire(ctxBg, warnsKey, 24*time.Hour)

	limitValStr, err := dbClient.Redis.Get(ctxBg, "userbot:pmpermit:limit").Result()
	limit := int64(4)
	if err == nil && limitValStr != "" {
		if l, err := strconv.ParseInt(limitValStr, 10, 64); err == nil && l > 0 {
			limit = l
		}
	}

	// Try to delete previous warning message
	lastMsgKey := fmt.Sprintf("userbot:pmpermit:lastmsg:%d", sender.ID)
	lastMsgIDStr, err := dbClient.Redis.Get(ctxBg, lastMsgKey).Result()
	if err == nil && lastMsgIDStr != "" {
		if id, err := strconv.Atoi(lastMsgIDStr); err == nil {
			_ = ctx.DeleteMessages(uChat.GetID(), []int{id})
		}
	}

	if warns >= limit {
		// Block the user
		inputPeer, err := ctx.ResolveInputPeerById(sender.ID)
		if err == nil {
			_, _ = ctx.Raw.ContactsBlock(ctx, &tg.ContactsBlockRequest{ID: inputPeer})
		}

		// Send final block notification
		blockedMsg := i18n.Localize("PMPermitBlockedMsg", nil, nil)
		if blockedMsg == "" {
			blockedMsg = "❌ <b>Anda telah diblokir karena melakukan spam tanpa persetujuan.</b>"
		}
		text, entities := utils.ParseHTML(blockedMsg)
		_, _ = ctx.SendMessage(uChat.GetID(), &tg.MessagesSendMessageRequest{
			Message:  text,
			Entities: entities,
		})

		// Clean up Redis
		dbClient.Redis.Del(ctxBg, warnsKey)
		dbClient.Redis.Del(ctxBg, lastMsgKey)
		return nil
	}

	// Get custom text or default
	customText, _ := dbClient.Redis.Get(ctxBg, "userbot:pmpermit:text").Result()
	if customText == "" {
		customText = i18n.Localize("PMPermitWarningMsg", map[string]any{
			"Warn":  warns,
			"Limit": limit,
		}, nil)
		if customText == "" {
			customText = fmt.Sprintf("⚠️ <b>PM Security Aktif!</b>\nHarap tunggu persetujuan dari pemilik akun sebelum mengirim pesan.\n\nPeringatan: <code>%d/%d</code>", warns, limit)
		}
	} else {
		customText = strings.ReplaceAll(customText, "{warn}", strconv.FormatInt(warns, 10))
		customText = strings.ReplaceAll(customText, "{limit}", strconv.FormatInt(limit, 10))
	}

	text, entities := utils.ParseHTML(customText)
	sentMsg, err := ctx.SendMessage(uChat.GetID(), &tg.MessagesSendMessageRequest{
		Message:  text,
		Entities: entities,
	})
	if err == nil && sentMsg != nil {
		dbClient.Redis.Set(ctxBg, lastMsgKey, sentMsg.ID, 24*time.Hour)
	}

	return nil
}
