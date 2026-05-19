package dialer

import (
	"testing"

	"p1/engine/internal/esl"
	"p1/engine/internal/fsm"
	"p1/engine/internal/lead"
)

func TestEventToTransitionAnswer(t *testing.T) {
	e := esl.ParseEvent(map[string]string{
		"Event-Name": "CHANNEL_ANSWER",
		"Unique-ID":  "abc-123",
	})
	got, ok := EventToTransition(e)
	if !ok {
		t.Fatal("answer event should map")
	}
	if got.To != fsm.StateAnswered {
		t.Errorf("to: %s", got.To)
	}
	if !got.StampAnswered {
		t.Error("should stamp answered_at")
	}
}

func TestEventToTransitionRinging(t *testing.T) {
	e := esl.ParseEvent(map[string]string{
		"Event-Name": "CHANNEL_CREATE",
		"Unique-ID":  "abc-123",
	})
	got, ok := EventToTransition(e)
	if !ok || got.To != fsm.StateRinging {
		t.Errorf("expected Ringing, got %+v ok=%v", got, ok)
	}
}

func TestEventToTransitionHangupCauses(t *testing.T) {
	cases := []struct {
		cause string
		want  fsm.State
	}{
		{"NORMAL_CLEARING", fsm.StateCompleted},
		{"USER_BUSY", fsm.StateBusy},
		{"NO_ANSWER", fsm.StateNoAnswer},
		{"NO_USER_RESPONSE", fsm.StateNoAnswer},
		{"ORIGINATOR_CANCEL", fsm.StateNoAnswer},
		{"DESTINATION_OUT_OF_ORDER", fsm.StateFailed},
		{"", fsm.StateCompleted},
	}
	for _, c := range cases {
		e := esl.ParseEvent(map[string]string{
			"Event-Name":   "CHANNEL_HANGUP_COMPLETE",
			"Unique-ID":    "uuid",
			"Hangup-Cause": c.cause,
		})
		got, ok := EventToTransition(e)
		if !ok {
			t.Errorf("cause %q: not mapped", c.cause)
			continue
		}
		if got.To != c.want {
			t.Errorf("cause %q: to=%s, want %s", c.cause, got.To, c.want)
		}
		if !got.StampEnded {
			t.Errorf("cause %q: should stamp ended_at", c.cause)
		}
	}
}

func TestEventWithoutUUIDIsIgnored(t *testing.T) {
	e := esl.ParseEvent(map[string]string{"Event-Name": "CHANNEL_ANSWER"})
	if _, ok := EventToTransition(e); ok {
		t.Error("event without UUID should not produce a transition")
	}
}

func TestUnknownEventIgnored(t *testing.T) {
	e := esl.ParseEvent(map[string]string{
		"Event-Name": "HEARTBEAT",
		"Unique-ID":  "abc",
	})
	if _, ok := EventToTransition(e); ok {
		t.Error("HEARTBEAT should be ignored")
	}
}

func TestCounterDeltaForCoversAllOutcomes(t *testing.T) {
	cases := []struct {
		state fsm.State
		want  lead.CounterDelta
	}{
		{fsm.StateOriginating, lead.CounterDelta{NCalls: 1, SetCallTimes: true}},
		{fsm.StateRinging, lead.CounterDelta{NRinged: 1}},
		{fsm.StateAnswered, lead.CounterDelta{NAnswered: 1}},
		{fsm.StateBridging, lead.CounterDelta{NTransferred: 1}},
		{fsm.StateBridged, lead.CounterDelta{NTransferCompleted: 1}},
		{fsm.StateFailed, lead.CounterDelta{NError: 1}},
		{fsm.StateVoicemail, lead.CounterDelta{NVoicemail: 1}},
		{fsm.StateOptOut, lead.CounterDelta{NWentToDNC: 1}},
		{fsm.StateCompleted, lead.CounterDelta{}},
		{fsm.StateBusy, lead.CounterDelta{}},
		{fsm.StateNoAnswer, lead.CounterDelta{}},
		{fsm.StateQueued, lead.CounterDelta{}},
	}
	for _, c := range cases {
		got := counterDeltaFor(c.state)
		if got != c.want {
			t.Errorf("state %s: want %+v, got %+v", c.state, c.want, got)
		}
	}
}
