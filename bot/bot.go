// Package bot menyediakan Bot Companion yang berjalan paralel dengan userbot.
// Bot ini menangani inline query, callback query, dan pengiriman pesan dengan inline button.
package bot

import (
	"context"
	"log/slog"
	"sync"

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

// entityStore menyimpan access hash channel & user yang dipelajari dari update
var entityStore = &store{
	channels: make(map[int64]int64),
	users:    make(map[int64]int64),
}

type store struct {
	mu       sync.RWMutex
	channels map[int64]int64 // channelID → accessHash
	users    map[int64]int64 // userID → accessHash
}

func (s *store) saveChats(chats []tg.ChatClass) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range chats {
		if ch, ok := c.(*tg.Channel); ok {
			s.channels[ch.ID] = ch.AccessHash
		}
	}
}

func (s *store) saveUsers(users []tg.UserClass) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range users {
		if user, ok := u.(*tg.User); ok {
			s.users[user.ID] = user.AccessHash
		}
	}
}

// resolveChannelHash mengembalikan access hash channel dari store (0 jika tidak diketahui)
func (s *store) resolveChannelHash(channelID int64) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.channels[channelID]
}

// resolveUserHash mengembalikan access hash user dari store (0 jika tidak diketahui)
func (s *store) resolveUserHash(userID int64) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.users[userID]
}

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
	// UpdateHandler menerima push update dari gotd/td secara real-time
	handler := telegram.UpdateHandlerFunc(func(ctx context.Context, u tg.UpdatesClass) error {
		switch upds := u.(type) {
		case *tg.Updates:
			// Simpan entity (channel/user + access hash) dari setiap update
			entityStore.saveChats(upds.Chats)
			entityStore.saveUsers(upds.Users)
			for _, upd := range upds.Updates {
				dispatch(ctx, b.api, upd)
			}
		case *tg.UpdatesCombined:
			entityStore.saveChats(upds.Chats)
			entityStore.saveUsers(upds.Users)
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
		if _, err := b.client.Auth().Bot(ctx, b.cfg.BotToken); err != nil {
			return err
		}

		b.api = b.client.API()
		slog.Info("✅ Bot Companion berhasil terautentikasi")

		// Preload entity store: ambil semua dialog yang bot sudah ikuti
		if err := b.preloadEntities(ctx); err != nil {
			slog.Warn("Bot: gagal preload entities", "error", err)
		}

		// Block sampai context selesai
		<-ctx.Done()
		return ctx.Err()
	})
}

// preloadEntities mengambil daftar dialog sehingga entity store terisi
// sebelum ada update pertama masuk
func (b *BotClient) preloadEntities(ctx context.Context) error {
	dialogs, err := b.api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		Limit:      100,
		OffsetPeer: &tg.InputPeerEmpty{},
	})
	if err != nil {
		return err
	}

	switch d := dialogs.(type) {
	case *tg.MessagesDialogs:
		entityStore.saveChats(d.Chats)
		entityStore.saveUsers(d.Users)
		slog.Info("Bot: entity preload selesai",
			"chats", len(d.Chats), "users", len(d.Users))
	case *tg.MessagesDialogsSlice:
		entityStore.saveChats(d.Chats)
		entityStore.saveUsers(d.Users)
		slog.Info("Bot: entity preload selesai",
			"chats", len(d.Chats), "users", len(d.Users))
	}
	return nil
}

// getInstance mengembalikan instance bot global (bisa nil jika dinonaktifkan)
func getInstance() *BotClient {
	return instance
}
