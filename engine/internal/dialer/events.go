package dialer

import (
	"strings"

	"p1/engine/internal/esl"
	"p1/engine/internal/fsm"
)

type StateAdvance struct {
	UUID          string
	To            fsm.State
	Reason        string
	HangupCause   string
	StampAnswered bool
	StampEnded    bool
}

func EventToTransition(e esl.Event) (StateAdvance, bool) {
	if e.UniqueID == "" {
		return StateAdvance{}, false
	}
	switch e.Name {
	case "CHANNEL_CREATE":
		return StateAdvance{
			UUID:   e.UniqueID,
			To:     fsm.StateRinging,
			Reason: "channel_create",
		}, true
	case "CHANNEL_ANSWER":
		return StateAdvance{
			UUID:          e.UniqueID,
			To:            fsm.StateAnswered,
			Reason:        "channel_answer",
			StampAnswered: true,
		}, true
	case "CHANNEL_HANGUP_COMPLETE":
		next := hangupToState(e.HangupCause)
		return StateAdvance{
			UUID:        e.UniqueID,
			To:          next,
			Reason:      "hangup:" + strings.ToLower(e.HangupCause),
			HangupCause: e.HangupCause,
			StampEnded:  true,
		}, true
	}
	return StateAdvance{}, false
}

func hangupToState(cause string) fsm.State {
	switch strings.ToUpper(cause) {
	case "NORMAL_CLEARING", "NORMAL_UNSPECIFIED", "ALLOTTED_TIMEOUT":
		return fsm.StateCompleted
	case "USER_BUSY":
		return fsm.StateBusy
	case "NO_ANSWER", "NO_USER_RESPONSE", "ORIGINATOR_CANCEL", "RECOVERY_ON_TIMER_EXPIRE":
		return fsm.StateNoAnswer
	case "":
		return fsm.StateCompleted
	default:
		return fsm.StateFailed
	}
}
