package voicechat

import (
	"github.com/hikari-work/userbot/modules/manager"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "JoinVoiceChat",
		Description: "Join the Voice Chat of the group/channel",
		Commands:    []string{"joinvc"},
		OnlyOut:     true,
		Handler:     JoinVCHandler,
		Help:        joinVCHelp,
	})

	manager.Register(&manager.Module{
		Name:        "LeaveVoiceChat",
		Description: "Leave the Voice Chat of the group/channel",
		Commands:    []string{"leavevc"},
		OnlyOut:     true,
		Handler:     LeaveVCHandler,
		Help:        leaveVCHelp,
	})

	manager.Register(&manager.Module{
		Name:        "PlayYouTube",
		Description: "Play audio from YouTube in the Voice Chat",
		Commands:    []string{"play"},
		OnlyOut:     true,
		Handler:     PlayHandler,
		Help:        playHelp,
	})

	manager.Register(&manager.Module{
		Name:        "PauseAudio",
		Description: "Pause current audio playback in Voice Chat",
		Commands:    []string{"pause"},
		OnlyOut:     true,
		Handler:     PauseHandler,
		Help:        pauseHelp,
	})

	manager.Register(&manager.Module{
		Name:        "ResumeAudio",
		Description: "Resume current audio playback in Voice Chat",
		Commands:    []string{"resume"},
		OnlyOut:     true,
		Handler:     ResumeHandler,
		Help:        resumeHelp,
	})

	manager.Register(&manager.Module{
		Name:        "StopAudio",
		Description: "Stop current audio playback in Voice Chat",
		Commands:    []string{"stop"},
		OnlyOut:     true,
		Handler:     StopHandler,
		Help:        stopHelp,
	})

	manager.Register(&manager.Module{
		Name:        "SkipAudio",
		Description: "Skip the currently playing song in the Voice Chat",
		Commands:    []string{"skip"},
		OnlyOut:     true,
		Handler:     SkipHandler,
		Help:        skipHelp,
	})

	manager.Register(&manager.Module{
		Name:        "PlaylistAudio",
		Description: "Show the current playlist queue and control it",
		Commands:    []string{"playlist", "pl"},
		OnlyOut:     true,
		Handler:     PlaylistHandler,
		Help:        playlistHelp,

		CallbackPrefix:  "vcpl",
		CallbackHandler: PlaylistCallbackHandler,
		InlineHandler:   PlaylistInlineHandler,
	})
}

func joinVCHelp() string {
	return "Format \n<code>.joinvc</code>\n<code>Contoh : .joinvc</code>"
}

func leaveVCHelp() string {
	return "Format \n<code>.leavevc</code>\n<code>Contoh : .leavevc</code>"
}

func playHelp() string {
	return "Format \n<code>.play &lt;link_youtube/judul_lagu&gt;</code>\n<code>Contoh : .play https://youtu.be/xxx</code>"
}

func pauseHelp() string {
	return "Format \n<code>.pause</code>\n<code>Contoh : .pause</code>"
}

func resumeHelp() string {
	return "Format \n<code>.resume</code>\n<code>Contoh : .resume</code>"
}

func stopHelp() string {
	return "Format \n<code>.stop</code>\n<code>Contoh : .stop</code>"
}

func skipHelp() string {
	return "Format \n<code>.skip</code>\n<code>Contoh : .skip</code>"
}

func playlistHelp() string {
	return "Format \n<code>.playlist</code>\n<code>Contoh : .playlist</code>"
}
