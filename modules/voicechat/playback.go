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
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "❌ <b>Error:</b> Please provide a valid YouTube URL (or 11-character video ID), or reply to a message containing a YouTube link.")
		return nil
	}

	state := getVCState(uChat.GetID())

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
	if state.cancelPlay != nil {
		state.cancelPlay()
	}
	playCtx, cancel := context.WithCancel(context.Background())
	state.cancelPlay = cancel
	state.isPlaying = true
	state.isPaused = false
	state.mu.Unlock()

	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "🎵 <b>Starting stream...</b>")

	// Fetch title asynchronously — tidak perlu nunggu download selesai
	go func() {
		titleCmd := exec.Command("yt-dlp", "--print", "title", "--no-warnings", youtubeURL)
		titleOutput, _ := titleCmd.Output()
		videoTitle := strings.TrimSpace(string(titleOutput))
		if videoTitle == "" {
			videoTitle = "YouTube Audio"
		}
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("🎵 <b>Now streaming:</b> %s", html.EscapeString(videoTitle)))
	}()

	go streamAudio(playCtx, state, youtubeURL)

	return nil
}

// streamAudio streams audio directly from YouTube via yt-dlp → ffmpeg pipe, tanpa download file dulu
func streamAudio(pCtx context.Context, state *State, youtubeURL string) {
	defer func() {
		state.mu.Lock()
		state.isPlaying = false
		state.mu.Unlock()
	}()

	// Tunggu WebRTC connection established (max 30 detik)
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

	// Jalankan yt-dlp dengan output ke stdout (-o -)
	ytdlpCmd := exec.CommandContext(pCtx, "yt-dlp",
		"-f", "bestaudio",
		"--no-playlist",
		"--no-warnings",
		"-o", "-", // output ke stdout
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

	// Jalankan ffmpeg, baca dari stdin (yt-dlp stdout), encode ke Opus/Ogg dan output ke stdout
	ffmpegCmd := exec.CommandContext(pCtx, "ffmpeg",
		"-i", "pipe:0", // baca dari stdin
		"-map", "0:a",
		"-c:a", "libopus",
		"-b:a", "64k",
		"-ar", "48000",
		"-ac", "2",
		"-frame_duration", "20",
		"-f", "ogg",
		"-page_duration", "20000",
		"pipe:1", // output ke stdout
	)
	ffmpegCmd.Stdin = ytdlpOut // pipe yt-dlp stdout → ffmpeg stdin
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

	// Baca OGG frames dari ffmpeg stdout
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
