package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"p1/engine/internal/tenant"
)

func TestDNCAddAndList(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "da", Name: "DA", SIPDomain: "da.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	reason := "customer requested"
	w := s.do(t, "POST", "/tenant/dnc/", tok, map[string]any{
		"phone_e164": "+15551112222",
		"reason":     reason,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	w = s.do(t, "GET", "/tenant/dnc/", tok, nil)
	if w.Code != 200 {
		t.Fatalf("list: %d", w.Code)
	}
	var resp struct {
		Entries []map[string]any `json:"entries"`
		Total   int              `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 1 || resp.Entries[0]["phone_e164"] != "+15551112222" {
		t.Fatalf("unexpected: %v", resp)
	}
}

func TestDNCInvalidPhoneRejected(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "dip", Name: "x", SIPDomain: "dip.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/dnc/", tok, map[string]any{"phone_e164": "5551234567"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestDNCDuplicateReturns409(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "ddup", Name: "x", SIPDomain: "ddup.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	body := map[string]any{"phone_e164": "+15553334444"}
	if w := s.do(t, "POST", "/tenant/dnc/", tok, body); w.Code != http.StatusCreated {
		t.Fatalf("first: %d", w.Code)
	}
	if w := s.do(t, "POST", "/tenant/dnc/", tok, body); w.Code != http.StatusConflict {
		t.Fatalf("dup: want 409, got %d", w.Code)
	}
}

func TestDNCRemove(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "drm", Name: "x", SIPDomain: "drm.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	s.do(t, "POST", "/tenant/dnc/", tok, map[string]any{"phone_e164": "+15554445555"})

	w := s.do(t, "DELETE", "/tenant/dnc/+15554445555", tok, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d body=%s", w.Code, w.Body.String())
	}

	w = s.do(t, "DELETE", "/tenant/dnc/+15554445555", tok, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("delete again: want 404, got %d", w.Code)
	}
}

func TestDNCCheck(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "dchk", Name: "x", SIPDomain: "dchk.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	s.do(t, "POST", "/tenant/dnc/", tok, map[string]any{"phone_e164": "+15556667777"})

	w := s.do(t, "GET", "/tenant/dnc/check?phone=%2B15556667777", tok, nil)
	if w.Code != 200 {
		t.Fatalf("check: %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["blocked"] != true || resp["scope"] != "internal" {
		t.Fatalf("expected blocked internal, got %v", resp)
	}

	w = s.do(t, "GET", "/tenant/dnc/check?phone=%2B15558889999", tok, nil)
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["blocked"] != false {
		t.Fatalf("expected not blocked, got %v", resp)
	}
}

func TestDNCCrossTenantIsolation(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tA, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "dxa", Name: "A", SIPDomain: "dxa.sip"})
	tB, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "dxb", Name: "B", SIPDomain: "dxb.sip"})

	tokA := s.tokenFor(t, 1, tA.ID, "tenant_owner")
	tokB := s.tokenFor(t, 2, tB.ID, "tenant_owner")

	s.do(t, "POST", "/tenant/dnc/", tokA, map[string]any{"phone_e164": "+15550010001"})
	s.do(t, "POST", "/tenant/dnc/", tokB, map[string]any{"phone_e164": "+15550010002"})

	w := s.do(t, "GET", "/tenant/dnc/", tokA, nil)
	var resp struct {
		Entries []map[string]any `json:"entries"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Entries) != 1 || resp.Entries[0]["phone_e164"] != "+15550010001" {
		t.Fatalf("tenant A should see only own DNC, got %v", resp.Entries)
	}

	w = s.do(t, "GET", "/tenant/dnc/check?phone=%2B15550010002", tokA, nil)
	var chk map[string]any
	json.Unmarshal(w.Body.Bytes(), &chk)
	if chk["blocked"] != false {
		t.Fatalf("tenant A check on B's number: should not be blocked from A's view, got %v", chk)
	}
}

func TestDNCCampaignManagerForbidden(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "dcm", Name: "x", SIPDomain: "dcm.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "campaign_manager")
	w := s.do(t, "POST", "/tenant/dnc/", tok, map[string]any{"phone_e164": "+15551112222"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("campaign_manager should not have ManageDNC: want 403, got %d", w.Code)
	}
}
