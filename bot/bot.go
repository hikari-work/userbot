package bot

import (
	"context"
	"log/slog"
	"os"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	"github.com/hikari-work/userbot/config"
)

type Client struct {
	client *telegram.Client
	api    *tg.Client
	cfg    *config.Config
}

var instance *Client
var Username string
var UserbotClient *gotgproto.Client

func New(cfg *config.Config) *Client {
	if cfg.BotToken == "" {
		slog.Warn("BOT_TOKEN tidak diset — Bot Companion dinonaktifkan")
		return nil
	}

	b := &Client{cfg: cfg}
	instance = b
	return b
}

func (b *Client) Run(ctx context.Context) error {
	handler := telegram.UpdateHandlerFunc(func(ctx context.Context, u tg.UpdatesClass) error {
		switch upds := u.(type) {
		case *tg.Updates:
			for _, upd := range upds.Updates {
				updCopy := upd
				go dispatch(context.Background(), b.api, updCopy)
			}
		case *tg.UpdatesCombined:
			for _, upd := range upds.Updates {
				updCopy := upd
				go dispatch(context.Background(), b.api, updCopy)
			}
		case *tg.UpdateShort:
			go dispatch(context.Background(), b.api, upds.Update)
		}
		return nil
	})

	_ = os.MkdirAll("sessions", 0755)
	if _, err := os.Stat("bot_session"); err == nil {
		slog.Info("Migrating bot_session to sessions/bot_session")
		_ = os.Rename("bot_session", "sessions/bot_session")
	}
	if _, err := os.Stat("bot_session-journal"); err == nil {
		_ = os.Rename("bot_session-journal", "sessions/bot_session-journal")
	}

	_, botSessionStorage, err := sessionMaker.NewSessionStorage(
		context.Background(),
		sessionMaker.SqlSession(sqlite.Open("sessions/bot_session")),
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
				Username = u.Username
				slog.Info("Bot Companion login berhasil", "username", Username)
			}
		} else {
			slog.Info("Bot Companion berhasil login menggunakan session SQLite lama (tanpa rpc token login)")
			if status.User != nil {
				Username = status.User.Username
				slog.Info("Bot Companion username dari session SQLite", "username", Username)
			}
		}

		b.api = b.client.API()
		slog.Info("✅ Bot Companion siap menerima updates")

		<-ctx.Done()
		return ctx.Err()
	})
}

func getInstance() *Client {
	return instance
}
