package callerid

import (
	"errors"
	"time"
)

type Attestation string

const (
	AttestationA    Attestation = "a"
	AttestationB    Attestation = "b"
	AttestationC    Attestation = "c"
	AttestationNone Attestation = "none"
)

func ValidAttestation(s string) bool {
	switch Attestation(s) {
	case AttestationA, AttestationB, AttestationC, AttestationNone:
		return true
	}
	return false
}

type CallerID struct {
	ID          int64
	TenantID    int64
	Name        string
	E164Number  string
	DisplayName *string
	Attestation Attestation
	Description *string
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

var ErrNotFound = errors.New("caller_id not found")
