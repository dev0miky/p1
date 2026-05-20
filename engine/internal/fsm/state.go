package fsm

type State string

const (
	StateQueued      State = "queued"
	StateOriginating State = "originating"
	StateRinging     State = "ringing"
	StateAnswered    State = "answered"
	StateAMDRunning  State = "amd_running"
	StateHuman       State = "human"
	StateMachine     State = "machine"
	StateNoInput     State = "no_input"
	StatePlayingMsg  State = "playing_msg"
	StateWaitDTMF    State = "wait_dtmf"
	StatePress1      State = "press1"
	StateBridging    State = "bridging"
	StateBridged     State = "bridged"

	StateCompleted State = "completed"
	StateFailed    State = "failed"
	StateNoAnswer  State = "no_answer"
	StateBusy      State = "busy"
	StateVoicemail State = "voicemail"
	StateOptOut    State = "opt_out"
)

var terminalStates = map[State]bool{
	StateCompleted: true,
	StateFailed:    true,
	StateNoAnswer:  true,
	StateBusy:      true,
	StateVoicemail: true,
	StateOptOut:    true,
}

func (s State) IsTerminal() bool {
	return terminalStates[s]
}

var validTransitions = map[State]map[State]bool{
	StateQueued: {
		StateOriginating: true,
		StateFailed:      true,
	},
	StateOriginating: {
		StateRinging:  true,
		StateAnswered: true,
		StateBusy:     true,
		StateNoAnswer: true,
		StateFailed:   true,
	},
	StateRinging: {
		StateAnswered: true,
		StateBusy:     true,
		StateNoAnswer: true,
		StateFailed:   true,
	},
	StateAnswered: {
		StateAMDRunning: true,
		StatePlayingMsg: true,
		StatePress1:     true,
		StateOptOut:     true,
		StateVoicemail:  true,
		StateCompleted:  true,
		StateFailed:     true,
	},
	StateAMDRunning: {
		StateHuman:     true,
		StateMachine:   true,
		StateNoInput:   true,
		StateCompleted: true,
		StateFailed:    true,
	},
	StateHuman: {
		StatePlayingMsg: true,
		StateCompleted:  true,
		StateFailed:     true,
	},
	StateMachine: {
		StateVoicemail: true,
		StateCompleted: true,
		StateFailed:    true,
	},
	StateNoInput: {
		StateCompleted: true,
	},
	StatePlayingMsg: {
		StateWaitDTMF:  true,
		StateVoicemail: true,
		StateCompleted: true,
		StateFailed:    true,
	},
	StateWaitDTMF: {
		StatePress1:    true,
		StateOptOut:    true,
		StateNoInput:   true,
		StateCompleted: true,
		StateFailed:    true,
	},
	StatePress1: {
		StateBridging:  true,
		StateCompleted: true,
		StateFailed:    true,
	},
	StateBridging: {
		StateBridged:   true,
		StateCompleted: true,
		StateFailed:    true,
	},
	StateBridged: {
		StateCompleted: true,
		StateFailed:    true,
	},
}

func (s State) CanTransitionTo(next State) bool {
	if s.IsTerminal() {
		return false
	}
	allowed, ok := validTransitions[s]
	if !ok {
		return false
	}
	return allowed[next]
}

func AllStates() []State {
	return []State{
		StateQueued, StateOriginating, StateRinging, StateAnswered,
		StateAMDRunning, StateHuman, StateMachine, StateNoInput,
		StatePlayingMsg, StateWaitDTMF, StatePress1, StateBridging, StateBridged,
		StateCompleted, StateFailed, StateNoAnswer, StateBusy, StateVoicemail, StateOptOut,
	}
}
