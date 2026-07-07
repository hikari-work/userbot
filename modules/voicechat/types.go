package voicechat

import (
	"sync"

	"github.com/pion/webrtc/v3"
	"golang.org/x/net/context"
)

type State struct {
	pc         *webrtc.PeerConnection
	audioTrack *webrtc.TrackLocalStaticSample
	cancelPlay context.CancelFunc
	mu         sync.Mutex
	isPaused   bool
	isPlaying  bool
	isReady    bool
}

type GroupJoinPayload struct {
	Ufrag        string             `json:"ufrag"`
	Pwd          string             `json:"pwd"`
	Fingerprints []GroupFingerprint `json:"fingerprints"`
	Ssrc         uint32             `json:"ssrc"`
}

type GroupFingerprint struct {
	Hash        string `json:"hash"`
	Setup       string `json:"setup"`
	Fingerprint string `json:"fingerprint"`
}

type GroupJoinResponse struct {
	Transport GroupTransportDescription `json:"transport"`
	Audio     *GroupMediaDescription    `json:"audio,omitempty"`
}

type GroupTransportDescription struct {
	Ufrag        string             `json:"ufrag"`
	Pwd          string             `json:"pwd"`
	Fingerprints []GroupFingerprint `json:"fingerprints"`
	Candidates   []GroupCandidate   `json:"candidates"`
}

type GroupCandidate struct {
	Foundation string `json:"foundation"`
	Component  string `json:"component"`
	Protocol   string `json:"protocol"`
	Priority   string `json:"priority"`
	IP         string `json:"ip"`
	Port       string `json:"port"`
	Type       string `json:"type"`
	Generation string `json:"generation"`
}

type GroupMediaDescription struct {
	PayloadTypes []GroupPayloadType     `json:"payload-types"`
	HDRExts      []GroupHeaderExtension `json:"rtp-hdrexts"`
}

type GroupHeaderExtension struct {
	ID  int    `json:"id"`
	URI string `json:"uri"`
}

type GroupPayloadType struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Clockrate int    `json:"clockrate"`
	Channels  int    `json:"channels,omitempty"`
}
