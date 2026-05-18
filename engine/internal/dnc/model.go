package dnc

import "time"

type Scope string

const (
	ScopeInternal Scope = "internal"
	ScopeFederal  Scope = "federal"
	ScopeState    Scope = "state"
	ScopeWireless Scope = "wireless"
	ScopeRND      Scope = "rnd"
	ScopeCustom   Scope = "custom"
)

type Entry struct {
	ID        int64
	TenantID  *int64
	Scope     Scope
	StateCode *string
	PhoneE164 string
	Source    *string
	Reason    *string
	AddedAt   time.Time
	ExpiresAt *time.Time
}

type OptOut struct {
	ID          int64
	TenantID    int64
	CampaignID  *int64
	PhoneE164   string
	Channel     string
	EvidenceRef *string
	CapturedAt  time.Time
}
