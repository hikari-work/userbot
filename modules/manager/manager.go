package manager

import (
	"github.com/celestix/gotgproto/ext"
)

type ModuleHandler func(ctx *ext.Context, update *ext.Update) error

type Module struct {
	Name        string
	Description string
	Commands    []string
	OnlyOut     bool
	Handler     ModuleHandler
	OnMessage   ModuleHandler
}

var Registry = make(map[string]*Module)

func Register(mod *Module) {
	if mod.Name == "" {
		panic("module must have a name")
	}
	Registry[mod.Name] = mod
}
