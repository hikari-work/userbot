package chat

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

var emojiPool = []string{
	"😀", "😃", "😄", "😁", "😆", "😅", "😂", "🤣", "😊", "😇",
	"🙂", "🙃", "😉", "😌", "😍", "🥰", "😘", "😗", "😙", "😚",
	"😋", "😛", "😝", "😜", "🤪", "🤨", "🧐", "🤓", "😎", "🤩",
	"🥳", "😏", "😒", "😞", "😔", "😟", "😕", "🙁", "☹️", "😣",
	"😖", "😫", "😩", "🥺", "😢", "😭", "😤", "😠", "😡", "🤬",
	"🤯", "😳", "🥵", "🥶", "😱", "😨", "😰", "😥", "😓", "🤗",
	"🤔", "🤭", "🤫", "🤥", "😶", "😐", "😑", "😬", "🙄", "😯",
	"😦", "😧", "😮", "😲", "🥱", "😴", "🤤", "😪", "😵", "🤐",
	"🥴", "🤢", "🤮", "🤧", "😷", "🤒", "🤕", "🤑", "🤠", "😈",
	"👿", "👹", "👺", "🤡", "💩", "👻", "💀", "☠️", "👽", "👾",
	"🤖", "🎃", "😺", "😸", "😹", "😻", "😼", "😽", "🙀", "😿",
	"😾", "👋", "🤚", "🖐", "✋", "🖖", "👌", "🤏", "✌️", "🤞",
	"🤟", "🤘", "🤙", "👈", "👉", "👆", "🖕", "👇", "☝️", "👍",
	"👎", "✊", "👊", "🤛", "🤜", "👏", "🙌", "👐", "🤲", "🤝",
	"🙏", "✍️", "💅", "🤳", "💪", "👑", "🎩", "🧢", "👒", "🎓",
	"💍", "💎", "⭐", "🌟", "✨", "💥", "🔥", "🌈", "☀️", "⚡",
	"🎈", "🎉", "🎊", "🎁", "🎗️", "🏆", "🥇", "⚽", "🏀", "🏈",
	"⚾", "🎾", "🏐", "🎯", "🎮", "🕹️", "🎲", "🎨", "🎤", "🎧",
}

func init() {
	manager.Register(&manager.Module{
		Name:        "TagAll",
		Description: "Mention all members in a group with emojis",
		Commands:    []string{"tagall", "all", "mentionall"},
		OnlyOut:     true,
		Help:        helpTagAll,
		Handler:     tagAllHandler,
	})
}

func helpTagAll() string {
	return "Format:\n" +
		"<code>.tagall</code> [limit=5] [delay=1s] [message]\n" +
		"<i>Examples:</i>\n" +
		"<code>.tagall Halo rapat!</code>\n" +
		"<code>.tagall limit=10 delay=2s Pesan penting</code>"
}

func parseTagAllArgs(args []string) (limit int, delay time.Duration, customMsg string) {
	limit = 5
	delay = 1 * time.Second

	var msgWords []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		lowerArg := strings.ToLower(arg)

		if strings.HasPrefix(lowerArg, "limit=") {
			valStr := arg[len("limit="):]
			if val, err := strconv.Atoi(valStr); err == nil && val > 0 {
				limit = val
				continue
			}
		}
		if (lowerArg == "-l" || lowerArg == "--limit") && i+1 < len(args) {
			if val, err := strconv.Atoi(args[i+1]); err == nil && val > 0 {
				limit = val
				i++
				continue
			}
		}

		if strings.HasPrefix(lowerArg, "delay=") {
			valStr := arg[len("delay="):]
			if d, err := time.ParseDuration(valStr); err == nil && d >= 0 {
				delay = d
				continue
			} else if sec, err := strconv.Atoi(valStr); err == nil && sec >= 0 {
				delay = time.Duration(sec) * time.Second
				continue
			}
		}
		if (lowerArg == "-d" || lowerArg == "--delay") && i+1 < len(args) {
			if d, err := time.ParseDuration(args[i+1]); err == nil && d >= 0 {
				delay = d
				i++
				continue
			} else if sec, err := strconv.Atoi(args[i+1]); err == nil && sec >= 0 {
				delay = time.Duration(sec) * time.Second
				i++
				continue
			}
		}

		msgWords = append(msgWords, arg)
	}

	customMsg = strings.Join(msgWords, " ")
	return limit, delay, customMsg
}

func tagAllHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if uMsg == nil || uChat == nil {
		return nil
	}

	if uChat.IsAUser() {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("TagallGroupOnly", nil, nil))
		return nil
	}

	inputPeer, err := ctx.ResolveInputPeerById(uChat.GetID())
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("TagallGroupOnly", nil, nil))
		return err
	}

	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("TagallFetching", nil, nil))

	cmdArgs := update.Args()
	limit, delayDuration, customMsg := parseTagAllArgs(cmdArgs)

	var users []*tg.User

	switch p := inputPeer.(type) {
	case *tg.InputPeerChannel:
		offset := 0
		fetchLimit := 100
		for {
			res, fetchErr := ctx.Raw.ChannelsGetParticipants(ctx, &tg.ChannelsGetParticipantsRequest{
				Channel: &tg.InputChannel{
					ChannelID:  p.ChannelID,
					AccessHash: p.AccessHash,
				},
				Filter: &tg.ChannelParticipantsRecent{},
				Offset: offset,
				Limit:  fetchLimit,
			})
			if fetchErr != nil {
				break
			}
			cp, ok := res.(*tg.ChannelsChannelParticipants)
			if !ok || len(cp.Users) == 0 {
				break
			}
			for _, uClass := range cp.Users {
				if u, ok := uClass.(*tg.User); ok && !u.Bot && !u.Deleted {
					users = append(users, u)
				}
			}
			if len(cp.Participants) < fetchLimit {
				break
			}
			offset += len(cp.Participants)
		}
	case *tg.InputPeerChat:
		fullChat, fetchErr := ctx.Raw.MessagesGetFullChat(ctx, p.ChatID)
		if fetchErr == nil && fullChat != nil {
			for _, uClass := range fullChat.Users {
				if u, ok := uClass.(*tg.User); ok && !u.Bot && !u.Deleted {
					users = append(users, u)
				}
			}
		}
	}

	if len(users) == 0 {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("TagallNoMembers", nil, nil))
		return nil
	}

	headerText := i18n.Localize("TagallHeader", nil, nil)

	for i := 0; i < len(users); i += limit {
		end := i + limit
		if end > len(users) {
			end = len(users)
		}

		chunkUsers := users[i:end]
		var mentionLinks []string

		for idx, u := range chunkUsers {
			emojiIndex := (i + idx) % len(emojiPool)
			emoji := emojiPool[emojiIndex]
			mentionLinks = append(mentionLinks, fmt.Sprintf(`<a href="tg://user?id=%d">%s</a>`, u.ID, emoji))
		}

		var sb strings.Builder
		sb.WriteString(headerText)
		if customMsg != "" {
			sb.WriteString("\n<i>" + customMsg + "</i>")
		}
		sb.WriteString("\n\n")
		sb.WriteString(strings.Join(mentionLinks, " "))

		msgContent := sb.String()

		if i == 0 {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, msgContent)
		} else {
			text, entities := utils.ParseHTML(msgContent)
			_, _ = ctx.SendMessage(uChat.GetID(), &tg.MessagesSendMessageRequest{
				Message:  text,
				Entities: entities,
			})
		}

		if end < len(users) && delayDuration > 0 {
			time.Sleep(delayDuration)
		}
	}

	return nil
}
