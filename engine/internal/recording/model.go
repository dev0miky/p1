package recording

import (
	"errors"
	"time"
)

type Recording struct {
	ID             int64
	TenantID       int64
	CallUUID       string
	CampaignID     *int64
	LeadID         *int64
	FileKey        string
	SHA256         string
	SizeBytes      int64
	DurationMS     *int
	RetentionUntil time.Time
	CreatedAt      time.Time
}

var ErrNotFound = errors.New("recording not found")
