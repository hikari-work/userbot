// Package bot menyediakan Bot Companion yang berjalan paralel dengan userbot.
// Bot ini menangani inline query, callback query, dan pengiriman pesan dengan inline button.
package bot

import (
	"context"
	"log/slog"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"
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

// UserbotClient menyimpan instance client gotgproto dari userbot untuk aksi cross-client
var UserbotClient *gotgproto.Client

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

	// Inisialisasi SQLite session storage menggunakan sessionMaker gotgproto
	_, botSessionStorage, err := sessionMaker.NewSessionStorage(
		context.Background(),
		sessionMaker.SqlSession(sqlite.Open("bot_session")),
		false,
	)
	if err != nil {
		slog.Error("Gagal inisialisasi SQLite session storage untuk bot", "error", err)
		return err
	}

	b.client = telegram.NewClient(b.cfg.ApiId, b.cfg.ApiHash, telegram.Options{
		UpdateHandler:  handler,
		SessionStorage: botSessionStorage,
	})

	return b.client.Run(ctx, func(ctx context.Context) error {
		// Periksa apakah bot companion sudah terautentikasi (menggunakan session SQLite yang tersimpan)
		status, err := b.client.Auth().Status(ctx)
		if err != nil {
			return err
		}

		if !status.Authorized {
			slog.Info("Bot Companion belum terautentikasi, melakukan login dengan token...")
			auth, err := b.client.Auth().Bot(ctx, b.cfg.BotToken)
			if err != nil {
				return err
			}
			if u, ok := auth.User.(*tg.User); ok {
				BotUsername = u.Username
				slog.Info("Bot Companion login berhasil", "username", BotUsername)
			}
		} else {
			slog.Info("Bot Companion berhasil login menggunakan session SQLite lama (tanpa rpc token login)")
			if status.User != nil {
				BotUsername = status.User.Username
				slog.Info("Bot Companion username dari session SQLite", "username", BotUsername)
			}
		}

		b.api = b.client.API()
		slog.Info("✅ Bot Companion siap menerima updates")

		// Block sampai context selesai
		<-ctx.Done()
		return ctx.Err()
	})
}

// getInstance mengembalikan instance bot global (bisa nil jika dinonaktifkan)
func getInstance() *BotClient {
	return instance
}
