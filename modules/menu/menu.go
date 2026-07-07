// Package menu menyediakan menu navigasi utama dengan inline button.
// Menu ini dikirim menggunakan inline query sehingga bisa digunakan di mana saja
// tanpa perlu memasukkan bot ke dalam grup/channel.
package menu

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"

	"github.com/hikari-work/userbot/bot"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

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
		Bot:    botInputPeer.GetInputUser(), // Menggunakan GetInputUser() untuk mengembalikan tg.InputUserClass
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

	// Susun menu inline
	keyboard := bot.BuildInlineKeyboard([][]bot.Button{
		{
			{Text: "🏓 Ping",       CallbackData: "menu:ping"},
			{Text: "ℹ️ Status",    CallbackData: "menu:status"},
		},
		{
			{Text: "🎵 Voice Chat", CallbackData: "menu:voice"},
			{Text: "🔧 Admin",      CallbackData: "menu:admin"},
		},
		{
			{Text: "❌ Tutup",      CallbackData: "menu:close"},
		},
	})

	// Menggunakan InputBotInlineResult yang valid di MTProto
	result := &tg.InputBotInlineResult{
		ID:   "menu_main",
		Type: "article",
		SendMessage: &tg.InputBotInlineMessageText{
			Message:     "📋 <b>Menu Utama</b>\n\nPilih salah satu menu di bawah:",
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

	switch payload {
	case "ping":
		return bot.AnswerCallbackQuery(ctx, q.QueryID, "🏓 Pong! Userbot aktif.", false)

	case "status":
		return bot.AnswerCallbackQuery(ctx, q.QueryID, "✅ Semua sistem normal.", false)

	case "voice":
		return bot.AnswerCallbackQuery(ctx, q.QueryID, "🎵 Gunakan .joinvc untuk masuk Voice Chat.", false)

	case "admin":
		return bot.AnswerCallbackQuery(ctx, q.QueryID, "🔧 Gunakan .ban / .kick / .promote di grup.", false)

	case "close":
		if q.IsInline {
			// Edit inline message (sumber inline query)
			if err := bot.EditInlineBotMessage(q.InlineMessageID, "❌ Menu ditutup.", nil); err != nil {
				return bot.AnswerCallbackQuery(ctx, q.QueryID, "Gagal menutup menu.", false)
			}
		} else {
			// Fallback untuk pesan biasa jika ada
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
