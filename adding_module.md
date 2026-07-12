# Adding Modules

To add a new feature or command to the userbot:

1. **Create a new package directory** under `modules/` (e.g., `modules/hello/`).
2. **Create a Go file** inside it (e.g., `modules/hello/hello.go`) and register your module:

```go
package hello

import (
	"github.com/celestix/gotgproto/ext"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "Hello",
		Description: "Simple greeting command",
		Commands:    []string{"hello"},
		OnlyOut:     true,
		Handler: func(ctx *ext.Context, update *ext.Update) error {
			_, err := utils.EditMessageHTML(ctx, update.EffectiveChat().GetID(), update.EffectiveMessage.ID, "👋 Hello!")
			return err
		},
	})
}
```

> [!NOTE]
> To see all available configuration fields (such as `OnMessage`, `CallbackHandler`, `InlineHandler`, etc.), refer to the [manager.Module](file:///home/stefani/GolandProjects/userbot/modules/manager/manager.go#L30-L43) struct definition.


3. **Regenerate the imports** to load the module:
   ```bash
   go generate ./...
   ```
4. **Build the project** again:
   ```bash
   go build -o userbot
   ```