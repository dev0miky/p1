package sound

import (
	"errors"
	"time"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusReady   Status = "ready"
	StatusFailed  Status = "failed"
)

type Sound struct {
	ID          int64
	TenantID    int64
	Name        string
	Description *string
	FileKey     string
	MimeType    string
	SizeBytes   int64
	DurationMS  *int
	SHA256      *string
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

var ErrNotFound = errors.New("sound not found")
