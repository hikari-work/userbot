package bot

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gotd/td/tg"

	"github.com/hikari-work/userbot/modules/manager"
)

func dispatch(ctx context.Context, api *tg.Client, upd tg.UpdateClass) {
	switch u := upd.(type) {

	case *tg.UpdateBotInlineQuery:
		handleInlineQuery(ctx, u)

	case *tg.UpdateBotCallbackQuery:
		q := &manager.CallbackQuery{
			QueryID:      u.QueryID,
			UserID:       u.UserID,
			Data:         u.Data,
			ChatInstance: u.ChatInstance,
			Peer:         u.Peer,
			MsgID:        u.MsgID,
			IsInline:     false,
		}
		handleCallbackQuery(ctx, api, q)

	case *tg.UpdateInlineBotCallbackQuery:
		q := &manager.CallbackQuery{
			QueryID:         u.QueryID,
			UserID:          u.UserID,
			Data:            u.Data,
			ChatInstance:    u.ChatInstance,
			InlineMessageID: u.MsgID,
			IsInline:        true,
		}
		handleCallbackQuery(ctx, api, q)

	case *tg.UpdateBotInlineSend:
		slog.Debug("Bot: inline result dipilih", "result_id", u.ID)
	}
}

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
		break
	}

	if !handled {
		slog.Debug("Bot: tidak ada modul yang handle inline query", "query", q.Query)
	}
}

func handleCallbackQuery(ctx context.Context, api *tg.Client, q *manager.CallbackQuery) {
	data := string(q.Data)
	slog.Debug("Bot: callback query diterima", "data", data, "is_inline", q.IsInline)

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

	slog.Warn("Bot: tidak ada modul yang handle callback", "data", data)
	if b := getInstance(); b != nil && b.api != nil {
		_, _ = b.api.MessagesSetBotCallbackAnswer(ctx, &tg.MessagesSetBotCallbackAnswerRequest{
			QueryID: q.QueryID,
		})
	}
}
