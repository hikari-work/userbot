package voicechat

import (
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
)

type TelegramVoIPInterceptorFactory struct{}

func (f *TelegramVoIPInterceptorFactory) NewInterceptor(id string) (interceptor.Interceptor, error) {
	return &TelegramVoIPInterceptor{streams: make(map[uint32]*telegramVoIPStream)}, nil
}

type TelegramVoIPInterceptor struct {
	interceptor.NoOp
	mu      sync.RWMutex
	streams map[uint32]*telegramVoIPStream
}

type telegramVoIPStream struct {
	audioLevelID   uint8
	absSendTimeID  uint8
	transportCCID  uint8
	hasAudioLevel  bool
	hasAbsSend     bool
	hasTransportCC bool
	twccSeq        uint16
}

func (a *TelegramVoIPInterceptor) BindLocalStream(info *interceptor.StreamInfo, writer interceptor.RTPWriter) interceptor.RTPWriter {
	s := &telegramVoIPStream{}
	for _, ext := range info.RTPHeaderExtensions {
		switch ext.URI {
		case "urn:ietf:params:rtp-hdrext:ssrc-audio-level":
			s.audioLevelID = uint8(ext.ID)
			s.hasAudioLevel = true
		case "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time":
			s.absSendTimeID = uint8(ext.ID)
			s.hasAbsSend = true
		case "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01":
			s.transportCCID = uint8(ext.ID)
			s.hasTransportCC = true
		}
	}

	a.mu.Lock()
	if a.streams == nil {
		a.streams = make(map[uint32]*telegramVoIPStream)
	}
	a.streams[info.SSRC] = s
	a.mu.Unlock()

	return interceptor.RTPWriterFunc(func(header *rtp.Header, payload []byte, attrs interceptor.Attributes) (int, error) {
		if s.hasAudioLevel {
			_ = header.SetExtension(s.audioLevelID, []byte{0x80 | 20}) // -20 dB
		}
		if s.hasAbsSend {
			now := time.Now()
			abs := (uint64(now.Unix())<<18 | uint64(now.Nanosecond())*uint64(1<<18)/uint64(1e9)) & 0x00FFFFFF
			_ = header.SetExtension(s.absSendTimeID, []byte{byte(abs >> 16), byte(abs >> 8), byte(abs)})
		}
		if s.hasTransportCC {
			s.twccSeq++
			_ = header.SetExtension(s.transportCCID, []byte{byte(s.twccSeq >> 8), byte(s.twccSeq)})
		}
		header.Marker = false
		return writer.Write(header, payload, attrs)
	})
}

func (a *TelegramVoIPInterceptor) UnbindLocalStream(info *interceptor.StreamInfo) {
	a.mu.Lock()
	delete(a.streams, info.SSRC)
	a.mu.Unlock()
}
