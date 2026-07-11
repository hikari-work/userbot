package voicechat

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/pion/webrtc/v3/pkg/media"

	"github.com/hikari-work/userbot/bot"
	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/modules/manager"
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

func streamAudio(pCtx context.Context, state *State, youtubeURL string) {
	defer func() {
		state.mu.Lock()
		state.isPlaying = false
		state.mu.Unlock()
	}()

	maxWait := 30 * time.Second
	waitStart := time.Now()
	isConnected := false
	for {
		state.mu.Lock()
		ready := state.isReady
		state.mu.Unlock()

		if ready {
			isConnected = true
			break
		}

		if time.Since(waitStart) > maxWait {
			break
		}

		select {
		case <-pCtx.Done():
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	if !isConnected {
		return
	}

	ytdlpCmd := exec.CommandContext(pCtx, "yt-dlp",
		"-f", "bestaudio",
		"--no-playlist",
		"--no-warnings",
		"-o", "-",
		youtubeURL,
	)
	ytdlpCmd.Stderr = os.Stderr

	ytdlpOut, err := ytdlpCmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := ytdlpCmd.Start(); err != nil {
		return
	}
	defer func() {
		_ = ytdlpCmd.Process.Kill()
	}()

	ffmpegCmd := exec.CommandContext(pCtx, "ffmpeg",
		"-loglevel", "error",
		"-i", "pipe:0",
		"-map", "0:a",
		"-c:a", "libopus",
		"-b:a", "64k",
		"-ar", "48000",
		"-ac", "2",
		"-frame_duration", "20",
		"-f", "ogg",
		"-page_duration", "20000",
		"pipe:1",
	)
	ffmpegCmd.Stdin = ytdlpOut
	ffmpegCmd.Stderr = os.Stderr

	ffmpegOut, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := ffmpegCmd.Start(); err != nil {
		return
	}
	defer func() {
		_ = ffmpegCmd.Process.Kill()
	}()

	oggReader, _, err := CustomOggNewWith(ffmpegOut)
	if err != nil {
		return
	}

	var nextTime time.Time
	var pending []byte
	for {
		select {
		case <-pCtx.Done():
			return
		default:
		}

		state.mu.Lock()
		if state.isPaused {
			state.mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			nextTime = time.Now()
			continue
		}
		audioTrack := state.audioTrack
		state.mu.Unlock()

		if audioTrack == nil {
			return
		}

		packets, _, err := oggReader.ParseNextPageSegments()
		if err != nil {
			return
		}

		if len(packets) == 0 {
			continue
		}

		if pending != nil {
			packets[0] = append(pending, packets[0]...)
			pending = nil
		}

		if oggReader.LastPageLastSegmentSize() == 255 {
			pending = packets[len(packets)-1]
			packets = packets[:len(packets)-1]
		}

		for _, pkt := range packets {
			if len(pkt) == 0 {
				continue
			}
			if bytes.HasPrefix(pkt, []byte("OpusHead")) || bytes.HasPrefix(pkt, []byte("OpusTags")) {
				continue
			}

			if len(pkt) < 20 {
				continue
			}

			samples := opusPacketSamples(pkt)
			if samples == 0 {
				samples = 960
			}
			sampleDuration := time.Duration(samples) * time.Second / 48000

			err = audioTrack.WriteSample(media.Sample{Data: pkt, Duration: sampleDuration})
			if err != nil {
				return
			}

			if nextTime.IsZero() {
				nextTime = time.Now()
			} else {
				nextTime = nextTime.Add(sampleDuration)
				time.Sleep(time.Until(nextTime))
			}
		}
	}
}

func getPlaylistItems(youtubeURL string) ([]PlaylistItem, error) {
	if youtubeURL == "" {
		return nil, fmt.Errorf("empty URL")
	}

	fallbackTitle := "Audio Stream"
	if strings.Contains(youtubeURL, "youtube") || strings.Contains(youtubeURL, "youtu.be") || len(youtubeURL) == 11 {
		fallbackTitle = "YouTube Audio"
	}

	cmd := exec.Command("yt-dlp", "--flat-playlist", "--print", "url", "--print", "title", "--no-warnings", youtubeURL)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		titleCmd := exec.Command("yt-dlp", "--print", "title", "--no-warnings", youtubeURL)
		titleOutput, _ := titleCmd.Output()
		title := strings.TrimSpace(string(titleOutput))
		if title == "" {
			title = fallbackTitle
		}
		return []PlaylistItem{{URL: youtubeURL, Title: title}}, nil
	}

	lines := strings.Split(stdout.String(), "\n")
	var items []PlaylistItem
	for i := 0; i < len(lines)-1; i += 2 {
		url := strings.TrimSpace(lines[i])
		if url == "" {
			continue
		}
		title := fallbackTitle
		if i+1 < len(lines) {
			title = strings.TrimSpace(lines[i+1])
		}
		items = append(items, PlaylistItem{
			URL:   url,
			Title: title,
		})
	}
	if len(items) == 0 {
		return []PlaylistItem{{URL: youtubeURL, Title: fallbackTitle}}, nil
	}
	return items, nil
}

func playLoop(ctx *ext.Context, update *ext.Update, chatID int64) {
	state := getVCState(chatID)
	uMsg := update.EffectiveMessage

	for {
		state.mu.Lock()
		if len(state.Playlist) == 0 || state.isStopped {
			state.isPlaying = false
			state.mu.Unlock()

			textFinished := i18n.Localize("VCPlaybackFinished", nil, nil)
			text, entities := utils.ParseHTML(textFinished)
			_, _ = ctx.EditMessage(chatID, &tg.MessagesEditMessageRequest{
				ID:       uMsg.ID,
				Message:  text,
				Entities: entities,
			})
			break
		}

		item := state.Playlist[0]
		state.Playlist = state.Playlist[1:]

		playCtx, cancel := context.WithCancel(context.Background())
		state.cancelPlay = cancel
		state.isPlaying = true
		state.isPaused = false
		state.mu.Unlock()

		text, entities := utils.ParseHTML(i18n.Localize("VCNowStreaming", map[string]interface{}{"Title": html.EscapeString(item.Title)}, nil))
		sentMsg, editErr := ctx.EditMessage(chatID, &tg.MessagesEditMessageRequest{
			ID:       uMsg.ID,
			Message:  text,
			Entities: entities,
		})
		if editErr != nil {
			newMsg, err := ctx.Reply(update, ext.ReplyTextString(i18n.Localize("VCNowStreamingRaw", map[string]interface{}{"Title": item.Title}, nil)), nil)
			if err == nil {
				uMsg = newMsg
			}
		} else {
			uMsg = sentMsg
		}

		streamAudio(playCtx, state, item.URL)
	}
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
