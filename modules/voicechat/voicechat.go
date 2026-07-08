package voicechat

import "github.com/hikari-work/userbot/modules/manager"

func init() {
	manager.Register(&manager.Module{
		Name:        "JoinVoiceChat",
		Description: "Join the Voice Chat of the group/channel",
		Commands:    []string{"joinvc"},
		OnlyOut:     true,
		Handler:     JoinVCHandler,
	})

	manager.Register(&manager.Module{
		Name:        "LeaveVoiceChat",
		Description: "Leave the Voice Chat of the group/channel",
		Commands:    []string{"leavevc"},
		OnlyOut:     true,
		Handler:     LeaveVCHandler,
	})

	manager.Register(&manager.Module{
		Name:        "PlayYouTube",
		Description: "Play audio from YouTube in the Voice Chat",
		Commands:    []string{"play"},
		OnlyOut:     true,
		Handler:     PlayHandler,
	})

	manager.Register(&manager.Module{
		Name:        "PauseAudio",
		Description: "Pause current audio playback in Voice Chat",
		Commands:    []string{"pause"},
		OnlyOut:     true,
		Handler:     PauseHandler,
	})

	manager.Register(&manager.Module{
		Name:        "ResumeAudio",
		Description: "Resume current audio playback in Voice Chat",
		Commands:    []string{"resume"},
		OnlyOut:     true,
		Handler:     ResumeHandler,
	})

	manager.Register(&manager.Module{
		Name:        "StopAudio",
		Description: "Stop current audio playback in Voice Chat",
		Commands:    []string{"stop"},
		OnlyOut:     true,
		Handler:     StopHandler,
	})

	manager.Register(&manager.Module{
		Name:        "SkipAudio",
		Description: "Skip the currently playing song in the Voice Chat",
		Commands:    []string{"skip"},
		OnlyOut:     true,
		Handler:     SkipHandler,
	})

	manager.Register(&manager.Module{
		Name:        "PlaylistAudio",
		Description: "Show the current playlist queue and control it",
		Commands:    []string{"playlist", "pl"},
		OnlyOut:     true,
		Handler:     PlaylistHandler,

		CallbackPrefix:  "vcpl",
		CallbackHandler: PlaylistCallbackHandler,
		InlineHandler:   PlaylistInlineHandler,
	})
}
