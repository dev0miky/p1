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
	ID          int64
	TenantID    int64
	Name        string
	Description *string
	Type        Type
	Body        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

var ErrNotFound = errors.New("script not found")
