package voicechat

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/pion/webrtc/v3/pkg/media"

	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/utils"
)

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

	// Use yt-dlp to pipe audio data directly to ffmpeg.
	// Fall back to --get-url approach if direct pipe fails.
	_, errCookies := os.ReadFile("cookies.txt")
	ytArgs := []string{
		"-f", "bestaudio",
		"--no-playlist",
		"--no-warnings",
		"-o", "-",
	}
	if errCookies == nil {
		ytArgs = append(ytArgs, "--cookies", "cookies.txt")
	}
	ytArgs = append(ytArgs, youtubeURL)

	ytdlpCmd := exec.CommandContext(pCtx, "yt-dlp", ytArgs...)
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

	// Decode OGG pages in a separate goroutine to decouple network I/O from playback timing.
	// The channel buffer absorbs network jitter so the playback loop runs at a steady pace.
	type opusPacket struct {
		data     []byte
		duration time.Duration
	}
	packetCh := make(chan opusPacket, 200)

	go func() {
		defer close(packetCh)
		var pending []byte
		for {
			select {
			case <-pCtx.Done():
				return
			default:
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

				select {
				case packetCh <- opusPacket{data: pkt, duration: sampleDuration}:
				case <-pCtx.Done():
					return
				}
			}
		}
	}()

	// Pre-buffer: wait until channel has some packets before starting playback
	// to avoid initial stutter from network latency.
	prebufferTimer := time.NewTimer(300 * time.Millisecond)
	select {
	case <-prebufferTimer.C:
	case <-pCtx.Done():
		prebufferTimer.Stop()
		return
	}
	prebufferTimer.Stop()

	var nextTime time.Time
	for pkt := range packetCh {
		// Check context cancellation
		select {
		case <-pCtx.Done():
			return
		default:
		}

		// Handle pause
		state.mu.Lock()
		for state.isPaused {
			state.mu.Unlock()
			select {
			case <-time.After(100 * time.Millisecond):
			case <-pCtx.Done():
				return
			}
			state.mu.Lock()
		}
		audioTrack := state.audioTrack
		state.mu.Unlock()

		if audioTrack == nil {
			return
		}

		err = audioTrack.WriteSample(media.Sample{Data: pkt.data, Duration: pkt.duration})
		if err != nil {
			return
		}

		if nextTime.IsZero() {
			nextTime = time.Now()
		} else {
			nextTime = nextTime.Add(pkt.duration)
			sleepDur := time.Until(nextTime)
			if sleepDur > 0 {
				time.Sleep(sleepDur)
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
	_, errRead := os.ReadFile("cookies.txt")
	var cmd *exec.Cmd

	if errRead != nil {
		cmd = exec.Command("yt-dlp", "--flat-playlist", "--print", "url", "--print", "title", "--no-warnings", youtubeURL)
	} else {
		cmd = exec.Command("yt-dlp", "--flat-playlist", "--print", "url", "--print", "title", "--no-warnings", "--cookies", "cookies.txt", youtubeURL)
	}

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
