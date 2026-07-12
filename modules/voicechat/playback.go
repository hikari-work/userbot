package voicechat

import (
	"fmt"
	"html"
	"math/rand"
	"strings"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"

	"github.com/hikari-work/userbot/bot"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/utils"
)

func PlayHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if uChat.IsAUser() {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "❌ <b>Error:</b> Play is only supported in groups/supergroups/channels.")
		return nil
	}

	var youtubeURL string
	args := update.Args()
	if len(args) > 0 {
		youtubeURL = extractYouTubeURL(strings.Join(args, " "))
	}

	if youtubeURL == "" && uMsg.ReplyTo != nil {
		if reply, ok := uMsg.ReplyTo.(*tg.MessageReplyHeader); ok && reply.ReplyToMsgID != 0 {
			m, err := ctx.GetMessages(uChat.GetID(), []tg.InputMessageClass{&tg.InputMessageID{
				ID: reply.ReplyToMsgID,
			}})
			if err == nil && len(m) > 0 {
				if repliedMsg, ok := m[0].(*tg.Message); ok {
					youtubeURL = extractYouTubeURL(repliedMsg.Message)
				}
			}
		}
	}

	if youtubeURL == "" {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("VCInvalidYouTube", nil, nil))
		return nil
	}

	textFetching := i18n.Localize("VCFetchingInfo", nil, nil)
	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, textFetching)

	items, err := getPlaylistItems(youtubeURL)
	if err != nil || len(items) == 0 {
		textFailed := i18n.Localize("VCFetchFailed", nil, nil)
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, textFailed)
		return nil
	}

	state := getVCState(uChat.GetID())

	state.mu.Lock()
	state.extCtx = ctx
	state.extUpdate = update
	state.isStopped = false
	state.Playlist = append(state.Playlist, items...)
	isPlaying := state.isPlaying
	state.mu.Unlock()

	if isPlaying {
		if len(items) == 1 {
			textAdded := i18n.Localize("VCAddedToPlaylist", map[string]interface{}{
				"Title": html.EscapeString(items[0].Title),
			}, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, textAdded)
		} else {
			textAddedCount := i18n.Localize("VCAddedSongsCount", map[string]interface{}{
				"Count": len(items),
			}, nil)
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, textAddedCount)
		}
		return nil
	}

	state.mu.Lock()
	if state.pc == nil {
		state.mu.Unlock()
		err := JoinVCHandler(ctx, update)
		if err != nil {
			return err
		}
		state = getVCState(uChat.GetID())
		state.mu.Lock()
		if state.pc == nil {
			state.mu.Unlock()
			return nil
		}
	}
	state.mu.Unlock()

	state.mu.Lock()
	state.isPlaying = true
	state.mu.Unlock()

	go playLoop(ctx, update, uChat.GetID())

	return nil
}




func SkipHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if uChat.IsAUser() {
		return nil
	}

	state := getVCState(uChat.GetID())
	state.mu.Lock()
	playing := state.isPlaying
	cancel := state.cancelPlay
	state.mu.Unlock()

	_ = ctx.DeleteMessages(uChat.GetID(), []int{uMsg.ID})

	if playing && cancel != nil {
		cancel()
	}
	return nil
}

func PlaylistHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if uChat.IsAUser() {
		return nil
	}

	botUsername := bot.Username
	if botUsername == "" {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "⚠️ Bot Companion is starting up. Please wait...")
		return nil
	}

	botInputPeer, err := ctx.ResolveUsername(botUsername)
	if err != nil {
		return err
	}

	chatInputPeer, err := ctx.ResolveInputPeerById(uChat.GetID())
	if err != nil {
		return err
	}

	_ = ctx.DeleteMessages(uChat.GetID(), []int{uMsg.ID})

	results, err := ctx.Raw.MessagesGetInlineBotResults(ctx, &tg.MessagesGetInlineBotResultsRequest{
		Bot:    botInputPeer.GetInputUser(),
		Peer:   chatInputPeer,
		Query:  fmt.Sprintf("vcpl:%d:0", uChat.GetID()),
		Offset: "",
	})
	if err != nil {
		return err
	}

	if len(results.Results) == 0 {
		return fmt.Errorf("bot did not return inline results")
	}

	_, err = ctx.Raw.MessagesSendInlineBotResult(ctx, &tg.MessagesSendInlineBotResultRequest{
		Peer:     chatInputPeer,
		RandomID: rand.Int63(),
		QueryID:  results.QueryID,
		ID:       results.Results[0].GetID(),
	})
	return err
}

