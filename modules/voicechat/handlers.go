package voicechat

import (
	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"

	"github.com/hikari-work/userbot/i18n"
	"github.com/hikari-work/userbot/utils"
)

func LeaveVCHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if uChat.IsAUser() {
		return nil
	}

	state := getVCState(uChat.GetID())
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.pc == nil {
		text := i18n.Localize("VCNotJoined", nil, nil)
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, text)
		return nil
	}
	if state.cancelPlay != nil {
		state.cancelPlay()
	}
	_ = state.pc.Close()
	state.pc = nil
	state.audioTrack = nil
	state.isPlaying = false
	state.isPaused = false
	state.isReady = false

	call, err := getGroupCall(ctx, uChat.GetID())
	if err == nil && call != nil {
		_, _ = ctx.Raw.PhoneLeaveGroupCall(ctx, &tg.PhoneLeaveGroupCallRequest{
			Call: call,
		})
	}

	text := i18n.Localize("VCLeftSuccess", nil, nil)
	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, text)
	return nil
}

// PauseHandler handles pausing audio playback
func PauseHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	state := getVCState(uChat.GetID())
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.isPlaying {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("VCNoAudioPlaying", nil, nil))
		return nil
	}

	state.isPaused = true
	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("VCPaybackPaused", nil, nil))
	return nil
}

// ResumeHandler handles resuming audio playback
func ResumeHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	state := getVCState(uChat.GetID())
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.isPlaying {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("VCNoAudioPlaying", nil, nil))
		return nil
	}

	state.isPaused = false
	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, i18n.Localize("VCPaybackResumed", nil, nil))
	return nil
}

// StopHandler handles stopping audio playback
func StopHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	state := getVCState(uChat.GetID())
	state.mu.Lock()
	defer state.mu.Unlock()

	state.isStopped = true
	state.Playlist = nil
	if state.cancelPlay != nil {
		state.cancelPlay()
	}
	state.isPlaying = false
	state.isPaused = false

	text := i18n.Localize("VCPlaybackStopped", nil, nil)
	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, text)
	return nil
}
