package api_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"p1/engine/internal/db"
	"p1/engine/internal/tenant"
)

func TestSSEStreamsTenantEvents(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "rt", Name: "x", SIPDomain: "rt.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")

	srv := httptest.NewServer(s.router)
	defer srv.Close()

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(reqCtx, "GET", srv.URL+"/tenant/events/?token="+tok, http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type: %s", ct)
	}

	reader := bufio.NewReader(resp.Body)
	// First frame is the hello.
	frame, err := readSSEFrame(reader)
	if err != nil {
		t.Fatalf("hello: %v", err)
	}
	if !strings.Contains(frame, "event: hello") {
		t.Fatalf("expected hello, got %q", frame)
	}

	// Trigger an insert into call_events — that fires the notify trigger.
	if err := db.WithCtx(ctx, s.repo.Pool(), db.Ctx{Role: "super_admin", TenantID: tn.ID}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO campaigns (tenant_id, name, mode) VALUES ($1, 'c', 'broadcast') RETURNING id
		`, tn.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO call_state (uuid, tenant_id, state, dialed_number, version)
			VALUES ('33333333-3333-3333-3333-333333333333', $1, 'queued', '+15550000001', 1)
		`, tn.ID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO call_events (call_uuid, tenant_id, from_state, to_state, reason)
			VALUES ('33333333-3333-3333-3333-333333333333', $1, NULL, 'queued', 'created')
		`, tn.ID)
		return err
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Next non-keepalive frame should be the call.event.
	deadline := time.Now().Add(4 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		frame, err := readSSEFrame(reader)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if strings.HasPrefix(frame, ":") {
			continue // keepalive comment
		}
		got = frame
		break
	}
	if !strings.Contains(got, "event: call.event") {
		t.Fatalf("expected call.event, got %q", got)
	}
	if !strings.Contains(got, "\"to_state\": \"queued\"") && !strings.Contains(got, "\"to_state\":\"queued\"") {
		t.Fatalf("payload missing to_state: %q", got)
	}
}

func TestSSECrossTenantFiltered(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tA, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "rta", Name: "A", SIPDomain: "rta.sip"})
	tB, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "rtb", Name: "B", SIPDomain: "rtb.sip"})

	// Subscribe as tenant A.
	tokA := s.tokenFor(t, 1, tA.ID, "tenant_owner")
	srv := httptest.NewServer(s.router)
	defer srv.Close()

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(reqCtx, "GET", srv.URL+"/tenant/events/?token="+tokA, http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	reader := bufio.NewReader(resp.Body)
	_, _ = readSSEFrame(reader) // hello

	// Emit an event for tenant B.
	if err := db.WithCtx(ctx, s.repo.Pool(), db.Ctx{Role: "super_admin", TenantID: tB.ID}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO call_state (uuid, tenant_id, state, dialed_number, version)
			VALUES ('44444444-4444-4444-4444-444444444444', $1, 'queued', '+15550000002', 1)
		`, tB.ID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO call_events (call_uuid, tenant_id, from_state, to_state, reason)
			VALUES ('44444444-4444-4444-4444-444444444444', $1, NULL, 'queued', 'created')
		`, tB.ID)
		return err
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Tenant A should NOT see tenant B's event. We allow up to 1s of frames
	// (keepalives only) before declaring success.
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		frame, err := readSSEFrame(reader)
		if err != nil {
			return // disconnect/timeout = no event delivered = pass
		}
		if strings.HasPrefix(frame, ":") {
			continue
		}
		if strings.Contains(frame, "event: call.event") {
			t.Fatalf("tenant A received tenant B's event: %q", frame)
		}
	}
}

// readSSEFrame reads up to a blank-line boundary.
func readSSEFrame(r *bufio.Reader) (string, error) {
	var b strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return b.String(), err
		}
		if line == "\n" || line == "\r\n" {
			return b.String(), nil
		}
		b.WriteString(line)
	}
}
