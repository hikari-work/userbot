package manager

import (
	"context"
	"errors"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

var ErrNotMatched = errors.New("query not matched")

type ModuleHandler func(ctx *ext.Context, update *ext.Update) error
type HelpString func() string

type InlineQueryHandler func(ctx context.Context, query *tg.UpdateBotInlineQuery) error

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

type CallbackQueryHandler func(ctx context.Context, query *CallbackQuery) error

type Module struct {
	Name        string
	Description string
	Commands    []string
	OnlyOut     bool
	Help        HelpString

	Handler   ModuleHandler
	OnMessage ModuleHandler

	CallbackPrefix  string
	CallbackHandler CallbackQueryHandler

	InlineHandler InlineQueryHandler
}

var Registry = make(map[string]*Module)

func Register(mod *Module) {
	if mod.Name == "" {
		panic("module must have a name")
	}
	Registry[mod.Name] = mod
}
