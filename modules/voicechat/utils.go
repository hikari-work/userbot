package voicechat

import "strings"

func extractYouTubeURL(text string) string {
	words := strings.Fields(text)
	for _, w := range words {
		if strings.Contains(w, "youtube.com/") || strings.Contains(w, "youtu.be/") {
			w = strings.Trim(w, "()[]{}<>\"'")
			return w
		}
	}
	if len(text) == 11 && !strings.Contains(text, " ") {
		return text
	}
	return ""
}

func opusPacketSamples(pkt []byte) uint64 {
	if len(pkt) < 1 {
		return 0
	}
	toc := pkt[0]
	config := toc >> 3
	var frameUs uint64
	switch {
	case config <= 11:
		switch config % 4 {
		case 0:
			frameUs = 10000
		case 1:
			frameUs = 20000
		case 2:
			frameUs = 40000
		case 3:
			frameUs = 60000
		}
	case config <= 15:
		if config%2 == 0 {
			frameUs = 10000
		} else {
			frameUs = 20000
		}
	default:
		switch config % 4 {
		case 0:
			frameUs = 2500
		case 1:
			frameUs = 5000
		case 2:
			frameUs = 10000
		case 3:
			frameUs = 20000
		}
	}
	if frameUs == 0 {
		return 0
	}

	var frames uint64
	switch toc & 0x03 {
	case 0:
		frames = 1
	case 1, 2:
		frames = 2
	case 3:
		if len(pkt) < 2 {
			return 0
		}
		frames = uint64(pkt[1] & 0x3F)
		if frames == 0 {
			return 0
		}
	}
	return frames * frameUs * 48000 / 1000000
}
