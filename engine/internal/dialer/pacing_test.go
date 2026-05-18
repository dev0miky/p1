package dialer

import "testing"

func TestBroadcastPacing(t *testing.T) {
	cases := []struct {
		in   PacingInput
		want int
	}{
		{PacingInput{Mode: "broadcast", MaxConcurrent: 100, InFlight: 0}, 100},
		{PacingInput{Mode: "broadcast", MaxConcurrent: 100, InFlight: 60}, 40},
		{PacingInput{Mode: "broadcast", MaxConcurrent: 100, InFlight: 150}, 0},
		{PacingInput{Mode: "broadcast", MaxConcurrent: 0, InFlight: 0}, 50},
	}
	for _, c := range cases {
		if got := DecideToDial(c.in); got != c.want {
			t.Errorf("%+v: got %d, want %d", c.in, got, c.want)
		}
	}
}

func TestPress1Pacing(t *testing.T) {
	cases := []struct {
		in   PacingInput
		want int
	}{
		{PacingInput{Mode: "press1", DialRatio: 1.0, AvailableAgents: 5, InFlight: 0}, 5},
		{PacingInput{Mode: "press1", DialRatio: 1.2, AvailableAgents: 5, InFlight: 0}, 6},
		{PacingInput{Mode: "press1", DialRatio: 2.0, AvailableAgents: 10, InFlight: 15}, 5},
		{PacingInput{Mode: "press1", DialRatio: 2.0, AvailableAgents: 10, InFlight: 20}, 0},
		{PacingInput{Mode: "press1", DialRatio: 1.0, AvailableAgents: 0, InFlight: 0}, 0},
		{PacingInput{Mode: "press1", DialRatio: 5.0, AvailableAgents: 10, MaxConcurrent: 20, InFlight: 0}, 20},
	}
	for _, c := range cases {
		if got := DecideToDial(c.in); got != c.want {
			t.Errorf("%+v: got %d, want %d", c.in, got, c.want)
		}
	}
}

func TestPreviewPacingAgentLed(t *testing.T) {
	in := PacingInput{Mode: "preview", AvailableAgents: 3, InFlight: 1}
	if got := DecideToDial(in); got != 2 {
		t.Errorf("preview: got %d, want 2", got)
	}
}

func TestUnknownModeReturnsZero(t *testing.T) {
	if got := DecideToDial(PacingInput{Mode: "telegram", AvailableAgents: 5}); got != 0 {
		t.Errorf("unknown mode should return 0, got %d", got)
	}
}
