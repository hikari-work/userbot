// Package bot menyediakan Bot Companion yang berjalan paralel dengan userbot.
// Bot ini menangani inline query, callback query, dan pengiriman pesan dengan inline button.
package bot

import (
	"context"
	"log/slog"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	"github.com/hikari-work/userbot/config"
)

// BotClient membungkus gotd/td client yang berjalan sebagai bot
type BotClient struct {
	client *telegram.Client
	api    *tg.Client
	cfg    *config.Config
}

// Instance global agar bisa diakses dari helper (SendWithButtons, dll.)
var instance *BotClient

// BotUsername menyimpan username bot companion yang didapatkan saat auth login
var BotUsername string

// New membuat BotClient baru dari config.
// Mengembalikan nil dan log warning jika BOT_TOKEN kosong (opsional).
func New(cfg *config.Config) *BotClient {
	if cfg.BotToken == "" {
		slog.Warn("BOT_TOKEN tidak diset — Bot Companion dinonaktifkan")
		return nil
	}

	b := &BotClient{cfg: cfg}
	instance = b
	return b
}

// Run memulai bot client, melakukan autentikasi, dan mulai menerima update.
// Fungsi ini blocking — jalankan di goroutine terpisah.
func (b *BotClient) Run(ctx context.Context) error {
	handler := telegram.UpdateHandlerFunc(func(ctx context.Context, u tg.UpdatesClass) error {
		switch upds := u.(type) {
		case *tg.Updates:
			for _, upd := range upds.Updates {
				dispatch(ctx, b.api, upd)
			}
		case *tg.UpdatesCombined:
			for _, upd := range upds.Updates {
				dispatch(ctx, b.api, upd)
			}
		case *tg.UpdateShort:
			dispatch(ctx, b.api, upds.Update)
		}
		return nil
	})

	b.client = telegram.NewClient(b.cfg.ApiId, b.cfg.ApiHash, telegram.Options{
		UpdateHandler: handler,
	})

	return b.client.Run(ctx, func(ctx context.Context) error {
		// Autentikasi sebagai bot menggunakan token
		auth, err := b.client.Auth().Bot(ctx, b.cfg.BotToken)
		if err != nil {
			return err
		}

		b.api = b.client.API()
		slog.Info("✅ Bot Companion berhasil terautentikasi")

		if u, ok := auth.User.(*tg.User); ok {
			BotUsername = u.Username
			slog.Info("Bot Companion username retrieved", "username", BotUsername)
		}

		// Block sampai context selesai
		<-ctx.Done()
		return ctx.Err()
	})
}

// getInstance mengembalikan instance bot global (bisa nil jika dinonaktifkan)
func getInstance() *BotClient {
	return instance
}
