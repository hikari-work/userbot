package chat

import (
	"fmt"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "CreateChat",
		Description: "Create Telegram channel or group",
		Commands:    []string{"createchannel", "createchan", "cc", "creategroup", "cg"},
		OnlyOut:     true,
		Help:        helpCreate,
		Handler:     createHandler,
	})
}

func helpCreate() string {
	return "Format:\n" +
		"<code>.createchannel</code> &lt;title&gt; [public/inviteonly] [@username] - Create a new channel\n" +
		"<code>.creategroup</code> &lt;title&gt; [public/inviteonly] [@username] - Create a new group/supergroup"
}

func parseCreateArgs(args []string) (title string, isPublic bool, username string) {
	var titleWords []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		lowerArg := strings.ToLower(arg)

		if lowerArg == "public" {
			isPublic = true
			continue
		}
		if lowerArg == "private" || lowerArg == "inviteonly" {
			isPublic = false
			continue
		}

		if strings.HasPrefix(arg, "@") && len(arg) > 1 {
			username = strings.TrimPrefix(arg, "@")
			isPublic = true
			continue
		}

		if (lowerArg == "-u" || lowerArg == "--username") && i+1 < len(args) {
			username = strings.TrimPrefix(args[i+1], "@")
			isPublic = true
			i++
			continue
		}

		if strings.HasPrefix(lowerArg, "username=") {
			username = strings.TrimPrefix(arg[len("username="):], "@")
			isPublic = true
			continue
		}

		titleWords = append(titleWords, arg)
	}

	title = strings.Join(titleWords, " ")
	return title, isPublic, username
}

func createHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if uMsg == nil || uChat == nil {
		return nil
	}

	rawArgs := strings.Fields(uMsg.Message.Message)
	if len(rawArgs) == 0 {
		return nil
	}

	cmd := strings.ToLower(rawArgs[0])
	cmdArgs := update.Args()

	title, isPublic, username := parseCreateArgs(cmdArgs)

	isChannelCmd := strings.HasSuffix(cmd, "createchannel") || strings.HasSuffix(cmd, "createchan") || strings.HasSuffix(cmd, "cc")

	if title == "" {
		usageKey := "CreateGroupUsage"
		if isChannelCmd {
			usageKey = "CreateChannelUsage"
		}
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize(usageKey, nil, nil))
		return nil
	}

	if isChannelCmd {
		return createChannel(ctx, update, title, isPublic, username)
	}
	return createGroup(ctx, update, title, isPublic, username)
}

func createChannel(ctx *ext.Context, update *ext.Update, title string, isPublic bool, username string) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	updates, err := ctx.Raw.ChannelsCreateChannel(ctx, &tg.ChannelsCreateChannelRequest{
		Broadcast: true,
		Title:     title,
	})
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("CreateChannelFailed", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	var channelID int64
	var accessHash int64
	for _, chat := range getChatsFromUpdates(updates) {
		if ch, ok := chat.(*tg.Channel); ok {
			channelID = ch.ID
			accessHash = ch.AccessHash
			break
		}
	}

	if channelID == 0 {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("CreateChannelFailed", map[string]interface{}{"Error": "Failed to retrieve channel info"}, nil))
		return nil
	}

	inputChannel := &tg.InputChannel{
		ChannelID:  channelID,
		AccessHash: accessHash,
	}

	typeStr := "Private"
	if isPublic || username != "" {
		typeStr = "Public"
	}

	if username != "" {
		_, errUser := ctx.Raw.ChannelsUpdateUsername(ctx, &tg.ChannelsUpdateUsernameRequest{
			Channel:  inputChannel,
			Username: username,
		})
		if errUser != nil {
			typeStr += fmt.Sprintf(" (Failed to set username: %s)", errUser.Error())
		}
	}

	var inviteLink string
	if username != "" {
		inviteLink = fmt.Sprintf("https://t.me/%s", username)
	} else {
		exportRes, errExp := ctx.Raw.MessagesExportChatInvite(ctx, &tg.MessagesExportChatInviteRequest{
			Peer: &tg.InputPeerChannel{
				ChannelID:  channelID,
				AccessHash: accessHash,
			},
		})
		if errExp == nil {
			if exportInvite, ok := exportRes.(*tg.ChatInviteExported); ok {
				inviteLink = exportInvite.Link
			}
		}
	}

	usernameDisplay := "None"
	if username != "" {
		usernameDisplay = "@" + username
	}
	if inviteLink == "" {
		inviteLink = "None"
	}

	successMsg := i18n.Localize("CreateChannelSuccess", map[string]interface{}{
		"Title":    title,
		"Type":     typeStr,
		"Username": usernameDisplay,
		"Link":     inviteLink,
	}, nil)

	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, successMsg)
	return nil
}

func createGroup(ctx *ext.Context, update *ext.Update, title string, isPublic bool, username string) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	updates, err := ctx.Raw.ChannelsCreateChannel(ctx, &tg.ChannelsCreateChannelRequest{
		Megagroup: true,
		Title:     title,
	})
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("CreateGroupFailed", map[string]interface{}{"Error": err.Error()}, nil))
		return err
	}

	var channelID int64
	var accessHash int64
	for _, chat := range getChatsFromUpdates(updates) {
		if ch, ok := chat.(*tg.Channel); ok {
			channelID = ch.ID
			accessHash = ch.AccessHash
			break
		}
	}

	if channelID == 0 {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("CreateGroupFailed", map[string]interface{}{"Error": "Failed to retrieve group info"}, nil))
		return nil
	}

	inputChannel := &tg.InputChannel{
		ChannelID:  channelID,
		AccessHash: accessHash,
	}

	typeStr := "Private"
	if isPublic || username != "" {
		typeStr = "Public"
	}

	if username != "" {
		_, errUser := ctx.Raw.ChannelsUpdateUsername(ctx, &tg.ChannelsUpdateUsernameRequest{
			Channel:  inputChannel,
			Username: username,
		})
		if errUser != nil {
			typeStr += fmt.Sprintf(" (Failed to set username: %s)", errUser.Error())
		}
	}

	var inviteLink string
	if username != "" {
		inviteLink = fmt.Sprintf("https://t.me/%s", username)
	} else {
		exportRes, errExp := ctx.Raw.MessagesExportChatInvite(ctx, &tg.MessagesExportChatInviteRequest{
			Peer: &tg.InputPeerChannel{
				ChannelID:  channelID,
				AccessHash: accessHash,
			},
		})
		if errExp == nil {
			if exportInvite, ok := exportRes.(*tg.ChatInviteExported); ok {
				inviteLink = exportInvite.Link
			}
		}
	}

	usernameDisplay := "None"
	if username != "" {
		usernameDisplay = "@" + username
	}
	if inviteLink == "" {
		inviteLink = "None"
	}

	successMsg := i18n.Localize("CreateGroupSuccess", map[string]interface{}{
		"Title":    title,
		"Type":     typeStr,
		"Username": usernameDisplay,
		"Link":     inviteLink,
	}, nil)

	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, successMsg)
	return nil
}

func getChatsFromUpdates(updates tg.UpdatesClass) []tg.ChatClass {
	switch u := updates.(type) {
	case *tg.Updates:
		return u.Chats
	case *tg.UpdatesCombined:
		return u.Chats
	}
	return nil
}
