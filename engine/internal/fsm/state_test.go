package fsm

import "testing"

func TestTerminalStatesCannotTransition(t *testing.T) {
	terminals := []State{StateCompleted, StateFailed, StateNoAnswer, StateBusy, StateVoicemail, StateOptOut}
	for _, s := range terminals {
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", s)
		}
		for _, next := range AllStates() {
			if s.CanTransitionTo(next) {
				t.Errorf("%s should not transition to %s (terminal)", s, next)
			}
		}
	}
}

func TestHappyPathPress1Flow(t *testing.T) {
	path := []State{
		StateQueued, StateOriginating, StateRinging, StateAnswered, StateAMDRunning,
		StateHuman, StatePlayingMsg, StateWaitDTMF, StatePress1, StateBridging, StateBridged, StateCompleted,
	}
	for i := 0; i < len(path)-1; i++ {
		if !path[i].CanTransitionTo(path[i+1]) {
			t.Errorf("transition %s → %s must be valid in happy press-1 path", path[i], path[i+1])
		}
	}
}

func TestBroadcastFlow(t *testing.T) {
	path := []State{StateQueued, StateOriginating, StateAnswered, StatePlayingMsg, StateCompleted}
	for i := 0; i < len(path)-1; i++ {
		if !path[i].CanTransitionTo(path[i+1]) {
			t.Errorf("transition %s → %s must be valid in broadcast path", path[i], path[i+1])
		}
	}
}

func TestVoicemailDropFlow(t *testing.T) {
	path := []State{StateQueued, StateOriginating, StateAnswered, StateAMDRunning, StateMachine, StateVoicemail}
	for i := 0; i < len(path)-1; i++ {
		if !path[i].CanTransitionTo(path[i+1]) {
			t.Errorf("transition %s → %s must be valid in vm-drop path", path[i], path[i+1])
		}
	}
}

func TestOptOutFlow(t *testing.T) {
	path := []State{StateQueued, StateOriginating, StateAnswered, StatePlayingMsg, StateWaitDTMF, StateOptOut}
	for i := 0; i < len(path)-1; i++ {
		if !path[i].CanTransitionTo(path[i+1]) {
			t.Errorf("transition %s → %s must be valid in opt-out path", path[i], path[i+1])
		}
	}
}

func TestInvalidTransitionsRejected(t *testing.T) {
	cases := []struct {
		from, to State
	}{
		{StateQueued, StateAnswered},
		{StateQueued, StateBridged},
		{StateOriginating, StateBridged},
		{StateRinging, StatePlayingMsg},
		{StateWaitDTMF, StateBridged},
	}
	for _, c := range cases {
		if c.from.CanTransitionTo(c.to) {
			t.Errorf("transition %s → %s should be rejected", c.from, c.to)
		}
	}
}

func TestPress1ShortcutFromAnswered(t *testing.T) {
	path := []State{StateAnswered, StatePress1, StateBridging, StateBridged, StateCompleted}
	for i := 0; i < len(path)-1; i++ {
		if !path[i].CanTransitionTo(path[i+1]) {
			t.Errorf("lua-driven press-1: %s → %s must be valid (skip amd/playing/waitdtmf)", path[i], path[i+1])
		}
	}
}

func TestOptOutShortcutFromAnswered(t *testing.T) {
	if !StateAnswered.CanTransitionTo(StateOptOut) {
		t.Error("lua-driven opt-out: Answered → OptOut must be valid")
	}
}

func TestEveryNonTerminalStateCanReachAFailureState(t *testing.T) {
	for _, s := range AllStates() {
		if s.IsTerminal() {
			continue
		}
		reachesFailure := false
		for _, term := range []State{StateFailed, StateCompleted, StateNoAnswer, StateBusy, StateVoicemail, StateOptOut} {
			if s.CanTransitionTo(term) {
				reachesFailure = true
				break
			}
		}
		if !reachesFailure {
			t.Errorf("state %s has no direct path to a terminal state — orphan risk", s)
		}
	}
}
