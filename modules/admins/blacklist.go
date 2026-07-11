package admins

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/i18n"
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
		localize := i18n.Localize("BLErrorNotGroup", nil, nil)
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
		return nil
	}

	args := update.Args()
	if len(args) == 0 {
		usageTxt := i18n.Localize("BLUsage", nil, nil)
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, usageTxt)
		return nil
	}

	subCommand := strings.ToLower(args[0])
	switch subCommand {
	case "add":
		if len(args) < 2 {
			localize := i18n.Localize("BLUsageAdd", nil, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return nil
		}
		word := strings.TrimSpace(strings.Join(args[1:], " "))
		if word == "" {
			return nil
		}

		if _, err := regexp.Compile("(?i)" + word); err != nil {
			localize := i18n.Localize("BLInvalidRegex", map[string]interface{}{
				"Error": err.Error(),
			}, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return nil
		}

		ctxBg := ctx
		key := fmt.Sprintf("userbot:blacklist:%d", uChat.GetID())
		err := dbClient.Redis.SAdd(ctxBg, key, word).Err()
		if err != nil {
			localize := i18n.Localize("BLFailedPattern", map[string]interface{}{
				"Error": err.Error(),
			}, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return err
		}

		localize := i18n.Localize("BLSuccess", map[string]interface{}{
			"Regex": html.EscapeString(word),
		}, nil)
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
		return nil

	case "del", "remove", "delete":
		if len(args) < 2 {
			localize := i18n.Localize("BLUsageDel", nil, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return nil
		}
		word := strings.TrimSpace(strings.Join(args[1:], " "))
		if word == "" {
			return nil
		}

		ctxBg := ctx
		key := fmt.Sprintf("userbot:blacklist:%d", uChat.GetID())
		removed, err := dbClient.Redis.SRem(ctxBg, key, word).Result()
		if err != nil {
			localize := i18n.Localize("BLFailedRemove", map[string]interface{}{"Error": err.Error()}, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return err
		}

		if removed == 0 {
			// Try lowercase as well for backward compatibility
			removed, _ = dbClient.Redis.SRem(ctxBg, key, strings.ToLower(word)).Result()
		}

		if removed == 0 {
			localize := i18n.Localize("BLNotFound", map[string]interface{}{"Word": html.EscapeString(word)}, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return nil
		}

		localize := i18n.Localize("BLSuccessRemove", map[string]interface{}{"Word": html.EscapeString(word)}, nil)
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
		return nil

	case "list":
		ctxBg := ctx
		key := fmt.Sprintf("userbot:blacklist:%d", uChat.GetID())
		words, err := dbClient.Redis.SMembers(ctxBg, key).Result()
		if err != nil {
			localize := i18n.Localize("BLFailedRetrieve", map[string]interface{}{"Error": err.Error()}, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return err
		}

		if len(words) == 0 {
			localize := i18n.Localize("BLEmpty", nil, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return nil
		}

		var sb strings.Builder
		sb.WriteString(i18n.Localize("BLListHeader", nil, nil))
		for i, w := range words {
			sb.WriteString(fmt.Sprintf("%d. <code>%s</code>\n", i+1, html.EscapeString(w)))
		}

		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, sb.String())
		return nil

	case "clear":
		ctxBg := ctx
		key := fmt.Sprintf("userbot:blacklist:%d", uChat.GetID())
		err := dbClient.Redis.Del(ctxBg, key).Err()
		if err != nil {
			localize := i18n.Localize("BLFailedClear", map[string]interface{}{"Error": err.Error()}, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
			return err
		}

		localize := i18n.Localize("BLSuccessClear", nil, nil)
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
		return nil

	default:
		localize := i18n.Localize("BLUnknownSubcommand", nil, nil)
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, localize)
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

	ctxBg := ctx
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

		warnMsg := i18n.Localize("BLTriggered", map[string]interface{}{
			"UserId": userID,
			"Word":   html.EscapeString(matchedWord),
		}, nil)

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
