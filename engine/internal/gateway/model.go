package gateway

import (
	"errors"
	"time"
)

type Transport string

const (
	TransportUDP Transport = "udp"
	TransportTCP Transport = "tcp"
	TransportTLS Transport = "tls"
)

func ValidTransport(s string) bool {
	switch Transport(s) {
	case TransportUDP, TransportTCP, TransportTLS:
		return true
	}
	return false
}

type Gateway struct {
	ID               int64
	Name             string
	Description      *string
	Proxy            string
	Register         bool
	Username         *string
	Password         *string // plaintext in-memory only; never serialized to API
	HasPassword      bool    // set on reads so the API/UI can show "password set"
	Realm            *string
	FromUser         *string
	FromDomain       *string
	Transport        Transport
	ExpireSeconds    int
	RetrySeconds     int
	CallerIDInFrom   bool
	ExtraParams      map[string]string
	Enabled          bool
	IsActive         bool
	RegisterStatus   string
	RegisterStatusAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

var ErrNotFound = errors.New("gateway not found")
