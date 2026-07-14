//go:generate go run scripts/gen_imports.go
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/ext"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/celestix/gotgproto/types"
	"github.com/glebarez/sqlite"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/bot"
	"github.com/hikari-work/userbot/config"
	dbClient "github.com/hikari-work/userbot/connection"
	_ "github.com/hikari-work/userbot/modules"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
)

func init() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
}

func main() {

	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "time"
	cfg.EncoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	logger, _ := cfg.Build()
	rateLimiter := ratelimit.New(rate.Every(time.Millisecond*100), 30)
	newConfig := config.NewConfig()
	var sessionConstructor sessionMaker.SessionConstructor
	if newConfig.TelethonSession != "" {
		slog.Info("Using Telethon string session")
		sessionConstructor = sessionMaker.TelethonSession(newConfig.TelethonSession)
	} else if newConfig.PyrogramSession != "" {
		slog.Info("Using Pyrogram string session")
		sessionConstructor = sessionMaker.PyrogramSession(newConfig.PyrogramSession)
	} else if newConfig.GramjsSession != "" {
		slog.Info("Using GramJS string session")
		sessionConstructor = sessionMaker.GramjsSession(newConfig.GramjsSession)
	} else {
		slog.Info("Using SQL session database")
		_ = os.MkdirAll("sessions", 0755)
		if _, err := os.Stat("user_session"); err == nil {
			slog.Info("Migrating user_session to sessions/user_session")
			_ = os.Rename("user_session", "sessions/user_session")
		}
		if _, err := os.Stat("user_session-journal"); err == nil {
			_ = os.Rename("user_session-journal", "sessions/user_session-journal")
		}
		sessionConstructor = sessionMaker.SqlSession(sqlite.Open("sessions/user_session"))
	}

	client, err := gotgproto.NewClient(
		newConfig.ApiId,
		newConfig.ApiHash,
		gotgproto.ClientTypePhone(newConfig.PhoneNumber),
		&gotgproto.ClientOpts{
			Logger:      logger,
			Session:     sessionConstructor,
			Middlewares: []telegram.Middleware{rateLimiter},
		})
	if err != nil {
		slog.Error("Error Create Telegram User Client", "error", err)
		os.Exit(1)
	}
	conn, err := dbClient.NewRedisClient(newConfig)
	if err != nil {
		logger.Fatal("Error Connecting Redis: " + err.Error())
	}
	dbClient.Redis = conn
	defer func(conn *redis.Client) {
		err := conn.Close()
		if err != nil {

		}
	}(conn)

	ctxBg := context.Background()
	exists, err := dbClient.Redis.Exists(ctxBg, "prefix").Result()
	if err != nil {
		slog.Error("Gagal memeriksa key 'prefix' di Redis", "error", err)
	} else if exists == 0 {
		err = dbClient.Redis.Set(ctxBg, "prefix", ".", 0).Err()
		if err != nil {
			slog.Error("Gagal menyimpan key 'prefix' default ke Redis", "error", err)
		} else {
			slog.Info("Key 'prefix' tidak ditemukan di Redis, berhasil menyetel default '.'")
		}
	}

	botClient := bot.New(newConfig)
	if botClient != nil {
		bot.UserbotClient = client
		go func() {
			if err := botClient.Run(context.Background()); err != nil {
				slog.Error("Bot Companion berhenti", "error", err)
			}
		}()
	}

	initHandlers(client)
	go syncDialogs(context.Background(), client)
	err = client.Idle()
	if err != nil {
		return
	}
}

func initHandlers(client *gotgproto.Client) {
	dp := client.Dispatcher

	ctxBg := context.Background()
	prefixVal, err := dbClient.Redis.Get(ctxBg, "prefix").Result()
	if err != nil {
		slog.Warn("Gagal mengambil prefix dari Redis, menggunakan fallback '.'", "error", err)
		prefixVal = "."
	}
	prefixRunes := []rune(prefixVal)

	dp.AddHandler(handlers.NewMessage(
		func(m *types.Message) bool { return true },
		func(ctx *ext.Context, update *ext.Update) error {
			for _, mod := range manager.Registry {
				if mod.OnMessage != nil {
					if err := mod.OnMessage(ctx, update); err != nil {

					}
				}
			}
			return nil
		},
	))

	var commandHandlers []*handlers.Command

	for _, mod := range manager.Registry {
		if mod.Handler == nil {
			continue
		}
		for _, cmd := range mod.Commands {
			cmdHandler := handlers.NewCommand(cmd, func(ctx *ext.Context, update *ext.Update) error {

				if mod.OnlyOut && (update.EffectiveMessage == nil || !update.EffectiveMessage.Out) {
					return nil
				}
				return mod.Handler(ctx, update)
			})
			cmdHandler.Prefix = prefixRunes

			hPtr := &cmdHandler
			commandHandlers = append(commandHandlers, hPtr)
			dp.AddHandler(hPtr)
		}
	}

	dbClient.UpdatePrefixFunc = func(newPrefix string) {
		runes := []rune(newPrefix)
		for _, h := range commandHandlers {
			h.Prefix = runes
		}
		slog.Info("Prefix seluruh handler command telah diupdate secara dinamis", "new_prefix", newPrefix)
	}
}
func syncDialogs(ctx context.Context, client *gotgproto.Client) {
	slog.Info("Synchronizing")
	dialogs, err := client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetDate: 0,
		OffsetID:   0,
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100,
	})
	if err != nil {
		slog.Error("Failed Sync, ", "error", err)
		return
	}
	switch d := dialogs.(type) {
	case *tg.MessagesDialogsSlice:
		slog.Info("Syncronizing peers done", "users", len(d.Users), "chats", len(d.Chats))
	case *tg.MessagesDialogs:
		slog.Info("Syncronizing peers done", "users", len(d.Users), "chats", len(d.Chats))

	}

}
