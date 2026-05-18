package tenant

import "time"

type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusDeleted   Status = "deleted"
)

type Tenant struct {
	ID        int64
	Slug      string
	Name      string
	SIPDomain string
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

type User struct {
	ID           int64
	TenantID     *int64
	Email        string
	Role         string
	PasswordHash string
	TOTPSecret   *string
	Status       string
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
