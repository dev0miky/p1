package script

import (
	"errors"
	"time"
)

type Type string

const (
	TypePress1    Type = "press1"
	TypeBroadcast Type = "broadcast"
	TypeSurvey    Type = "survey"
	TypeCustom    Type = "custom"
)

func ValidType(s string) bool {
	switch Type(s) {
	case TypePress1, TypeBroadcast, TypeSurvey, TypeCustom:
		return true
	}
	return false
}

type Script struct {
	ID               int64
	TenantID         int64
	Name             string
	Description      *string
	Type             Type
	Body             string
	TransferTo       *string
	ExternalAgentID  *int64
	GreetingSoundID  *int64
	PreBridgeSoundID *int64
	BridgeDigit      string
	WaitTimeoutMS    int
	OptOutDigit      *string
	Tags             []string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func ValidDTMFDigit(s string) bool {
	if len(s) != 1 {
		return false
	}
	c := s[0]
	return (c >= '0' && c <= '9') || c == '*' || c == '#'
}

var ErrNotFound = errors.New("script not found")
