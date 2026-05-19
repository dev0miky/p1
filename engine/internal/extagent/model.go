package extagent

import (
	"errors"
	"time"
)

type Agent struct {
	ID          int64
	TenantID    int64
	Name        string
	Description *string
	DialString  string
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

var ErrNotFound = errors.New("external agent not found")
