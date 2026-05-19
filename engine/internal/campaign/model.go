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
	ID             int64
	TenantID       int64
	Name           string
	Mode           Mode
	Status         Status
	DialRatio      float64
	MaxAbandonPct  float64
	PromptAudio    *string
	TransferDest   *string
	CallerIDPool   json.RawMessage
	RetryPolicy    json.RawMessage
	CallingHours   json.RawMessage
	TZStrategy     string
	DNCListIDs     []int64
	RunNo          int
	CallConstraint string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CallConstraint string

const (
	ConstraintNone                    CallConstraint = "no_constraint"
	ConstraintOnlyAnswered            CallConstraint = "only_answered"
	ConstraintOnlyHumanAnswered       CallConstraint = "only_human_answered"
	ConstraintOnlyMachineAnswered     CallConstraint = "only_machine_answered"
	ConstraintOnlyFailedTransfers     CallConstraint = "only_failed_transfers"
	ConstraintOnlyTransfers           CallConstraint = "only_transfers"
	ConstraintOnlySuccessfulTransfers CallConstraint = "only_successful_transfers"
	ConstraintOnlyErrors              CallConstraint = "only_errors"
	ConstraintSkipAnswered            CallConstraint = "skip_answered"
	ConstraintSkipHumanAnswered       CallConstraint = "skip_human_answered"
	ConstraintSkipMachineAnswered     CallConstraint = "skip_machine_answered"
	ConstraintSkipSuccessfulTransfers CallConstraint = "skip_successful_transfers"
	ConstraintSkipErrors              CallConstraint = "skip_errors"
)

func ValidCallConstraint(s string) bool {
	switch CallConstraint(s) {
	case ConstraintNone, ConstraintOnlyAnswered, ConstraintOnlyHumanAnswered,
		ConstraintOnlyMachineAnswered, ConstraintOnlyFailedTransfers, ConstraintOnlyTransfers,
		ConstraintOnlySuccessfulTransfers, ConstraintOnlyErrors,
		ConstraintSkipAnswered, ConstraintSkipHumanAnswered, ConstraintSkipMachineAnswered,
		ConstraintSkipSuccessfulTransfers, ConstraintSkipErrors:
		return true
	}
	return false
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
