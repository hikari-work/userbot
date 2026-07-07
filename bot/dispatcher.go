package bot

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gotd/td/tg"

	"github.com/hikari-work/userbot/modules/manager"
)

// dispatch merouting update dari Telegram ke handler modul yang sesuai
func dispatch(ctx context.Context, api *tg.Client, upd tg.UpdateClass) {
	switch u := upd.(type) {

	case *tg.UpdateBotInlineQuery:
		handleInlineQuery(ctx, u)

	case *tg.UpdateBotCallbackQuery:
		handleCallbackQuery(ctx, api, u)

	case *tg.UpdateBotInlineSend:
		// Opsional: tracking hasil inline yang dipilih user
		slog.Debug("Bot: inline result dipilih", "result_id", u.ID)
	}
}

// handleInlineQuery merouting inline query ke modul yang punya InlineHandler.
// Semua modul dengan InlineHandler akan dipanggil (bisa difilter per query prefix).
func handleInlineQuery(ctx context.Context, q *tg.UpdateBotInlineQuery) {
	slog.Debug("Bot: inline query diterima", "query", q.Query, "user_id", q.UserID)

	handled := false
	for _, mod := range manager.Registry {
		if mod.InlineHandler == nil {
			continue
		}
		if err := mod.InlineHandler(ctx, q); err != nil {
			slog.Error("Bot: error pada InlineHandler", "module", mod.Name, "error", err)
			continue
		}
		handled = true
		break // modul pertama yang handle dianggap cukup
	}

	if !handled {
		slog.Debug("Bot: tidak ada modul yang handle inline query", "query", q.Query)
	}
}

// handleCallbackQuery merouting callback query ke modul berdasarkan prefix data.
// Format callback data: "<prefix>:<payload>", contoh: "menu:ping", "admin:ban:12345"
func handleCallbackQuery(ctx context.Context, api *tg.Client, q *tg.UpdateBotCallbackQuery) {
	data := string(q.Data)
	slog.Debug("Bot: callback query diterima", "data", data)

	// Cari prefix dari data: ambil bagian sebelum ":"
	prefix := data
	if idx := strings.Index(data, ":"); idx >= 0 {
		prefix = data[:idx]
	}

	for _, mod := range manager.Registry {
		if mod.CallbackHandler == nil {
			continue
		}
		if mod.CallbackPrefix != prefix {
			continue
		}
		if err := mod.CallbackHandler(ctx, q); err != nil {
			slog.Error("Bot: error pada CallbackHandler",
				"module", mod.Name,
				"data", data,
				"error", err,
			)
		}
		return
	}

	// Tidak ada modul yang menangani — jawab kosong agar loading di client berhenti
	slog.Warn("Bot: tidak ada modul yang handle callback", "data", data)
	if b := getInstance(); b != nil && b.api != nil {
		_, _ = b.api.MessagesSetBotCallbackAnswer(ctx, &tg.MessagesSetBotCallbackAnswerRequest{
			QueryID: q.QueryID,
		})
	}
}
