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

	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "⏳ <b>Downloading audio from YouTube...</b>")

	// Generate temporary file path (use .opus extension for yt-dlp output)
	tempFile := fmt.Sprintf("/tmp/vc-audio-%d-%d.opus", uChat.GetID(), time.Now().Unix())

	// Download audio using yt-dlp
	downloadCmd := exec.Command("yt-dlp",
		"-f", "bestaudio",
		"-x",
		"--audio-format", "opus",
		"--audio-quality", "0",
		"-o", tempFile,
		youtubeURL,
	)
	downloadCmd.Stderr = os.Stderr

	if err := downloadCmd.Run(); err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to download audio:</b> %v", err))
		return nil
	}

	// Verify file exists
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		// Try with .webm extension (fallback)
		tempFileWebm := strings.TrimSuffix(tempFile, ".opus") + ".webm"
		if _, err := os.Stat(tempFileWebm); err == nil {
			tempFile = tempFileWebm
		} else {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Downloaded file not found. Expected: %s</b>", tempFile))
			return nil
		}
	}

	// Get video title for display
	titleCmd := exec.Command("yt-dlp", "--get-title", youtubeURL)
	titleOutput, _ := titleCmd.Output()
	videoTitle := strings.TrimSpace(string(titleOutput))
	if videoTitle == "" {
		videoTitle = "Downloaded Audio"
	}

	state.mu.Lock()
	if state.cancelPlay != nil {
		state.cancelPlay()
	}
	playCtx, cancel := context.WithCancel(context.Background())
	state.cancelPlay = cancel
	state.isPlaying = true
	state.isPaused = false
	state.mu.Unlock()

	go playAudio(playCtx, state, tempFile)

	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("🎵 <b>Now playing:</b> %s", html.EscapeString(videoTitle)))
	return nil
}

// playAudio plays audio file in the voice chat
func playAudio(pCtx context.Context, state *State, audioFile string) {
	defer func() {
		state.mu.Lock()
		state.isPlaying = false
		state.mu.Unlock()

		// Clean up downloaded file after playback completes
		_ = os.Remove(audioFile)
	}()

	// Wait for WebRTC connection to establish (max 30 seconds)
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

	cmd := exec.Command("ffmpeg",
		"-i", audioFile, // Input from local file
		"-map", "0:a", // Map audio stream
		"-c:a", "libopus", // Encode to Opus
		"-b:a", "64k", // 64kbps bitrate
		"-ar", "48000", // 48kHz sample rate
		"-ac", "2", // Stereo
		"-frame_duration", "20", // 20ms frame duration
		"-f", "ogg", // Ogg container
		"-page_duration", "20000", // Ogg page duration
		"pipe:1", // Output to stdout
	)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	defer func() {
		_ = cmd.Process.Kill()
	}()

	oggReader, _, err := CustomOggNewWith(stdout)
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
