package admins

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "Blacklist",
		Description: "Blacklisted Word from channel/group",
		Commands:    []string{"bl", "blacklist"},
		OnlyOut:     true,
		Handler:     blacklistHandler,
		OnMessage:   blacklistMessageHook,
	})
}

func blacklistHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if uChat.IsAUser() {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "❌ <b>Error:</b> Blacklist commands can only be used in groups or supergroups.")
		return nil
	}

	args := update.Args()
	if len(args) == 0 {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "❌ <b>Usage:</b>\n"+
			"• <code>.bl add [word/phrase]</code>\n"+
			"• <code>.bl del [word/phrase]</code>\n"+
			"• <code>.bl list</code>\n"+
			"• <code>.bl clear</code>")
		return nil
	}

	subCommand := strings.ToLower(args[0])
	switch subCommand {
	case "add":
		if len(args) < 2 {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "❌ <b>Usage:</b> <code>.bl add [word/phrase]</code>")
			return nil
		}
		word := strings.TrimSpace(strings.Join(args[1:], " "))
		if word == "" {
			return nil
		}

		if _, err := regexp.Compile("(?i)" + word); err != nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Invalid regex pattern:</b> %s", err.Error()))
			return nil
		}

		ctxBg := context.Background()
		key := fmt.Sprintf("userbot:blacklist:%d", uChat.GetID())
		err := dbClient.Redis.SAdd(ctxBg, key, word).Err()
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to add pattern to blacklist:</b> %s", err.Error()))
			return err
		}

		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("✅ Added regex pattern <code>%s</code> to the blacklist.", html.EscapeString(word)))
		return nil

	case "del", "remove", "delete":
		if len(args) < 2 {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "❌ <b>Usage:</b> <code>.bl del [word/phrase]</code>")
			return nil
		}
		word := strings.TrimSpace(strings.Join(args[1:], " "))
		if word == "" {
			return nil
		}

		ctxBg := context.Background()
		key := fmt.Sprintf("userbot:blacklist:%d", uChat.GetID())
		removed, err := dbClient.Redis.SRem(ctxBg, key, word).Result()
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to remove pattern from blacklist:</b> %s", err.Error()))
			return err
		}

		if removed == 0 {
			// Try lowercase as well for backward compatibility
			removed, _ = dbClient.Redis.SRem(ctxBg, key, strings.ToLower(word)).Result()
		}

		if removed == 0 {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("ℹ️ <code>%s</code> was not in the blacklist.", html.EscapeString(word)))
			return nil
		}

		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("✅ Removed pattern <code>%s</code> from the blacklist.", html.EscapeString(word)))
		return nil

	case "list":
		ctxBg := context.Background()
		key := fmt.Sprintf("userbot:blacklist:%d", uChat.GetID())
		words, err := dbClient.Redis.SMembers(ctxBg, key).Result()
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to retrieve blacklist:</b> %s", err.Error()))
			return err
		}

		if len(words) == 0 {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "ℹ️ <b>Blacklist is empty in this chat.</b>")
			return nil
		}

		var sb strings.Builder
		sb.WriteString("📋 <b>Blacklisted words/phrases in this chat:</b>\n")
		for i, w := range words {
			sb.WriteString(fmt.Sprintf("%d. <code>%s</code>\n", i+1, html.EscapeString(w)))
		}

		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, sb.String())
		return nil

	case "clear":
		ctxBg := context.Background()
		key := fmt.Sprintf("userbot:blacklist:%d", uChat.GetID())
		err := dbClient.Redis.Del(ctxBg, key).Err()
		if err != nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to clear blacklist:</b> %s", err.Error()))
			return err
		}

		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "✅ <b>Blacklist cleared successfully for this chat!</b>")
		return nil

	default:
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "❌ <b>Unknown subcommand.</b>\nUsage:\n• <code>.bl add [word]</code>\n• <code>.bl del [word]</code>\n• <code>.bl list</code>\n• <code>.bl clear</code>")
		return nil
	}
}

func blacklistMessageHook(ctx *ext.Context, update *ext.Update) error {
	msg := update.EffectiveMessage
	if msg == nil {
		return nil
	}
	user := update.EffectiveUser()

	if msg.Out || user == nil || user.ID == ctx.Self.ID {
		return nil
	}

	uChat := update.EffectiveChat()
	userID := user.ID

	if uChat.IsAUser() {
		return nil
	}

	ctxBg := context.Background()
	key := fmt.Sprintf("userbot:blacklist:%d", uChat.GetID())
	words, err := dbClient.Redis.SMembers(ctxBg, key).Result()
	if err != nil || len(words) == 0 {
		return nil
	}

	isAdmin, errAdmin := utils.IsAdminOrSelf(ctx, uChat.GetID(), userID)
	if errAdmin != nil || isAdmin {
		return nil
	}

	hasBlacklistedWord := false
	var matchedWord string
	for _, word := range words {
		re, err := regexp.Compile("(?i)" + word)
		if err != nil {
			continue
		}
		if re.MatchString(msg.Text) {
			hasBlacklistedWord = true
			matchedWord = word
			break
		}
	}

	if hasBlacklistedWord {
		canDelete, errCanDelete := canDeleteMessages(ctx, uChat.GetID())
		if errCanDelete != nil || !canDelete {
			return nil
		}

		err = ctx.DeleteMessages(uChat.GetID(), []int{msg.ID})
		if err != nil {
			return err
		}

		warnMsg := fmt.Sprintf("🚨 <b>Blacklist Triggered!</b>\n"+
			"User <code>%d</code> sent a message containing a blacklisted word: <code>%s</code>. The message has been deleted.",
			userID, html.EscapeString(matchedWord))

		text, entities := utils.ParseHTML(warnMsg)
		sentMsg, errSent := ctx.SendMessage(uChat.GetID(), &tg.MessagesSendMessageRequest{
			Message:  text,
			Entities: entities,
		})

		if errSent == nil && sentMsg != nil {
			go func(chatID int64, msgID int) {
				time.Sleep(5 * time.Second)
				_ = ctx.DeleteMessages(chatID, []int{msgID})
			}(uChat.GetID(), sentMsg.ID)
		}
	}

	return nil
}
