package dialer

import "math"

type PacingInput struct {
	Mode            string
	DialRatio       float64
	MaxConcurrent   int
	InFlight        int
	AvailableAgents int
}

func DecideToDial(in PacingInput) int {
	switch in.Mode {
	case "broadcast":
		return broadcastPace(in)
	case "press1":
		return press1Pace(in)
	case "predictive":
		return predictivePace(in)
	case "preview":
		return previewPace(in)
	default:
		return 0
	}
}

func broadcastPace(in PacingInput) int {
	target := in.MaxConcurrent
	if target <= 0 {
		target = 50
	}
	need := target - in.InFlight
	if need < 0 {
		return 0
	}
	return need
}

func press1Pace(in PacingInput) int {
	if in.AvailableAgents <= 0 {
		return 0
	}
	target := int(math.Ceil(float64(in.AvailableAgents) * in.DialRatio))
	if in.MaxConcurrent > 0 && target > in.MaxConcurrent {
		target = in.MaxConcurrent
	}
	need := target - in.InFlight
	if need < 0 {
		return 0
	}
	return need
}

func predictivePace(in PacingInput) int {
	return press1Pace(in)
}

func previewPace(in PacingInput) int {
	if in.AvailableAgents <= 0 {
		return 0
	}
	need := in.AvailableAgents - in.InFlight
	if need < 0 {
		return 0
	}
	return need
}
