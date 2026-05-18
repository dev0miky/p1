package campaign

import (
	"encoding/json"
	"time"
)

type Mode string

const (
	ModePress1     Mode = "press1"
	ModeBroadcast  Mode = "broadcast"
	ModePredictive Mode = "predictive"
	ModePreview    Mode = "preview"
)

type Status string

const (
	StatusPaused    Status = "paused"
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusArchived  Status = "archived"
)

type Campaign struct {
	ID            int64
	TenantID      int64
	Name          string
	Mode          Mode
	Status        Status
	DialRatio     float64
	MaxAbandonPct float64
	PromptAudio   *string
	TransferDest  *string
	CallerIDPool  json.RawMessage
	RetryPolicy   json.RawMessage
	CallingHours  json.RawMessage
	TZStrategy    string
	DNCListIDs    []int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func ValidMode(m string) bool {
	switch Mode(m) {
	case ModePress1, ModeBroadcast, ModePredictive, ModePreview:
		return true
	}
	return false
}

func ValidStatus(s string) bool {
	switch Status(s) {
	case StatusPaused, StatusActive, StatusCompleted, StatusArchived:
		return true
	}
	return false
}
