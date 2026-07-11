package lang

import (
	"context"
	"fmt"
	"math/rand"
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

func init() {
	manager.Register(&manager.Module{
		Name:            "Lang",
		Description:     "Tampilkan pengaturan bahasa dengan inline button",
		Commands:        []string{"lang"},
		OnlyOut:         true,
		Handler:         langHandler,
		CallbackPrefix:  "lang",
		CallbackHandler: langCallbackHandler,
		InlineHandler:   langInlineHandler,
	})
}

func langHandler(ctx *ext.Context, update *ext.Update) error {
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
		Query:  fmt.Sprintf("lang:%d", uChat.GetID()),
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
		key := fmt.Sprintf("lang_msg:%d", uChat.GetID())
		_ = dbClient.Redis.Set(ctx, key, msgID, 0).Err()
	}

	return nil
}

func langInlineHandler(ctx context.Context, q *tg.UpdateBotInlineQuery) error {
	if !strings.HasPrefix(q.Query, "lang:") {
		return manager.ErrNotMatched
	}

	var chatID int64
	parts := strings.Split(q.Query, ":")
	if len(parts) == 2 {
		chatID, _ = strconv.ParseInt(parts[1], 10, 64)
	}

	text, buttons := getLangPanel(ctx, chatID)
	keyboard := bot.BuildInlineKeyboard(buttons)
	plainText, entities := utils.ParseHTML(text)

	result := &tg.InputBotInlineResult{
		ID:   "lang_main",
		Type: "article",
		SendMessage: &tg.InputBotInlineMessageText{
			Message:     plainText,
			Entities:    entities,
			ReplyMarkup: keyboard,
			NoWebpage:   true,
		},
	}
	result.SetTitle(i18n.Localize("LangTitle", nil, nil))
	result.SetDescription(i18n.Localize("LangDesc", nil, nil))

	results := []tg.InputBotInlineResultClass{result}
	return bot.AnswerInlineQuery(ctx, q.QueryID, results)
}

func langCallbackHandler(ctx context.Context, q *manager.CallbackQuery) error {
	payload := strings.TrimPrefix(string(q.Data), "lang:")

	if strings.HasPrefix(payload, "close:") {
		chatIDStr := strings.TrimPrefix(payload, "close:")
		chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)

		key := fmt.Sprintf("lang_msg:%d", chatID)
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

	if strings.HasPrefix(payload, "set:") {
		parts := strings.Split(strings.TrimPrefix(payload, "set:"), ":")
		if len(parts) < 2 {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, "Invalid language request", false)
		}
		lang := parts[0]
		chatID, _ := strconv.ParseInt(parts[1], 10, 64)

		err := i18n.SetLanguage(ctx, lang)
		if err != nil {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, fmt.Sprintf("Error setting language: %v", err), true)
		}

		successMsg := i18n.Localize("LangSuccess", map[string]interface{}{"Language": getLangLabel(lang)}, nil)
		_ = bot.AnswerCallbackQuery(ctx, q.QueryID, successMsg, false)

		text, buttons := getLangPanel(ctx, chatID)
		if q.IsInline {
			_ = bot.EditInlineBotMessage(q.InlineMessageID, text, buttons)
		} else {
			peer := inputPeerFromID(chatID)
			_ = bot.EditBotMessage(peer, q.MsgID, text, buttons)
		}
		return nil
	}

	return nil
}

func getLangPanel(ctx context.Context, chatID int64) (string, [][]bot.Button) {
	currentLang := i18n.GetLanguage()
	text := i18n.Localize("LangSelect", map[string]interface{}{"Current": currentLang}, nil)

	availLanguages := i18n.GetAllAvailLanguage()

	// build button rows
	var buttons [][]bot.Button
	var row []bot.Button
	for _, lang := range availLanguages {
		label := getLangLabel(lang)
		if lang == currentLang {
			label = "✅ " + label
		}
		btn := bot.Button{
			Text:         label,
			CallbackData: fmt.Sprintf("lang:set:%s:%d", lang, chatID),
		}
		row = append(row, btn)
		if len(row) == 2 {
			buttons = append(buttons, row)
			row = []bot.Button{}
		}
	}
	if len(row) > 0 {
		buttons = append(buttons, row)
	}

	closeBtnText := i18n.Localize("MenuCloseBtn", nil, nil)
	buttons = append(buttons, []bot.Button{
		{
			Text:         closeBtnText,
			CallbackData: fmt.Sprintf("lang:close:%d", chatID),
		},
	})

	return text, buttons
}

func getLangLabel(lang string) string {
	switch lang {
	case "en":
		return "🇬🇧 English"
	case "id":
		return "🇮🇩 Bahasa Indonesia"
	default:
		return strings.ToUpper(lang)
	}
}

func inputPeerFromID(chatID int64) tg.InputPeerClass {
	if chatID > 0 {
		return &tg.InputPeerUser{UserID: chatID}
	}
	if chatID < -1_000_000_000_000 {
		channelID := -(chatID + 1_000_000_000_000)
		return &tg.InputPeerChannel{ChannelID: channelID}
	}
	return &tg.InputPeerChat{ChatID: -chatID}
}
