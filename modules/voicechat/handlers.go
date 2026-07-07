package voicechat

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"

	"github.com/hikari-work/userbot/utils"
)

func JoinVCHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	if uChat.IsAUser() {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "❌ <b>Error:</b> Voice chat is only supported in groups/supergroups/channels.")
		return nil
	}

	args := update.Args()
	joinOn := true
	if len(args) > 0 {
		arg := strings.ToLower(args[0])
		if arg == "off" || arg == "stop" {
			joinOn = false
		}
	}

	if len(update.Args()) == 0 && strings.HasPrefix(strings.ToLower(uMsg.Text), ".leavevc") {
		joinOn = false
	}

	state := getVCState(uChat.GetID())

	if !joinOn {
		state.mu.Lock()
		defer state.mu.Unlock()
		if state.pc == nil {
			_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "ℹ️ Bot is not currently in a Voice Chat.")
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

		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "✅ Successfully left the Voice Chat.")
		return nil
	}

	state.mu.Lock()
	if state.pc != nil {
		state.mu.Unlock()
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "ℹ️ Bot is already in a Voice Chat.")
		return nil
	}
	state.mu.Unlock()

	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "⏳ <b>Joining Voice Chat...</b>")

	var groupCall *tg.InputGroupCall

	call, err := getGroupCall(ctx, uChat.GetID())
	if err != nil || call == nil {

		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to get voice chat info:</b> %v", err))
		return nil
	}

	groupCall = call

	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>MediaEngine error:</b> %v", err))
		return nil
	}

	_ = m.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: "urn:ietf:params:rtp-hdrext:ssrc-audio-level"}, webrtc.RTPCodecTypeAudio)
	_ = m.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time"}, webrtc.RTPCodecTypeAudio)
	_ = m.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01"}, webrtc.RTPCodecTypeAudio)

	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Interceptor registry error:</b> %v", err))
		return nil
	}
	i.Add(&TelegramVoIPInterceptorFactory{})

	se := webrtc.SettingEngine{}
	se.SetICETimeouts(30*time.Second, 60*time.Second, 2*time.Second)
	se.SetSrflxAcceptanceMinWait(0)
	se.SetNetworkTypes([]webrtc.NetworkType{
		webrtc.NetworkTypeUDP4,
		webrtc.NetworkTypeUDP6,
	})

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(m),
		webrtc.WithSettingEngine(se),
		webrtc.WithInterceptorRegistry(i),
	)

	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to create peer connection:</b> %v", err))
		return nil
	}

	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
	})

	pc.OnConnectionStateChange(func(connectionState webrtc.PeerConnectionState) {
		if connectionState == webrtc.PeerConnectionStateConnected {
			state := getVCState(uChat.GetID())
			state.mu.Lock()
			state.isReady = true
			state.mu.Unlock()

			// Unmute ourselves NOW that connection is established
			if groupCall != nil {
				editReq := &tg.PhoneEditGroupCallParticipantRequest{
					Call:        groupCall,
					Participant: &tg.InputPeerSelf{},
				}
				editReq.SetMuted(false)
				editReq.SetVolume(10000)

				_, _ = ctx.Raw.PhoneEditGroupCallParticipant(ctx, editReq)
			}
		}
	})

	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType:    webrtc.MimeTypeOpus,
		ClockRate:   48000,
		Channels:    2,
		SDPFmtpLine: "minptime=10;useinbandfec=1;stereo=1;sprop-stereo=1;maxaveragebitrate=510000",
	}, "audio", "gotd-audio")
	if err != nil {
		err := pc.Close()
		if err != nil {
			return err
		}
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to create audio track:</b> %v", err))
		return nil
	}

	audioSSRC := rand.Uint32() & 0x7fffffff
	if audioSSRC == 0 {
		audioSSRC = 1
	}
	transceiver, err := pc.AddTransceiverFromTrack(audioTrack, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendonly,
		SendEncodings: []webrtc.RTPEncodingParameters{{
			RTPCodingParameters: webrtc.RTPCodingParameters{SSRC: webrtc.SSRC(audioSSRC)},
		}},
	})
	if err != nil {
		err := pc.Close()
		if err != nil {
			return err
		}
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to add track:</b> %v", err))
		return nil
	}

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := transceiver.Sender().Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		err := pc.Close()
		if err != nil {
			return err
		}
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to create offer:</b> %v", err))
		return nil
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		err := pc.Close()
		if err != nil {
			return err
		}
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to set local description:</b> %v", err))
		return nil
	}

	time.Sleep(150 * time.Millisecond)

	localSDP := pc.LocalDescription().SDP
	ufrag, pwd, fingerprint, hash := extractSDPParams(localSDP)
	realSSRC := extractSSRC(localSDP)
	if realSSRC == 0 {
		realSSRC = audioSSRC
	}

	joinPayloadObj := GroupJoinPayload{
		Ufrag:        ufrag,
		Pwd:          pwd,
		Fingerprints: []GroupFingerprint{{Hash: hash, Setup: "passive", Fingerprint: fingerprint}},
		Ssrc:         realSSRC,
	}
	joinPayloadBytes, _ := json.Marshal(joinPayloadObj)

	updatesClass, err := ctx.Raw.PhoneJoinGroupCall(ctx, &tg.PhoneJoinGroupCallRequest{
		Call:   call,
		JoinAs: &tg.InputPeerSelf{},
		Params: tg.DataJSON{Data: string(joinPayloadBytes)},
	})
	if err != nil {
		_ = pc.Close()
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to join group call via Telegram:</b> %v", err))
		return nil
	}

	var connParams string
	switch u := updatesClass.(type) {
	case *tg.Updates:
		for _, upd := range u.Updates {
			if connUpd, ok := upd.(*tg.UpdateGroupCallConnection); ok {
				connParams = connUpd.Params.Data
				break
			}
		}
	case *tg.UpdatesCombined:
		for _, upd := range u.Updates {
			if connUpd, ok := upd.(*tg.UpdateGroupCallConnection); ok {
				connParams = connUpd.Params.Data
				break
			}
		}
	}

	if connParams == "" {
		err := pc.Close()
		if err != nil {
			return err
		}
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "❌ <b>Did not receive voice chat connection parameters from Telegram.</b>")
		return nil
	}

	var resp GroupJoinResponse
	if err := json.Unmarshal([]byte(connParams), &resp); err != nil {
		err := pc.Close()
		if err != nil {
			return err
		}
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to parse Telegram connection params:</b> %v", err))
		return nil
	}

	remoteSDP := buildAnswerSDP(resp)

	err = pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  remoteSDP,
	})
	if err != nil {
		err := pc.Close()
		if err != nil {
			return err
		}
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, fmt.Sprintf("❌ <b>Failed to set remote description:</b> %v", err))
		return nil
	}

	state.mu.Lock()
	state.pc = pc
	state.audioTrack = audioTrack
	state.mu.Unlock()

	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "✅ <b>Successfully joined Voice Chat!</b>")
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
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "ℹ️ No audio is currently playing.")
		return nil
	}

	state.isPaused = true
	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "⏸️ <b>Audio playback paused.</b>")
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
		_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "ℹ️ No audio is currently playing.")
		return nil
	}

	state.isPaused = false
	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "▶️ <b>Audio playback resumed.</b>")
	return nil
}

// StopHandler handles stopping audio playback
func StopHandler(ctx *ext.Context, update *ext.Update) error {
	uChat := update.EffectiveChat()
	uMsg := update.EffectiveMessage

	state := getVCState(uChat.GetID())
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.cancelPlay != nil {
		state.cancelPlay()
	}
	state.isPlaying = false
	state.isPaused = false

	_, _ = utils.EditMessageHTML(ctx, uChat.GetID(), uMsg.ID, "⏹️ <b>Playback stopped.</b>")
	return nil
}
