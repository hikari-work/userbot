package voicechat

import (
	"strconv"
	"strings"
)

func extractSDPParams(sdp string) (ufrag, pwd, fingerprint, hash string) {
	for _, line := range strings.Split(sdp, "\r\n") {
		switch {
		case strings.HasPrefix(line, "a=ice-ufrag:"):
			ufrag = strings.TrimPrefix(line, "a=ice-ufrag:")
		case strings.HasPrefix(line, "a=ice-pwd:"):
			pwd = strings.TrimPrefix(line, "a=ice-pwd:")
		case strings.HasPrefix(line, "a=fingerprint:"):
			if parts := strings.SplitN(strings.TrimPrefix(line, "a=fingerprint:"), " ", 2); len(parts) == 2 {
				hash, fingerprint = parts[0], parts[1]
			}
		}
	}
	return ufrag, pwd, fingerprint, hash
}

func extractSSRC(sdp string) uint32 {
	for _, line := range strings.Split(sdp, "\r\n") {
		if strings.HasPrefix(line, "a=ssrc:") {
			parts := strings.Split(strings.TrimPrefix(line, "a=ssrc:"), " ")
			if len(parts) > 0 {
				if ssrcVal, err := strconv.ParseUint(parts[0], 10, 32); err == nil {
					return uint32(ssrcVal)
				}
			}
		}
	}
	return 0
}

func buildAnswerSDP(resp GroupJoinResponse) string {
	t := resp.Transport

	var candidates []GroupCandidate
	for _, c := range t.Candidates {
		if !strings.Contains(c.IP, ":") {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) == 0 {
		candidates = t.Candidates
	}

	port := "10000"
	ip := "0.0.0.0"
	ipType := "IP4"
	if len(candidates) > 0 {
		port = candidates[0].Port
		ip = candidates[0].IP
		if strings.Contains(ip, ":") {
			ipType = "IP6"
		} else {
			ipType = "IP4"
		}
	}

	var payloads []string
	if resp.Audio != nil {
		for _, pt := range resp.Audio.PayloadTypes {
			payloads = append(payloads, strconv.Itoa(pt.ID))
		}
	}
	if len(payloads) == 0 {
		payloads = []string{"111"}
	}

	lines := []string{
		"v=0",
		"o=- 1 2 IN IP4 0.0.0.0",
		"s=-",
		"t=0 0",
		"a=group:BUNDLE 0",
		"a=ice-lite",
		"m=audio " + port + " RTP/SAVPF " + strings.Join(payloads, " "),
		"c=IN " + ipType + " " + ip,
		"a=mid:0",
		"a=ice-ufrag:" + t.Ufrag,
		"a=ice-pwd:" + t.Pwd,
	}
	if len(t.Fingerprints) > 0 {
		lines = append(lines, "a=fingerprint:"+t.Fingerprints[0].Hash+" "+t.Fingerprints[0].Fingerprint)
	}
	lines = append(lines, "a=setup:active")

	for _, c := range candidates {
		lines = append(lines, strings.Join([]string{
			"a=candidate:" + c.Foundation, c.Component, c.Protocol, c.Priority,
			c.IP, c.Port, "typ", c.Type, "generation", c.Generation,
		}, " "))
	}

	if resp.Audio != nil {
		for _, pt := range resp.Audio.PayloadTypes {
			rtpmap := "a=rtpmap:" + strconv.Itoa(pt.ID) + " " + pt.Name + "/" + strconv.Itoa(pt.Clockrate)
			if pt.Channels > 1 {
				rtpmap += "/" + strconv.Itoa(pt.Channels)
			}
			lines = append(lines, rtpmap)
			if pt.Name == "opus" {
				lines = append(lines, "a=fmtp:"+strconv.Itoa(pt.ID)+" minptime=10;useinbandfec=1")
			}
		}
		for _, ext := range resp.Audio.HDRExts {
			lines = append(lines, "a=extmap:"+strconv.Itoa(ext.ID)+" "+ext.URI)
		}
	}

	return strings.Join(lines, "\r\n") + "\r\n"
}
