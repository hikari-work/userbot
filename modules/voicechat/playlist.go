package voicechat

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/gotd/td/tg"
	"github.com/hikari-work/userbot/bot"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func PlaylistInlineHandler(ctx context.Context, q *tg.UpdateBotInlineQuery) error {
	if !strings.HasPrefix(q.Query, "vcpl") {
		return manager.ErrNotMatched
	}

	parts := strings.Split(q.Query, ":")
	if len(parts) < 3 {
		return nil
	}
	chatID, _ := strconv.ParseInt(parts[1], 10, 64)
	page, _ := strconv.Atoi(parts[2])

	text, buttons := getPlaylistPage(chatID, page)
	keyboard := bot.BuildInlineKeyboard(buttons)
	plainText, entities := utils.ParseHTML(text)

	result := &tg.InputBotInlineResult{
		ID:   "vcpl_main",
		Type: "article",
		SendMessage: &tg.InputBotInlineMessageText{
			Message:     plainText,
			Entities:    entities,
			ReplyMarkup: keyboard,
			NoWebpage:   true,
		},
	}
	result.SetTitle("Playlist Queue")
	result.SetDescription("Show the current voice chat playlist queue")

	return bot.AnswerInlineQuery(ctx, q.QueryID, []tg.InputBotInlineResultClass{result})
}

func getPlaylistPage(chatID int64, page int) (string, [][]bot.Button) {
	state := getVCState(chatID)
	state.mu.Lock()
	playlist := make([]PlaylistItem, len(state.Playlist))
	copy(playlist, state.Playlist)
	state.mu.Unlock()

	totalItems := len(playlist)
	pageSize := 5
	totalPages := (totalItems + pageSize - 1) / pageSize
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
	if end > totalItems {
		end = totalItems
	}

	var buttons [][]bot.Button
	for i := start; i < end; i++ {
		songIndex := i
		title := playlist[i].Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		btn := bot.Button{
			Text:         fmt.Sprintf("%d. %s", i+1, title),
			CallbackData: fmt.Sprintf("vcpl:play:%d:%d:%d", chatID, songIndex, page),
		}
		buttons = append(buttons, []bot.Button{btn})
	}

	prevPage := page - 1
	nextPage := page + 1
	navRow := []bot.Button{
		{Text: "◀️ Prev", CallbackData: fmt.Sprintf("vcpl:page:%d:%d", chatID, prevPage)},
		{Text: "❌ Close", CallbackData: fmt.Sprintf("vcpl:close:%d", chatID)},
		{Text: "▶️ Next", CallbackData: fmt.Sprintf("vcpl:page:%d:%d", chatID, nextPage)},
	}
	buttons = append(buttons, navRow)

	var text string
	if totalItems == 0 {
		text = "📭 <b>Playlist kosong.</b>\nGunakan <code>.play [link]</code> untuk menambahkan lagu."
	} else {
		text = fmt.Sprintf("🎵 <b>Daftar Putar Voice Chat</b> (Hal %d/%d, Total %d lagu):\n\n", page+1, totalPages, totalItems)
		for i, item := range playlist {
			prefixStr := "  "
			if i >= start && i < end {
				prefixStr = "👉"
			}
			text += fmt.Sprintf("%s <b>%d.</b> %s\n", prefixStr, i+1, html.EscapeString(item.Title))
		}
	}

	return text, buttons
}

func PlaylistCallbackHandler(ctx context.Context, q *manager.CallbackQuery) error {
	payload := strings.TrimPrefix(string(q.Data), "vcpl:")

	if strings.HasPrefix(payload, "close:") {
		parts := strings.Split(payload, ":")
		chatID, _ := strconv.ParseInt(parts[1], 10, 64)
		if q.IsInline {
			_ = bot.EditInlineBotMessage(q.InlineMessageID, ".", nil)
		} else {
			peer := inputPeerFromID(chatID)
			_ = bot.EditBotMessage(peer, q.MsgID, ".", nil)
		}
		return bot.AnswerCallbackQuery(ctx, q.QueryID, "Daftar putar ditutup.", false)
	}

	if strings.HasPrefix(payload, "page:") {
		parts := strings.Split(payload, ":")
		chatID, _ := strconv.ParseInt(parts[1], 10, 64)
		page, _ := strconv.Atoi(parts[2])

		text, buttons := getPlaylistPage(chatID, page)
		if q.IsInline {
			_ = bot.EditInlineBotMessage(q.InlineMessageID, text, buttons)
		} else {
			peer := inputPeerFromID(chatID)
			_ = bot.EditBotMessage(peer, q.MsgID, text, buttons)
		}
		return bot.AnswerCallbackQuery(ctx, q.QueryID, "", false)
	}

	if strings.HasPrefix(payload, "play:") {
		parts := strings.Split(payload, ":")
		chatID, _ := strconv.ParseInt(parts[1], 10, 64)
		songIndex, _ := strconv.Atoi(parts[2])
		page, _ := strconv.Atoi(parts[3])

		state := getVCState(chatID)
		state.mu.Lock()
		if songIndex >= len(state.Playlist) {
			state.mu.Unlock()
			return bot.AnswerCallbackQuery(ctx, q.QueryID, "Lagu tidak ditemukan.", true)
		}

		clickedSong := state.Playlist[songIndex]
		state.Playlist = append(state.Playlist[:songIndex], state.Playlist[songIndex+1:]...)
		state.Playlist = append([]PlaylistItem{clickedSong}, state.Playlist...)

		cancel := state.cancelPlay
		isPlaying := state.isPlaying
		extCtx := state.extCtx
		extUpdate := state.extUpdate
		state.mu.Unlock()

		if isPlaying && cancel != nil {
			cancel()
		} else if !isPlaying && extCtx != nil && extUpdate != nil {
			state.mu.Lock()
			state.isPlaying = true
			state.mu.Unlock()
			go playLoop(extCtx, extUpdate, chatID)
		}

		text, buttons := getPlaylistPage(chatID, page)
		if q.IsInline {
			_ = bot.EditInlineBotMessage(q.InlineMessageID, text, buttons)
		} else {
			peer := inputPeerFromID(chatID)
			_ = bot.EditBotMessage(peer, q.MsgID, text, buttons)
		}

		return bot.AnswerCallbackQuery(ctx, q.QueryID, fmt.Sprintf("Memutar: %s", clickedSong.Title), false)
	}

	return nil
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
