// Package menu menyediakan menu navigasi utama dengan inline button.
// Menu ini dikirim menggunakan inline query sehingga bisa digunakan di mana saja
// tanpa perlu memasukkan bot ke dalam grup/channel.
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
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

const pageSize = 6 // 3 baris x 2 kolom

func init() {
	manager.Register(&manager.Module{
		Name:        "Menu",
		Description: "Tampilkan menu navigasi utama dengan inline button (inline query mode)",
		Commands:    []string{"menu"},
		OnlyOut:     true,
		Handler:     menuHandler,

		// Callback query routing
		CallbackPrefix:  "menu",
		CallbackHandler: menuCallbackHandler,

		// Inline query handler (sisi bot companion)
		InlineHandler: menuInlineHandler,
	})
}

// menuHandler dipanggil saat user mengetik .menu (sisi userbot)
// Userbot akan memanggil bot companion via inline query, lalu mengirimkan hasilnya ke grup.
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

	// Resolve bot peer
	botInputPeer, err := ctx.ResolveUsername(botUsername)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID,
			"❌ <b>Gagal resolve bot username:</b> "+err.Error())
		return nil
	}

	// Resolve chat peer
	chatInputPeer, err := ctx.ResolveInputPeerById(uChat.GetID())
	if err != nil {
		return err
	}

	// Hapus pesan perintah .menu agar chat bersih
	_ = ctx.DeleteMessages(uChat.GetID(), []int{uMsg.ID})

	// Panggil inline query ke bot companion
	results, err := ctx.Raw.MessagesGetInlineBotResults(ctx, &tg.MessagesGetInlineBotResultsRequest{
		Bot:    botInputPeer.GetInputUser(),
		Peer:   chatInputPeer,
		Query:  "menu",
		Offset: "",
	})
	if err != nil {
		return err
	}

	if len(results.Results) == 0 {
		return fmt.Errorf("bot tidak mengembalikan hasil inline query")
	}

	// Kirim hasil inline query ke chat
	_, err = ctx.Raw.MessagesSendInlineBotResult(ctx, &tg.MessagesSendInlineBotResultRequest{
		Peer:     chatInputPeer,
		RandomID: rand.Int63(),
		QueryID:  results.QueryID,
		ID:       results.Results[0].GetID(),
	})
	return err
}

// menuInlineHandler dipanggil saat bot menerima inline query dari userbot (sisi bot)
func menuInlineHandler(ctx context.Context, q *tg.UpdateBotInlineQuery) error {
	if q.Query != "menu" {
		return nil
	}

	// Generate menu list halaman pertama (page 0)
	text, buttons := getModulesPage(0)
	keyboard := bot.BuildInlineKeyboard(buttons)

	result := &tg.InputBotInlineResult{
		ID:   "menu_main",
		Type: "article",
		SendMessage: &tg.InputBotInlineMessageText{
			Message:     text,
			ReplyMarkup: keyboard,
			NoWebpage:   true,
		},
	}
	result.SetTitle("Menu Utama Userbot")
	result.SetDescription("Klik di sini untuk mengirim menu utama")

	results := []tg.InputBotInlineResultClass{result}

	return bot.AnswerInlineQuery(ctx, q.QueryID, results)
}

// menuCallbackHandler dipanggil saat user menekan salah satu tombol menu (sisi bot)
func menuCallbackHandler(ctx context.Context, q *manager.CallbackQuery) error {
	payload := strings.TrimPrefix(string(q.Data), "menu:")

	// Paginasi: menu:page:<page_num>
	if strings.HasPrefix(payload, "page:") {
		pageNumStr := strings.TrimPrefix(payload, "page:")
		pageNum, err := strconv.Atoi(pageNumStr)
		if err != nil {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, "Halaman tidak valid.", false)
		}

		text, buttons := getModulesPage(pageNum)
		if q.IsInline {
			_ = bot.EditInlineBotMessage(q.InlineMessageID, text, buttons)
		} else {
			chatID := peerToID(q.Peer)
			peer := inputPeerFromID(chatID)
			_ = bot.EditBotMessage(peer, q.MsgID, text, buttons)
		}
		return bot.AnswerCallbackQuery(ctx, q.QueryID, "", false)
	}

	// Detail Modul: menu:mod:<mod_name>:<from_page>
	if strings.HasPrefix(payload, "mod:") {
		parts := strings.Split(strings.TrimPrefix(payload, "mod:"), ":")
		if len(parts) < 2 {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, "Detail tidak valid.", false)
		}
		modName := parts[0]
		fromPageStr := parts[1]

		mod, exists := manager.Registry[modName]
		if !exists {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, "Modul tidak ditemukan.", false)
		}

		text, buttons := getModuleDetail(mod, fromPageStr)
		if q.IsInline {
			_ = bot.EditInlineBotMessage(q.InlineMessageID, text, buttons)
		} else {
			chatID := peerToID(q.Peer)
			peer := inputPeerFromID(chatID)
			_ = bot.EditBotMessage(peer, q.MsgID, text, buttons)
		}
		return bot.AnswerCallbackQuery(ctx, q.QueryID, "", false)
	}

	// Tutup / Close Menu: menu:close
	if payload == "close" {
		if q.IsInline {
			if err := bot.EditInlineBotMessage(q.InlineMessageID, "❌ Menu ditutup.", nil); err != nil {
				return bot.AnswerCallbackQuery(ctx, q.QueryID, "Gagal menutup menu.", false)
			}
		} else {
			chatID := peerToID(q.Peer)
			peer := inputPeerFromID(chatID)
			if err := bot.EditBotMessage(peer, q.MsgID, "❌ Menu ditutup.", nil); err != nil {
				return bot.AnswerCallbackQuery(ctx, q.QueryID, "Gagal menutup menu.", false)
			}
		}
		return bot.AnswerCallbackQuery(ctx, q.QueryID, "", false)
	}

	return bot.AnswerCallbackQuery(ctx, q.QueryID, "", false)
}

// getSortedModules mengambil dan mengurutkan seluruh modul terdaftar secara alfabetis
func getSortedModules() []*manager.Module {
	var modules []*manager.Module
	for _, mod := range manager.Registry {
		modules = append(modules, mod)
	}
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name < modules[j].Name
	})
	return modules
}

// getModulesPage menghasilkan teks menu dan list tombol untuk halaman modul tertentu
func getModulesPage(page int) (string, [][]bot.Button) {
	modules := getSortedModules()
	totalModules := len(modules)

	// Hitung total halaman
	totalPages := (totalModules + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	// Batasi page index agar tidak out of bound (wrap around)
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

	// Grid tombol modul (2 kolom)
	var modRows [][]bot.Button
	var currentRow []bot.Button
	for i := start; i < end; i++ {
		mod := modules[i]
		btn := bot.Button{
			Text:         mod.Name,
			CallbackData: fmt.Sprintf("menu:mod:%s:%d", mod.Name, page),
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

	// Baris tombol navigasi di bagian paling bawah
	prevPage := page - 1
	nextPage := page + 1

	navRow := []bot.Button{
		{Text: "◀️ Prev", CallbackData: fmt.Sprintf("menu:page:%d", prevPage)},
		{Text: "❌ Close", CallbackData: "menu:close"},
		{Text: "▶️ Next", CallbackData: fmt.Sprintf("menu:page:%d", nextPage)},
	}
	modRows = append(modRows, navRow)

	// Format teks menu utama
	text := fmt.Sprintf("📦 <b>Daftar Modul Userbot</b> (Hal %d/%d)\n\nSilakan pilih modul di bawah untuk melihat detail commands:", page+1, totalPages)

	return text, modRows
}

// getModuleDetail menghasilkan teks detail modul dan list tombol (Back & Close)
func getModuleDetail(mod *manager.Module, fromPage string) (string, [][]bot.Button) {
	// Ambil prefix perintah yang aktif saat ini dari Redis
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

	// Tombol Back & Close
	buttons := [][]bot.Button{
		{
			{Text: "◀️ Back", CallbackData: fmt.Sprintf("menu:page:%s", fromPage)},
			{Text: "❌ Close", CallbackData: "menu:close"},
		},
	}

	return text, buttons
}

// peerToID mengkonversi PeerClass ke int64 chat ID
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

// inputPeerFromID membuat InputPeer untuk fallback pesan normal
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
