package voicechat

import "testing"

func TestExtractYouTubeURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "full youtube watch URL",
			input: "https://www.youtube.com/watch?v=HEAn4FqXFY4",
			want:  "https://www.youtube.com/watch?v=HEAn4FqXFY4",
		},
		{
			name:  "short youtu.be URL",
			input: "https://youtu.be/HEAn4FqXFY4",
			want:  "https://youtu.be/HEAn4FqXFY4",
		},
		{
			name:  "URL with text around it",
			input: "play this https://www.youtube.com/watch?v=HEAn4FqXFY4 please",
			want:  "https://www.youtube.com/watch?v=HEAn4FqXFY4",
		},
		{
			name:  "URL in brackets",
			input: "[https://www.youtube.com/watch?v=HEAn4FqXFY4]",
			want:  "https://www.youtube.com/watch?v=HEAn4FqXFY4",
		},
		{
			name:  "URL in angle brackets",
			input: "<https://www.youtube.com/watch?v=HEAn4FqXFY4>",
			want:  "https://www.youtube.com/watch?v=HEAn4FqXFY4",
		},
		{
			name:  "11 char video ID",
			input: "HEAn4FqXFY4",
			want:  "HEAn4FqXFY4",
		},
		{
			name:  "search query fallback",
			input: "never gonna give you up",
			want:  "ytsearch1:never gonna give you up",
		},
		{
			name:  "explicit ytsearch",
			input: "ytsearch:some song",
			want:  "ytsearch:some song",
		},
		{
			name:  "ytsearch with count",
			input: "ytsearch5:some song",
			want:  "ytsearch5:some song",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  "",
		},
		{
			name:  "http URL",
			input: "http://www.youtube.com/watch?v=HEAn4FqXFY4",
			want:  "http://www.youtube.com/watch?v=HEAn4FqXFY4",
		},
		{
			name:  "youtube playlist URL",
			input: "https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf",
			want:  "https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf",
		},
		{
			name:  "non-youtube URL passes through",
			input: "https://soundcloud.com/some-track",
			want:  "https://soundcloud.com/some-track",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractYouTubeURL(tt.input)
			if got != tt.want {
				t.Errorf("extractYouTubeURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestOpusPacketSamples(t *testing.T) {
	tests := []struct {
		name string
		pkt  []byte
		want uint64
	}{
		{
			name: "empty packet",
			pkt:  []byte{},
			want: 0,
		},
		{
			name: "SILK 10ms mono single frame (config=0, code=0)",
			// TOC byte = 0x00 → config=0 (10ms SILK NB), s=0, c=0 → 1 frame × 10ms = 480 samples
			pkt:  make([]byte, 50),
			want: 480,
		},
		{
			name: "SILK 20ms mono single frame (config=1, code=0)",
			// TOC byte = 0x08 → config=1 (20ms SILK NB), s=0, c=0 → 1 frame × 20ms = 960 samples
			pkt:  append([]byte{0x08}, make([]byte, 49)...),
			want: 960,
		},
		{
			name: "CELT 10ms stereo single frame (config=16, code=0)",
			// TOC byte = 0x80 → config=16, code=0 → 1 frame × 2500us → 120 samples
			pkt:  append([]byte{0x80}, make([]byte, 49)...),
			want: 120,
		},
		{
			name: "Hybrid 10ms single frame (config=12, code=0)",
			// TOC byte = 0x60 → config=12, code=0 → 1 frame × 10ms → 480 samples
			pkt:  append([]byte{0x60}, make([]byte, 49)...),
			want: 480,
		},
		{
			name: "two frames code=1",
			// TOC: config=0 (10ms), code=1 → 2 frames × 10ms = 960 samples
			pkt:  append([]byte{0x01}, make([]byte, 49)...),
			want: 960,
		},
		{
			name: "arbitrary frames code=3",
			// TOC: config=0 (10ms), code=3 → read pkt[1] & 0x3F for frame count
			// pkt[1] = 0x05 → 5 frames × 10ms = 2400 samples
			pkt:  append([]byte{0x03, 0x05}, make([]byte, 48)...),
			want: 2400,
		},
		{
			name: "code=3 but packet too short",
			pkt:  []byte{0x03},
			want: 0,
		},
		{
			name: "code=3 zero frames",
			pkt:  []byte{0x03, 0x00, 0x00},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := opusPacketSamples(tt.pkt)
			if got != tt.want {
				t.Errorf("opusPacketSamples(%v) = %d, want %d", tt.pkt, got, tt.want)
			}
		})
	}
}
