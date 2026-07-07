package manager

import (
	"context"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

// ModuleHandler adalah handler untuk command userbot
type ModuleHandler func(ctx *ext.Context, update *ext.Update) error

// InlineQueryHandler adalah handler untuk inline query (@bot <query>)
type InlineQueryHandler func(ctx context.Context, query *tg.UpdateBotInlineQuery) error

// CallbackQueryHandler adalah handler untuk callback query (tombol ditekan)
type CallbackQueryHandler func(ctx context.Context, query *tg.UpdateBotCallbackQuery) error

type Module struct {
	Name        string
	Description string
	Commands    []string
	OnlyOut     bool

	// Userbot handlers
	Handler   ModuleHandler
	OnMessage ModuleHandler

	// Bot companion handlers
	// CallbackPrefix adalah prefix callback data yang dimiliki modul ini (misal "menu")
	// dispatcher akan routing callback "menu:ping" ke modul yang punya prefix "menu"
	CallbackPrefix  string
	CallbackHandler CallbackQueryHandler

	// InlineHandler dipanggil saat user ketik @bot <query>
	InlineHandler InlineQueryHandler
}

var Registry = make(map[string]*Module)

func Register(mod *Module) {
	if mod.Name == "" {
		panic("module must have a name")
	}
	Registry[mod.Name] = mod
}
