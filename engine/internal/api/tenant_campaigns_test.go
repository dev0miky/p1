package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"p1/engine/internal/tenant"
)

func TestCampaignCreateAsTenantOwner(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cc", Name: "CC", SIPDomain: "cc.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{
		"name": "spring promo",
		"mode": "press1",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["name"] != "spring promo" || got["mode"] != "press1" || got["status"] != "paused" {
		t.Fatalf("unexpected: %v", got)
	}
	if got["dial_ratio"].(float64) != 1.0 {
		t.Fatalf("want default dial_ratio 1.0, got %v", got["dial_ratio"])
	}
}

func TestCampaignInvalidModeRejected(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cim", Name: "x", SIPDomain: "cim.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{
		"name": "x", "mode": "telegram",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCampaignMissingNameRejected(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cmn", Name: "x", SIPDomain: "cmn.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"mode": "press1"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCampaignDuplicateNameReturns409(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cdn", Name: "x", SIPDomain: "cdn.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	body := map[string]any{"name": "dup", "mode": "press1"}
	if w := s.do(t, "POST", "/tenant/campaigns/", tok, body); w.Code != http.StatusCreated {
		t.Fatalf("first: %d", w.Code)
	}
	if w := s.do(t, "POST", "/tenant/campaigns/", tok, body); w.Code != http.StatusConflict {
		t.Fatalf("dup: want 409, got %d", w.Code)
	}
}

func TestCampaignListExcludesOtherTenants(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tA, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cla", Name: "A", SIPDomain: "cla.sip"})
	tB, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "clb", Name: "B", SIPDomain: "clb.sip"})

	tokA := s.tokenFor(t, 1, tA.ID, "tenant_owner")
	tokB := s.tokenFor(t, 2, tB.ID, "tenant_owner")
	s.do(t, "POST", "/tenant/campaigns/", tokA, map[string]any{"name": "for-a", "mode": "press1"})
	s.do(t, "POST", "/tenant/campaigns/", tokB, map[string]any{"name": "for-b", "mode": "press1"})

	w := s.do(t, "GET", "/tenant/campaigns/", tokA, nil)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var resp struct {
		Campaigns []map[string]any `json:"campaigns"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Campaigns) != 1 || resp.Campaigns[0]["name"] != "for-a" {
		t.Fatalf("tenant A should see only own campaign, got %v", resp.Campaigns)
	}
}

func TestCampaignGetCrossTenantReturns404(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tA, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cga", Name: "A", SIPDomain: "cga.sip"})
	tB, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cgb", Name: "B", SIPDomain: "cgb.sip"})

	tokB := s.tokenFor(t, 2, tB.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/campaigns/", tokB, map[string]any{"name": "bs-campaign", "mode": "press1"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	id := int64(created["id"].(float64))

	tokA := s.tokenFor(t, 1, tA.ID, "tenant_owner")
	w = s.do(t, "GET", "/tenant/campaigns/"+itoa(id), tokA, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant get: want 404 from RLS, got %d", w.Code)
	}
}

func TestCampaignUpdate(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cu", Name: "U", SIPDomain: "cu.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"name": "u1", "mode": "press1"})
	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	id := int64(created["id"].(float64))

	w = s.do(t, "PATCH", "/tenant/campaigns/"+itoa(id), tok, map[string]any{
		"status":     "active",
		"dial_ratio": 1.5,
	})
	if w.Code != 200 {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["status"] != "active" || got["dial_ratio"].(float64) != 1.5 {
		t.Fatalf("update did not apply: %v", got)
	}
}

func TestCampaignAgentRoleForbidden(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "caf", Name: "X", SIPDomain: "caf.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "agent")
	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"name": "x", "mode": "press1"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("agent should not have ManageCampaigns: want 403, got %d", w.Code)
	}
}

func TestCampaignSuperAdminCanAccessTenantRoute(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "GET", "/tenant/campaigns/", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("super_admin should pass RequireTenant for cross-tenant view: want 200, got %d", w.Code)
	}
}
