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
			"⚠️ <b>Bot Companion tidak aktif.</b>\nSet <code>BOT_TOKEN</code> di .env untuk menggunakan fitur inline menu.")
		return nil
	}

	botUsername := bot.BotUsername
	if botUsername == "" {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID,
			"⚠️ <b>Bot Username belum didapatkan.</b> Harap tunggu beberapa detik saat startup.")
		return nil
	}

	botInputPeer, err := ctx.ResolveUsername(botUsername)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID,
			"❌ <b>Gagal resolve bot username:</b> "+err.Error())
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
		return nil
	}

	var chatID int64
	parts := strings.Split(q.Query, ":")
	if len(parts) == 2 {
		chatID, _ = strconv.ParseInt(parts[1], 10, 64)
	}

	text, buttons := getModulesPage(0, chatID)
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
	result.SetTitle("Menu Utama Userbot")
	result.SetDescription("Klik di sini untuk mengirim menu utama")

	results := []tg.InputBotInlineResultClass{result}

	return bot.AnswerInlineQuery(ctx, q.QueryID, results)
}

func menuCallbackHandler(ctx context.Context, q *manager.CallbackQuery) error {
	payload := strings.TrimPrefix(string(q.Data), "menu:")

	if strings.HasPrefix(payload, "page:") {
		parts := strings.Split(strings.TrimPrefix(payload, "page:"), ":")
		if len(parts) < 2 {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, "Detail page tidak valid.", false)
		}
		pageNum, _ := strconv.Atoi(parts[0])
		chatID, _ := strconv.ParseInt(parts[1], 10, 64)

		text, buttons := getModulesPage(pageNum, chatID)
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
			return bot.AnswerCallbackQuery(ctx, q.QueryID, "Detail modul tidak valid.", false)
		}
		modID := parts[0]
		fromPageStr := parts[1]
		chatID, _ := strconv.ParseInt(parts[2], 10, 64)

		logicalMods := getLogicalModules()
		var targetMod *LogicalModule
		for i := range logicalMods {
			if logicalMods[i].ID == modID {
				targetMod = &logicalMods[i]
				break
			}
		}

		if targetMod == nil {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, "Modul tidak ditemukan.", false)
		}

		text, buttons := getModuleDetail(targetMod, fromPageStr, chatID)
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

func getLogicalModules() []LogicalModule {
	prettyNames := map[string]string{
		"admins":    "👮 Admins",
		"afk":       "💤 AFK",
		"antiflood": "🛡️ Antiflood",
		"download":  "📥 Download",
		"ping":      "🏓 Ping",
		"prefix":    "⚙️ Prefix",
		"status":    "ℹ️ Status",
		"voicechat": "🎵 Voice Chat",
	}

	prettyDescriptions := map[string]string{
		"admins":    "Mengelola administrasi grup (ban, kick, promote, dll.)",
		"afk":       "Mengatur status Away From Keyboard (AFK) saat Anda tidak aktif",
		"antiflood": "Mengamankan grup dari spamming/flood pesan",
		"download":  "Mengunduh file atau media dari link internet secara langsung",
		"ping":      "Memeriksa kecepatan respon (latensi) userbot ke Telegram",
		"prefix":    "Mengubah prefix (simbol pemicu) untuk semua command userbot",
		"status":    "Melihat status sistem, versi, uptime, dan info sistem lainnya",
		"voicechat": "Mengontrol pemutaran musik/audio secara real-time di voice chat grup",
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
				name = strings.Title(pkgName)
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

func getModulesPage(page int, chatID int64) (string, [][]bot.Button) {
	logicalMods := getLogicalModules()
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
		{Text: "◀️ Prev", CallbackData: fmt.Sprintf("menu:page:%d:%d", prevPage, chatID)},
		{Text: "❌ Close", CallbackData: fmt.Sprintf("menu:close:%d", chatID)},
		{Text: "▶️ Next", CallbackData: fmt.Sprintf("menu:page:%d:%d", nextPage, chatID)},
	}
	modRows = append(modRows, navRow)

	text := fmt.Sprintf("📦 <b>Daftar Modul Userbot</b> (Hal %d/%d)\n\nSilakan pilih modul di bawah untuk melihat detail commands:", page+1, totalPages)

	return text, modRows
}

func getModuleDetail(mod *LogicalModule, fromPage string, chatID int64) (string, [][]bot.Button) {
	prefix, err := dbClient.Redis.Get(context.Background(), "prefix").Result()
	if err != nil || prefix == "" {
		prefix = "."
	}

	var cmdList []string
	if len(mod.Commands) > 0 {
		for _, cmd := range mod.Commands {
			cmdList = append(cmdList, fmt.Sprintf("- <code>%s%s</code>", prefix, cmd))
		}
	} else {
		cmdList = append(cmdList, "<i>Tidak ada direct command. Modul bekerja di latar belakang.</i>")
	}

	text := fmt.Sprintf("📦 <b>Modul: %s</b>\n\nℹ️ <i>%s</i>\n\n<b>Commands:</b>\n%s",
		mod.Name, mod.Description, strings.Join(cmdList, "\n"))

	buttons := [][]bot.Button{
		{
			{Text: "◀️ Back", CallbackData: fmt.Sprintf("menu:page:%s:%d", fromPage, chatID)},
			{Text: "❌ Close", CallbackData: fmt.Sprintf("menu:close:%d", chatID)},
		},
	}

	return text, buttons
}

func peerToID(peer tg.PeerClass) int64 {
	if peer == nil {
		return 0
	}
	switch p := peer.(type) {
	case *tg.PeerUser:
		return p.UserID
	case *tg.PeerChat:
		return -p.ChatID
	case *tg.PeerChannel:
		return -1_000_000_000_000 - p.ChannelID
	}
	return 0
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
