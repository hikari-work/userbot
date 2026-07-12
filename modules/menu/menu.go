package menu

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"

	"github.com/hikari-work/userbot/bot"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

const pageSize = 6

func init() {
	manager.Register(&manager.Module{
		Name:        "Menu",
		Description: "Tampilkan menu navigasi utama dengan inline button (inline query mode)",
		Commands:    []string{"menu"},
		OnlyOut:     true,
		Handler:     menuHandler,

		CallbackPrefix:  "menu",
		CallbackHandler: menuCallbackHandler,

		InlineHandler: menuInlineHandler,
	})
	manager.Register(&manager.Module{
		Name:        "List Command",
		Description: "Tampilkan seluruh menu",
		Commands:    []string{"listmenu"},
		OnlyOut:     true,
		Handler:     listMenu,
	})
}

func listMenu(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage
	if uMsg == nil || uChat == nil {
		return nil
	}

	prefix, err := dbClient.Redis.Get(ctx, "prefix").Result()
	if err != nil || prefix == "" {
		prefix = "."
	}

	var allCmds []string
	for _, mod := range manager.Registry {
		for _, cmd := range mod.Commands {
			found := false
			for _, c := range allCmds {
				if strings.EqualFold(c, cmd) {
					found = true
					break
				}
			}
			if !found {
				allCmds = append(allCmds, cmd)
			}
		}
	}

	sort.Strings(allCmds)

	sb := strings.Builder{}
	sb.WriteString("📖 <b>MENU BANTUAN USERBOT</b>\n\n")
	sb.WriteString("<b>Daftar Perintah:</b>\n<blockquote>")
	sb.WriteString(strings.Join(allCmds, ", "))
	sb.WriteString("</blockquote>\n\n")
	sb.WriteString(fmt.Sprintf("Ketik <code>%shelp &lt;nama_modul&gt;</code> untuk melihat detail modul.", prefix))

	_, err = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, sb.String())
	return err
}

func menuHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if !bot.IsActive() {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID,
			i18n.Localize("MenuBotNotActive", nil, nil))
		return nil
	}

	botUsername := bot.Username
	if botUsername == "" {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID,
			i18n.Localize("MenuBotUsernameMissing", nil, nil))
		return nil
	}

	botInputPeer, err := ctx.ResolveUsername(botUsername)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID,
			i18n.Localize("MenuFailedResolveUsername", map[string]interface{}{"Error": err.Error()}, nil))
		return nil
	}

	chatInputPeer, err := ctx.ResolveInputPeerById(uChat.GetID())
	if err != nil {
		return err
	}

	_ = ctx.DeleteMessages(uChat.GetID(), []int{uMsg.ID})

	results, err := ctx.Raw.MessagesGetInlineBotResults(ctx, &tg.MessagesGetInlineBotResultsRequest{
		Bot:    botInputPeer.GetInputUser(),
		Peer:   chatInputPeer,
		Query:  fmt.Sprintf("menu:%d", uChat.GetID()),
		Offset: "",
	})
	if err != nil {
		return err
	}

	if len(results.Results) == 0 {
		return fmt.Errorf("bot tidak mengembalikan hasil inline query")
	}

	sentUpdates, err := ctx.Raw.MessagesSendInlineBotResult(ctx, &tg.MessagesSendInlineBotResultRequest{
		Peer:     chatInputPeer,
		RandomID: rand.Int63(),
		QueryID:  results.QueryID,
		ID:       results.Results[0].GetID(),
	})
	if err != nil {
		return err
	}

	var msgID int
	switch u := sentUpdates.(type) {
	case *tg.Updates:
		for _, upd := range u.Updates {
			if nm, ok := upd.(*tg.UpdateNewMessage); ok {
				msgID = nm.Message.GetID()
				break
			}
			if ncm, ok := upd.(*tg.UpdateNewChannelMessage); ok {
				msgID = ncm.Message.GetID()
				break
			}
		}
	case *tg.UpdateShortSentMessage:
		msgID = u.ID
	}

	if msgID != 0 {
		key := fmt.Sprintf("menu_msg:%d", uChat.GetID())
		_ = dbClient.Redis.Set(ctx, key, msgID, 0).Err()
	}

	return nil
}

func menuInlineHandler(ctx context.Context, q *tg.UpdateBotInlineQuery) error {
	if !strings.HasPrefix(q.Query, "menu") {
		return manager.ErrNotMatched
	}

	var chatID int64
	parts := strings.Split(q.Query, ":")
	if len(parts) == 2 {
		chatID, _ = strconv.ParseInt(parts[1], 10, 64)
	}

	text, buttons := getModulesPage(ctx, 0, chatID)
	keyboard := bot.BuildInlineKeyboard(buttons)

	plainText, entities := utils.ParseHTML(text)

	result := &tg.InputBotInlineResult{
		ID:   "menu_main",
		Type: "article",
		SendMessage: &tg.InputBotInlineMessageText{
			Message:     plainText,
			Entities:    entities,
			ReplyMarkup: keyboard,
			NoWebpage:   true,
		},
	}
	result.SetTitle(i18n.Localize("MenuInlineTitle", nil, nil))
	result.SetDescription(i18n.Localize("MenuInlineDesc", nil, nil))

	results := []tg.InputBotInlineResultClass{result}

	return bot.AnswerInlineQuery(ctx, q.QueryID, results)
}

func menuCallbackHandler(ctx context.Context, q *manager.CallbackQuery) error {
	payload := strings.TrimPrefix(string(q.Data), "menu:")

	if strings.HasPrefix(payload, "page:") {
		parts := strings.Split(strings.TrimPrefix(payload, "page:"), ":")
		if len(parts) < 2 {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, i18n.Localize("MenuInvalidPage", nil, nil), false)
		}
		pageNum, _ := strconv.Atoi(parts[0])
		chatID, _ := strconv.ParseInt(parts[1], 10, 64)

		text, buttons := getModulesPage(ctx, pageNum, chatID)
		if q.IsInline {
			_ = bot.EditInlineBotMessage(q.InlineMessageID, text, buttons)
		} else {
			peer := inputPeerFromID(chatID)
			_ = bot.EditBotMessage(peer, q.MsgID, text, buttons)
		}
		return bot.AnswerCallbackQuery(ctx, q.QueryID, "", false)
	}

	if strings.HasPrefix(payload, "mod:") {
		parts := strings.Split(strings.TrimPrefix(payload, "mod:"), ":")
		if len(parts) < 3 {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, i18n.Localize("MenuInvalidModule", nil, nil), false)
		}
		modID := parts[0]
		fromPageStr := parts[1]
		chatID, _ := strconv.ParseInt(parts[2], 10, 64)

		logicalMods := getLogicalModules(ctx)
		var targetMod *LogicalModule
		for i := range logicalMods {
			if logicalMods[i].ID == modID {
				targetMod = &logicalMods[i]
				break
			}
		}

		if targetMod == nil {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, i18n.Localize("MenuModuleNotFound", nil, nil), false)
		}

		text, buttons := getModuleDetail(ctx, targetMod, fromPageStr, chatID)
		if q.IsInline {
			_ = bot.EditInlineBotMessage(q.InlineMessageID, text, buttons)
		} else {
			peer := inputPeerFromID(chatID)
			_ = bot.EditBotMessage(peer, q.MsgID, text, buttons)
		}
		return bot.AnswerCallbackQuery(ctx, q.QueryID, "", false)
	}

	if strings.HasPrefix(payload, "close:") {
		chatIDStr := strings.TrimPrefix(payload, "close:")
		chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)

		key := fmt.Sprintf("menu_msg:%d", chatID)
		msgIDStr, err := dbClient.Redis.Get(ctx, key).Result()
		deleted := false
		if err == nil && msgIDStr != "" {
			msgID, _ := strconv.Atoi(msgIDStr)
			err = bot.DeleteMessageWithUserbot(chatID, msgID)
			if err == nil {
				deleted = true
				_ = dbClient.Redis.Del(ctx, key).Err()
			}
		}

		if !deleted {
			if q.IsInline {
				_ = bot.EditInlineBotMessage(q.InlineMessageID, ".", nil)
			} else {
				peer := inputPeerFromID(chatID)
				_ = bot.EditBotMessage(peer, q.MsgID, ".", nil)
			}
		}

		return bot.AnswerCallbackQuery(ctx, q.QueryID, "", false)
	}

	return bot.AnswerCallbackQuery(ctx, q.QueryID, "", false)
}
