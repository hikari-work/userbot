// Package menu menyediakan menu navigasi utama dengan inline button.
// Menu ini dikirim menggunakan inline query sehingga bisa digunakan di mana saja
// tanpa perlu memasukkan bot ke dalam grup/channel.
package menu

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
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

type LogicalModule struct {
	ID          string
	Name        string
	Description string
	Commands    []string
}

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
}

func menuHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if !bot.IsActive() {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID,
			i18n.Localize(ctx, "MenuBotNotActive", nil, nil))
		return nil
	}

	botUsername := bot.Username
	if botUsername == "" {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID,
			i18n.Localize(ctx, "MenuBotUsernameMissing", nil, nil))
		return nil
	}

	botInputPeer, err := ctx.ResolveUsername(botUsername)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID,
			i18n.Localize(ctx, "MenuFailedResolveUsername", map[string]interface{}{"Error": err.Error()}, nil))
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
	result.SetTitle(i18n.Localize(ctx, "MenuInlineTitle", nil, nil))
	result.SetDescription(i18n.Localize(ctx, "MenuInlineDesc", nil, nil))

	results := []tg.InputBotInlineResultClass{result}

	return bot.AnswerInlineQuery(ctx, q.QueryID, results)
}

func menuCallbackHandler(ctx context.Context, q *manager.CallbackQuery) error {
	payload := strings.TrimPrefix(string(q.Data), "menu:")

	if strings.HasPrefix(payload, "page:") {
		parts := strings.Split(strings.TrimPrefix(payload, "page:"), ":")
		if len(parts) < 2 {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, i18n.Localize(ctx, "MenuInvalidPage", nil, nil), false)
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
			return bot.AnswerCallbackQuery(ctx, q.QueryID, i18n.Localize(ctx, "MenuInvalidModule", nil, nil), false)
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
			return bot.AnswerCallbackQuery(ctx, q.QueryID, i18n.Localize(ctx, "MenuModuleNotFound", nil, nil), false)
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

func getPackageName(handler interface{}) string {
	if handler == nil {
		return ""
	}
	funcValue := reflect.ValueOf(handler)
	if funcValue.Kind() != reflect.Func {
		return ""
	}
	funcName := runtime.FuncForPC(funcValue.Pointer()).Name()
	const modulesMarker = "modules/"
	idx := strings.LastIndex(funcName, modulesMarker)
	if idx != -1 {
		subPath := funcName[idx+len(modulesMarker):]
		dotIdx := strings.Index(subPath, ".")
		slashIdx := strings.Index(subPath, "/")

		endIdx := dotIdx
		if slashIdx != -1 && (endIdx == -1 || slashIdx < endIdx) {
			endIdx = slashIdx
		}
		if endIdx != -1 {
			return subPath[:endIdx]
		}
		return subPath
	}

	parts := strings.Split(funcName, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		dotIdx := strings.Index(lastPart, ".")
		if dotIdx >= 0 {
			return lastPart[:dotIdx]
		}
	}
	return ""
}

func getLogicalModules(ctx context.Context) []LogicalModule {
	prettyNames := map[string]string{
		"admins":    i18n.Localize(ctx, "module_name_admins", nil, nil),
		"afk":       i18n.Localize(ctx, "module_name_afk", nil, nil),
		"antiflood": i18n.Localize(ctx, "module_name_antiflood", nil, nil),
		"download":  i18n.Localize(ctx, "module_name_download", nil, nil),
		"ping":      i18n.Localize(ctx, "module_name_ping", nil, nil),
		"prefix":    i18n.Localize(ctx, "module_name_prefix", nil, nil),
		"status":    i18n.Localize(ctx, "module_name_status", nil, nil),
		"voicechat": i18n.Localize(ctx, "module_name_voicechat", nil, nil),
	}

	prettyDescriptions := map[string]string{
		"admins":    i18n.Localize(ctx, "module_desc_admins", nil, nil),
		"afk":       i18n.Localize(ctx, "module_desc_afk", nil, nil),
		"antiflood": i18n.Localize(ctx, "module_desc_antiflood", nil, nil),
		"download":  i18n.Localize(ctx, "module_desc_download", nil, nil),
		"ping":      i18n.Localize(ctx, "module_desc_ping", nil, nil),
		"prefix":    i18n.Localize(ctx, "module_desc_prefix", nil, nil),
		"status":    i18n.Localize(ctx, "module_desc_status", nil, nil),
		"voicechat": i18n.Localize(ctx, "module_desc_voicechat", nil, nil),
	}

	groups := make(map[string]*LogicalModule)

	for _, mod := range manager.Registry {
		if strings.ToLower(mod.Name) == "menu" {
			continue
		}

		pkgName := getPackageName(mod.Handler)
		if pkgName == "" {
			pkgName = getPackageName(mod.OnMessage)
		}
		if pkgName == "" {
			pkgName = strings.ToLower(mod.Name)
		}

		lm, exists := groups[pkgName]
		if !exists {
			name, ok := prettyNames[pkgName]
			if !ok {
				name = strings.ToTitle(pkgName)
			}
			desc := prettyDescriptions[pkgName]
			if desc == "" {
				desc = mod.Description
			}

			lm = &LogicalModule{
				ID:          pkgName,
				Name:        name,
				Description: desc,
				Commands:    []string{},
			}
			groups[pkgName] = lm
		}

		for _, cmd := range mod.Commands {
			found := false
			for _, c := range lm.Commands {
				if c == cmd {
					found = true
					break
				}
			}
			if !found {
				lm.Commands = append(lm.Commands, cmd)
			}
		}
	}

	var list []LogicalModule
	for _, lm := range groups {
		list = append(list, *lm)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return list
}

func getModulesPage(ctx context.Context, page int, chatID int64) (string, [][]bot.Button) {
	logicalMods := getLogicalModules(ctx)
	totalModules := len(logicalMods)

	totalPages := (totalModules + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	if page < 0 {
		page = totalPages - 1
	} else if page >= totalPages {
		page = 0
	}

	start := page * pageSize
	end := start + pageSize
	if end > totalModules {
		end = totalModules
	}

	var modRows [][]bot.Button
	var currentRow []bot.Button
	for i := start; i < end; i++ {
		mod := logicalMods[i]
		btn := bot.Button{
			Text:         mod.Name,
			CallbackData: fmt.Sprintf("menu:mod:%s:%d:%d", mod.ID, page, chatID),
		}
		currentRow = append(currentRow, btn)

		if len(currentRow) == 2 {
			modRows = append(modRows, currentRow)
			currentRow = nil
		}
	}
	if len(currentRow) > 0 {
		modRows = append(modRows, currentRow)
	}

	prevPage := page - 1
	nextPage := page + 1

	navRow := []bot.Button{
		{Text: i18n.Localize(ctx, "MenuPrevBtn", nil, nil), CallbackData: fmt.Sprintf("menu:page:%d:%d", prevPage, chatID)},
		{Text: i18n.Localize(ctx, "MenuCloseBtn", nil, nil), CallbackData: fmt.Sprintf("menu:close:%d", chatID)},
		{Text: i18n.Localize(ctx, "MenuNextBtn", nil, nil), CallbackData: fmt.Sprintf("menu:page:%d:%d", nextPage, chatID)},
	}
	modRows = append(modRows, navRow)

	text := i18n.Localize(ctx, "MenuListText", map[string]interface{}{
		"Page":  page + 1,
		"Total": totalPages,
	}, nil)

	return text, modRows
}

func getCommandDescription(cmd string) string {
	for _, mod := range manager.Registry {
		for _, c := range mod.Commands {
			if strings.EqualFold(c, cmd) {
				return mod.Description
			}
		}
	}
	return ""
}

func getModuleDetail(ctx context.Context, mod *LogicalModule, fromPage string, chatID int64) (string, [][]bot.Button) {
	prefix, err := dbClient.Redis.Get(ctx, "prefix").Result()
	if err != nil || prefix == "" {
		prefix = "."
	}

	var cmdList []string
	if len(mod.Commands) > 0 {
		for _, cmd := range mod.Commands {
			desc := getCommandDescription(cmd)
			if desc != "" {
				cmdList = append(cmdList, fmt.Sprintf("- <code>%s%s</code> - %s", prefix, cmd, desc))
			} else {
				cmdList = append(cmdList, fmt.Sprintf("- <code>%s%s</code>", prefix, cmd))
			}
		}
	} else {
		cmdList = append(cmdList, i18n.Localize(ctx, "MenuNoDirectCommands", nil, nil))
	}

	text := i18n.Localize(ctx, "MenuModuleDetail", map[string]interface{}{
		"Name":     mod.Name,
		"Desc":     mod.Description,
		"Commands": strings.Join(cmdList, "\n"),
	}, nil)

	buttons := [][]bot.Button{
		{
			{Text: i18n.Localize(ctx, "MenuBackBtn", nil, nil), CallbackData: fmt.Sprintf("menu:page:%s:%d", fromPage, chatID)},
			{Text: i18n.Localize(ctx, "MenuCloseBtn", nil, nil), CallbackData: fmt.Sprintf("menu:close:%d", chatID)},
		},
	}

	return text, buttons
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
