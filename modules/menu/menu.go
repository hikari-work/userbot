// Package menu menyediakan menu navigasi utama dengan inline button.
// Contoh penggunaan Bot Companion — kirim tombol via bot, handle callback.
package menu

import (
	"context"
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
		Description: "Tampilkan menu navigasi utama dengan inline button",
		Commands:    []string{"menu"},
		OnlyOut:     true,
		Handler:     menuHandler,

		// Semua callback "menu:*" akan dirouting ke sini
		CallbackPrefix:  "menu",
		CallbackHandler: menuCallbackHandler,
	})
}

// menuHandler dipanggil saat user ketik .menu
func menuHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if !bot.IsActive() {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID,
			"⚠️ <b>Bot Companion tidak aktif.</b>\nSet <code>BOT_TOKEN</code> di .env untuk menggunakan fitur tombol.")
		return nil
	}

	// Resolve peer dari userbot context — sudah include access hash yang benar
	peer, err := ctx.ResolveInputPeerById(uChat.GetID())
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID,
			"❌ <b>Gagal resolve peer:</b> "+err.Error())
		return nil
	}

	err = bot.SendWithButtons(peer, "📋 <b>Menu Utama</b>\n\nPilih salah satu menu di bawah:", [][]bot.Button{
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
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID,
			"❌ <b>Gagal mengirim menu:</b> "+err.Error())
	}
	return nil
}

// menuCallbackHandler dipanggil saat user menekan tombol menu
func menuCallbackHandler(ctx context.Context, q *tg.UpdateBotCallbackQuery) error {
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
		// Resolve peer dari entity store bot (terisi saat bot menerima update)
		peer := bot.PeerFromCallbackQuery(q.Peer)
		if err := bot.EditBotMessage(peer, q.MsgID, "❌ Menu ditutup.", nil); err != nil {
			return bot.AnswerCallbackQuery(ctx, q.QueryID, "Gagal menutup menu.", false)
		}
		return bot.AnswerCallbackQuery(ctx, q.QueryID, "", false)
	}

	return bot.AnswerCallbackQuery(ctx, q.QueryID, "", false)
}
