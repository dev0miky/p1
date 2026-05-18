package lead

import (
	"encoding/json"
	"regexp"
	"time"
)

type Status string

const (
	StatusNew         Status = "new"
	StatusQueued      Status = "queued"
	StatusInFlight    Status = "in_flight"
	StatusDone        Status = "done"
	StatusDNC         Status = "dnc"
	StatusMaxAttempts Status = "max_attempts"
	StatusFailed      Status = "failed"
	StatusOptOut      Status = "opt_out"
)

type Lead struct {
	ID              int64
	TenantID        int64
	ListID          *int64
	CampaignID      *int64
	PhoneE164       string
	DialDestination *string
	FirstName       *string
	LastName        *string
	Email           *string
	Timezone        *string
	StateCode       *string
	Status          Status
	Attempts        int
	LastAttemptAt   *time.Time
	NextEligibleAt  *time.Time
	CustomFields    json.RawMessage
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type List struct {
	ID        int64
	TenantID  int64
	Name      string
	Source    *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

var e164re = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

func ValidE164(s string) bool {
	return e164re.MatchString(s)
}
