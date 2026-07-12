package menu

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/bot"
	dbClient "github.com/hikari-work/userbot/connection"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
)

type LogicalModule struct {
	ID          string
	Name        string
	Description string
	Commands    []string
}

func getPackageName(handler interface{}) string {
	if handler == nil {
		return ""
	}
	funcValue := reflect.ValueOf(handler)
	if funcValue.Kind() != reflect.Func {
		return ""
	}
	funcName := runtime.FuncForPC(funcValue.Pointer()).Name()
	const modulesMarker = "modules/"
	idx := strings.LastIndex(funcName, modulesMarker)
	if idx != -1 {
		subPath := funcName[idx+len(modulesMarker):]
		dotIdx := strings.Index(subPath, ".")
		slashIdx := strings.Index(subPath, "/")

		endIdx := dotIdx
		if slashIdx != -1 && (endIdx == -1 || slashIdx < endIdx) {
			endIdx = slashIdx
		}
		if endIdx != -1 {
			return subPath[:endIdx]
		}
		return subPath
	}

	parts := strings.Split(funcName, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		dotIdx := strings.Index(lastPart, ".")
		if dotIdx >= 0 {
			return lastPart[:dotIdx]
		}
	}
	return ""
}

func getLogicalModules() []LogicalModule {

	groups := make(map[string]*LogicalModule)

	for _, mod := range manager.Registry {
		if strings.ToLower(mod.Name) == "menu" {
			continue
		}

		pkgName := getPackageName(mod.Handler)
		if pkgName == "" {
			pkgName = getPackageName(mod.OnMessage)
		}
		if pkgName == "" {
			pkgName = strings.ToLower(mod.Name)
		}

		lm, exists := groups[pkgName]
		if !exists {
			name := strings.ToTitle(pkgName)
			desc := mod.Description

			lm = &LogicalModule{
				ID:          pkgName,
				Name:        name,
				Description: desc,
				Commands:    []string{},
			}
			groups[pkgName] = lm
		}

		for _, cmd := range mod.Commands {
			found := false
			for _, c := range lm.Commands {
				if c == cmd {
					found = true
					break
				}
			}
			if !found {
				lm.Commands = append(lm.Commands, cmd)
			}
		}
	}

	var list []LogicalModule
	for _, lm := range groups {
		list = append(list, *lm)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return list
}

func getModulesPage(page int, chatID int64) (string, [][]bot.Button) {
	logicalMods := getLogicalModules()
	totalModules := len(logicalMods)

	totalPages := (totalModules + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	if page < 0 {
		page = totalPages - 1
	} else if page >= totalPages {
		page = 0
	}

	start := page * pageSize
	end := start + pageSize
	if end > totalModules {
		end = totalModules
	}

	var modRows [][]bot.Button
	var currentRow []bot.Button
	for i := start; i < end; i++ {
		mod := logicalMods[i]
		btn := bot.Button{
			Text:         mod.Name,
			CallbackData: fmt.Sprintf("menu:mod:%s:%d:%d", mod.ID, page, chatID),
		}
		currentRow = append(currentRow, btn)

		if len(currentRow) == 2 {
			modRows = append(modRows, currentRow)
			currentRow = nil
		}
	}
	if len(currentRow) > 0 {
		modRows = append(modRows, currentRow)
	}

	prevPage := page - 1
	nextPage := page + 1

	navRow := []bot.Button{
		{Text: i18n.Localize("MenuPrevBtn", nil, nil), CallbackData: fmt.Sprintf("menu:page:%d:%d", prevPage, chatID)},
		{Text: i18n.Localize("MenuCloseBtn", nil, nil), CallbackData: fmt.Sprintf("menu:close:%d", chatID)},
		{Text: i18n.Localize("MenuNextBtn", nil, nil), CallbackData: fmt.Sprintf("menu:page:%d:%d", nextPage, chatID)},
	}
	modRows = append(modRows, navRow)

	text := i18n.Localize("MenuListText", map[string]interface{}{
		"Page":  page + 1,
		"Total": totalPages,
	}, nil)

	return text, modRows
}

func getCommandDescription(cmd string) string {
	for _, mod := range manager.Registry {
		for _, c := range mod.Commands {
			if strings.EqualFold(c, cmd) {
				return mod.Description
			}
		}
	}
	return ""
}

func getModuleDetail(ctx context.Context, mod *LogicalModule, fromPage string, chatID int64) (string, [][]bot.Button) {
	prefix, err := dbClient.Redis.Get(ctx, "prefix").Result()
	if err != nil || prefix == "" {
		prefix = "."
	}

	var cmdList []string
	if len(mod.Commands) > 0 {
		for _, cmd := range mod.Commands {
			desc := getCommandDescription(cmd)
			if desc != "" {
				cmdList = append(cmdList, fmt.Sprintf("- <code>%s%s</code> - %s", prefix, cmd, desc))
			} else {
				cmdList = append(cmdList, fmt.Sprintf("- <code>%s%s</code>", prefix, cmd))
			}
		}
	} else {
		cmdList = append(cmdList, i18n.Localize("MenuNoDirectCommands", nil, nil))
	}

	text := i18n.Localize("MenuModuleDetail", map[string]interface{}{
		"Name":     mod.Name,
		"Desc":     mod.Description,
		"Commands": strings.Join(cmdList, "\n"),
	}, nil)

	buttons := [][]bot.Button{
		{
			{Text: i18n.Localize("MenuBackBtn", nil, nil), CallbackData: fmt.Sprintf("menu:page:%s:%d", fromPage, chatID)},
			{Text: i18n.Localize("MenuCloseBtn", nil, nil), CallbackData: fmt.Sprintf("menu:close:%d", chatID)},
		},
	}

	return text, buttons
}

func inputPeerFromID(chatID int64) tg.InputPeerClass {
	if chatID > 0 {
		return &tg.InputPeerUser{UserID: chatID}
	}
	if chatID < -1_000_000_000_000 {
		channelID := -(chatID + 1_000_000_000_000)
		return &tg.InputPeerChannel{ChannelID: channelID}
	}
	return &tg.InputPeerChat{ChatID: -chatID}
}
