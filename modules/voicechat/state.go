package voicechat

import "sync"

var (
	vcStates   = make(map[int64]*State)
	vcStatesMu sync.Mutex
)

func getVCState(chatID int64) *State {
	vcStatesMu.Lock()
	defer vcStatesMu.Unlock()
	state, exists := vcStates[chatID]
	if !exists {
		state = &State{}
		vcStates[chatID] = state
	}
	return state
}
