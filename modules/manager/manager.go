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

// CallbackQuery merepresentasikan callback query yang disatukan (dari normal / inline message)
type CallbackQuery struct {
	QueryID         int64
	UserID          int64
	Data            []byte
	ChatInstance    int64
	Peer            tg.PeerClass
	MsgID           int
	InlineMessageID tg.InputBotInlineMessageIDClass
	IsInline        bool
}

// CallbackQueryHandler adalah handler untuk callback query (tombol ditekan)
type CallbackQueryHandler func(ctx context.Context, query *CallbackQuery) error

type Module struct {
	Name        string
	Description string
	Commands    []string
	OnlyOut     bool

	// Userbot handlers
	Handler   ModuleHandler
	OnMessage ModuleHandler

	// Bot companion handlers
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
