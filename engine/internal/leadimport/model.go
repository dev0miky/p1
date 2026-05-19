package leadimport

import (
	"encoding/json"
	"errors"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusAborted   Status = "aborted"
)

type Job struct {
	ID            int64
	TenantID      int64
	ListID        *int64
	Status        Status
	CSVFilename   string
	FileKey       string
	ColumnMap     json.RawMessage
	TotalRows     int
	ProcessedRows int
	ErrorRows     int
	LastError     *string
	StartedAt     *time.Time
	FinishedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

var ErrNotFound = errors.New("import job not found")
