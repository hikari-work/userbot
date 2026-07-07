// Package menu menyediakan menu navigasi utama dengan inline button.
// Menu ini dikirim menggunakan inline query sehingga bisa digunakan di mana saja
// tanpa perlu memasukkan bot ke dalam grup/channel.
package menu

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
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

const pageSize = 6 // 3 baris x 2 kolom

// LogicalModule mendefinisikan modul kategori secara logis
type LogicalModule struct {
	ID          string
	Name        string
	Description string
	Commands    []string
}

// Daftar modul kategori logis yang akan ditampilkan di tombol menu
var logicalModules = []LogicalModule{
	{
		ID:          "admins",
		Name:        "👮 Admins",
		Description: "Mengelola administrasi grup (ban, kick, promote, dll.)",
		Commands:    []string{"promote", "demote", "ban", "unban", "kick", "purge", "purgeme", "cleanservice", "addblacklist", "remblacklist", "blacklist"},
	},
	{
		ID:          "afk",
		Name:        "💤 AFK",
		Description: "Mengatur status Away From Keyboard (AFK) saat Anda tidak aktif",
		Commands:    []string{"afk"},
	},
	{
		ID:          "antiflood",
		Name:        "🛡️ Antiflood",
		Description: "Mengamankan grup dari spamming/flood pesan",
		Commands:    []string{"antiflood", "setflood"},
	},
	{
		ID:          "download",
		Name:        "📥 Download",
		Description: "Mengunduh file atau media dari link internet secara langsung",
		Commands:    []string{"download"},
	},
	{
		ID:          "ping",
		Name:        "🏓 Ping",
		Description: "Memeriksa kecepatan respon (latensi) userbot ke Telegram",
		Commands:    []string{"ping"},
	},
	{
		ID:          "prefix",
		Name:        "⚙️ Prefix",
		Description: "Mengubah prefix (simbol pemicu) untuk semua command userbot",
		Commands:    []string{"prefix"},
	},
	{
		ID:          "status",
		Name:        "ℹ️ Status",
		Description: "Melihat status sistem, versi, uptime, dan info sistem lainnya",
		Commands:    []string{"status"},
	},
	{
		ID:          "voice_chat",
		Name:        "🎵 Voice Chat",
		Description: "Mengontrol pemutaran musik/audio secara real-time di voice chat grup",
		Commands:    []string{"joinvc", "leavevc", "play", "pause", "resume", "stop"},
	},
}

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

	// Panggil inline query ke bot companion dengan menyertakan ChatID di query
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

	// Kirim hasil inline query ke chat
	sentUpdates, err := ctx.Raw.MessagesSendInlineBotResult(ctx, &tg.MessagesSendInlineBotResultRequest{
		Peer:     chatInputPeer,
		RandomID: rand.Int63(),
		QueryID:  results.QueryID,
		ID:       results.Results[0].GetID(),
	})
	if err != nil {
		return err
	}

	// Dapatkan msgID dari updates response
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
		// Simpan msgID ke Redis agar userbot bisa menghapusnya nanti
		key := fmt.Sprintf("menu_msg:%d", uChat.GetID())
		_ = dbClient.Redis.Set(ctx, key, msgID, 0).Err()
	}

	return nil
}

// menuInlineHandler dipanggil saat bot menerima inline query dari userbot (sisi bot)
func menuInlineHandler(ctx context.Context, q *tg.UpdateBotInlineQuery) error {
	if !strings.HasPrefix(q.Query, "menu") {
		return nil
	}

	// Ambil chatID dari query (format: "menu:<chatID>")
	var chatID int64
	parts := strings.Split(q.Query, ":")
	if len(parts) == 2 {
		chatID, _ = strconv.ParseInt(parts[1], 10, 64)
	}

	// Generate menu list halaman pertama (page 0)
	text, buttons := getModulesPage(0, chatID)
	keyboard := bot.BuildInlineKeyboard(buttons)

	// Parse HTML agar bold tag <b> dan ℹ️ dirender dengan benar
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

// menuCallbackHandler dipanggil saat user menekan salah satu tombol menu (sisi bot)
func menuCallbackHandler(ctx context.Context, q *manager.CallbackQuery) error {
	payload := strings.TrimPrefix(string(q.Data), "menu:")

	// Paginasi: menu:page:<page_num>:<chat_id>
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

	// Detail Modul: menu:mod:<mod_id>:<from_page>:<chat_id>
	if strings.HasPrefix(payload, "mod:") {
		parts := strings.Split(strings.TrimPrefix(payload, "mod:"), ":")
		if len(parts) < 3 {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, "Detail modul tidak valid.", false)
		}
		modID := parts[0]
		fromPageStr := parts[1]
		chatID, _ := strconv.ParseInt(parts[2], 10, 64)

		// Cari modul kategori logis
		var targetMod *LogicalModule
		for i := range logicalModules {
			if logicalModules[i].ID == modID {
				targetMod = &logicalModules[i]
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

	// Tutup / Close Menu: menu:close:<chat_id>
	if strings.HasPrefix(payload, "close:") {
		chatIDStr := strings.TrimPrefix(payload, "close:")
		chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)

		// Hapus pesan menggunakan client userbot (karena userbot yang mengirim pesannya)
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

		// Jika gagal menghapus (misal ID pesan hilang dari Redis), fallback ke edit jadi titik
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

// getModulesPage menghasilkan teks menu dan list tombol untuk halaman modul tertentu
func getModulesPage(page int, chatID int64) (string, [][]bot.Button) {
	totalModules := len(logicalModules)

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
		mod := logicalModules[i]
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

	// Baris tombol navigasi di bagian paling bawah
	prevPage := page - 1
	nextPage := page + 1

	navRow := []bot.Button{
		{Text: "◀️ Prev", CallbackData: fmt.Sprintf("menu:page:%d:%d", prevPage, chatID)},
		{Text: "❌ Close", CallbackData: fmt.Sprintf("menu:close:%d", chatID)},
		{Text: "▶️ Next", CallbackData: fmt.Sprintf("menu:page:%d:%d", nextPage, chatID)},
	}
	modRows = append(modRows, navRow)

	// Format teks menu utama
	text := fmt.Sprintf("📦 <b>Daftar Modul Userbot</b> (Hal %d/%d)\n\nSilakan pilih modul di bawah untuk melihat detail commands:", page+1, totalPages)

	return text, modRows
}

// getModuleDetail menghasilkan teks detail modul dan list tombol (Back & Close)
func getModuleDetail(mod *LogicalModule, fromPage string, chatID int64) (string, [][]bot.Button) {
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
			{Text: "◀️ Back", CallbackData: fmt.Sprintf("menu:page:%s:%d", fromPage, chatID)},
			{Text: "❌ Close", CallbackData: fmt.Sprintf("menu:close:%d", chatID)},
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
