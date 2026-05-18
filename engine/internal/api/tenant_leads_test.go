package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"p1/engine/internal/tenant"
)

func TestLeadCreate(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "lc", Name: "LC", SIPDomain: "lc.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	first := "Jane"
	w := s.do(t, "POST", "/tenant/leads/", tok, map[string]any{
		"phone_e164": "+15551234567",
		"first_name": first,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["phone_e164"] != "+15551234567" || got["status"] != "new" {
		t.Fatalf("unexpected: %v", got)
	}
}

func TestLeadInvalidPhoneRejected(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "lip", Name: "x", SIPDomain: "lip.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	cases := []string{"5551234567", "+1abc", "1234567890", ""}
	for _, p := range cases {
		w := s.do(t, "POST", "/tenant/leads/", tok, map[string]any{"phone_e164": p})
		if w.Code != http.StatusBadRequest {
			t.Errorf("phone %q: want 400, got %d", p, w.Code)
		}
	}
}

func TestLeadAttachToCampaignViaPatch(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "lac", Name: "x", SIPDomain: "lac.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")

	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"name": "C1", "mode": "press1"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create campaign: %d %s", w.Code, w.Body.String())
	}
	var camp map[string]any
	json.Unmarshal(w.Body.Bytes(), &camp)
	campID := int64(camp["id"].(float64))

	w = s.do(t, "POST", "/tenant/leads/", tok, map[string]any{"phone_e164": "+15554443333"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create lead: %d %s", w.Code, w.Body.String())
	}
	var L map[string]any
	json.Unmarshal(w.Body.Bytes(), &L)
	leadID := int64(L["id"].(float64))

	w = s.do(t, "PATCH", "/tenant/leads/"+strconv.FormatInt(leadID, 10), tok, map[string]any{"campaign_id": campID})
	if w.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if int64(got["campaign_id"].(float64)) != campID {
		t.Fatalf("campaign_id not updated: %v", got)
	}

	w = s.do(t, "PATCH", "/tenant/leads/"+strconv.FormatInt(leadID, 10), tok, map[string]any{"campaign_id": nil})
	if w.Code != http.StatusOK {
		t.Fatalf("detach: %d %s", w.Code, w.Body.String())
	}
	var after map[string]any
	json.Unmarshal(w.Body.Bytes(), &after)
	if v, ok := after["campaign_id"]; ok && v != nil {
		t.Fatalf("campaign_id should be nil after detach: %v", v)
	}
}

func TestLeadCreateWithDialDestinationAllowsEmptyPhone(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "ldd", Name: "x", SIPDomain: "ldd.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/leads/", tok, map[string]any{
		"dial_destination": "mikephone",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["dial_destination"] != "mikephone" {
		t.Fatalf("dial_destination not stored: %v", got)
	}
	if got["phone_e164"] == "" || got["phone_e164"] == nil {
		t.Fatalf("phone_e164 should be auto-populated, got %v", got["phone_e164"])
	}
}

func TestLeadDuplicateInCampaignReturns409(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "ldc", Name: "x", SIPDomain: "ldc.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")

	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"name": "c1", "mode": "press1"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create campaign: %d", w.Code)
	}
	var camp map[string]any
	json.Unmarshal(w.Body.Bytes(), &camp)
	campID := int64(camp["id"].(float64))

	body := map[string]any{"phone_e164": "+15559999999", "campaign_id": campID}
	if w := s.do(t, "POST", "/tenant/leads/", tok, body); w.Code != http.StatusCreated {
		t.Fatalf("first: %d body=%s", w.Code, w.Body.String())
	}
	if w := s.do(t, "POST", "/tenant/leads/", tok, body); w.Code != http.StatusConflict {
		t.Fatalf("dup: want 409, got %d", w.Code)
	}
}

func TestLeadListExcludesOtherTenants(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tA, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "lla", Name: "A", SIPDomain: "lla.sip"})
	tB, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "llb", Name: "B", SIPDomain: "llb.sip"})

	tokA := s.tokenFor(t, 1, tA.ID, "tenant_owner")
	tokB := s.tokenFor(t, 2, tB.ID, "tenant_owner")
	s.do(t, "POST", "/tenant/leads/", tokA, map[string]any{"phone_e164": "+15550000001"})
	s.do(t, "POST", "/tenant/leads/", tokB, map[string]any{"phone_e164": "+15550000002"})

	w := s.do(t, "GET", "/tenant/leads/", tokA, nil)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var resp struct {
		Leads []map[string]any `json:"leads"`
		Total int              `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 1 || len(resp.Leads) != 1 || resp.Leads[0]["phone_e164"] != "+15550000001" {
		t.Fatalf("tenant A should see only own lead, got total=%d leads=%v", resp.Total, resp.Leads)
	}
}

func TestLeadGetCrossTenantReturns404(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tA, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "lga", Name: "A", SIPDomain: "lga.sip"})
	tB, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "lgb", Name: "B", SIPDomain: "lgb.sip"})

	tokB := s.tokenFor(t, 2, tB.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/leads/", tokB, map[string]any{"phone_e164": "+15558888888"})
	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	id := int64(created["id"].(float64))

	tokA := s.tokenFor(t, 1, tA.ID, "tenant_owner")
	w = s.do(t, "GET", "/tenant/leads/"+itoa(id), tokA, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant get: want 404 from RLS, got %d", w.Code)
	}
}

func TestLeadDelete(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "ld", Name: "D", SIPDomain: "ld.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/leads/", tok, map[string]any{"phone_e164": "+15557777777"})
	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	id := int64(created["id"].(float64))

	w = s.do(t, "DELETE", "/tenant/leads/"+itoa(id), tok, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", w.Code)
	}

	w = s.do(t, "GET", "/tenant/leads/"+itoa(id), tok, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("after delete: want 404, got %d", w.Code)
	}
}

func TestLeadPagination(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "lp", Name: "P", SIPDomain: "lp.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	for i := 0; i < 5; i++ {
		s.do(t, "POST", "/tenant/leads/", tok, map[string]any{
			"phone_e164": "+1555000010" + string(rune('0'+i)),
		})
	}

	w := s.do(t, "GET", "/tenant/leads/?limit=2&offset=0", tok, nil)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var resp struct {
		Leads  []map[string]any `json:"leads"`
		Total  int              `json:"total"`
		Limit  int              `json:"limit"`
		Offset int              `json:"offset"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 5 || len(resp.Leads) != 2 || resp.Limit != 2 {
		t.Fatalf("pagination: total=%d returned=%d limit=%d", resp.Total, len(resp.Leads), resp.Limit)
	}
}

func TestLeadAgentRoleForbidden(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "laf", Name: "X", SIPDomain: "laf.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "agent")
	w := s.do(t, "POST", "/tenant/leads/", tok, map[string]any{"phone_e164": "+15551111111"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("agent should not have ManageLeads: want 403, got %d", w.Code)
	}
}
