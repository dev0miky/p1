package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"p1/engine/internal/tenant"
)

func seedLeadIDs(t *testing.T, s stack, ctx context.Context, tn tenant.Tenant, tok string, phones ...string) []int64 {
	t.Helper()
	var ids []int64
	for _, p := range phones {
		w := s.do(t, "POST", "/tenant/leads/", tok, map[string]any{"phone_e164": p})
		if w.Code != http.StatusCreated {
			t.Fatalf("seed lead %s: %d %s", p, w.Code, w.Body.String())
		}
		var L map[string]any
		json.Unmarshal(w.Body.Bytes(), &L)
		ids = append(ids, int64(L["id"].(float64)))
	}
	_ = ctx
	_ = tn
	return ids
}

func TestBulkDelete(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "bd", Name: "x", SIPDomain: "bd.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	ids := seedLeadIDs(t, s, ctx, tn, tok, "+15550000100", "+15550000101", "+15550000102")

	w := s.do(t, "POST", "/tenant/leads/bulk/delete", tok, map[string]any{"lead_ids": ids})
	if w.Code != http.StatusOK {
		t.Fatalf("bulk delete: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["deleted"].(float64)) != 3 {
		t.Fatalf("want 3 deleted, got %v", resp["deleted"])
	}

	// Verify they're gone.
	w = s.do(t, "GET", "/tenant/leads/?limit=10", tok, nil)
	var listResp struct {
		Total int `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	if listResp.Total != 0 {
		t.Fatalf("total after bulk delete: %d", listResp.Total)
	}
}

func TestBulkAttach(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "ba", Name: "x", SIPDomain: "ba.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")

	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"name": "c1", "mode": "broadcast"})
	var c map[string]any
	json.Unmarshal(w.Body.Bytes(), &c)
	campID := int64(c["id"].(float64))

	ids := seedLeadIDs(t, s, ctx, tn, tok, "+15550000200", "+15550000201", "+15550000202")

	w = s.do(t, "POST", "/tenant/leads/bulk/attach", tok, map[string]any{"lead_ids": ids, "campaign_id": campID})
	if w.Code != http.StatusOK {
		t.Fatalf("attach: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["updated"].(float64)) != 3 {
		t.Fatalf("want 3 updated, got %v", resp["updated"])
	}

	// Spot check one lead
	w = s.do(t, "GET", "/tenant/leads/"+strconv.FormatInt(ids[0], 10), tok, nil)
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if int64(got["campaign_id"].(float64)) != campID {
		t.Fatalf("campaign_id not set: %v", got["campaign_id"])
	}

	// Bulk detach (null campaign_id)
	w = s.do(t, "POST", "/tenant/leads/bulk/attach", tok, map[string]any{"lead_ids": ids, "campaign_id": nil})
	if w.Code != http.StatusOK {
		t.Fatalf("detach: %d %s", w.Code, w.Body.String())
	}
	w = s.do(t, "GET", "/tenant/leads/"+strconv.FormatInt(ids[0], 10), tok, nil)
	var after map[string]any
	json.Unmarshal(w.Body.Bytes(), &after)
	if v, ok := after["campaign_id"]; ok && v != nil {
		t.Fatalf("campaign_id should be nil after detach: %v", v)
	}
}

func TestBulkDNC(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "bdn", Name: "x", SIPDomain: "bdn.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	ids := seedLeadIDs(t, s, ctx, tn, tok, "+15550000300", "+15550000301", "+15550000302")

	w := s.do(t, "POST", "/tenant/leads/bulk/dnc", tok, map[string]any{"lead_ids": ids, "reason": "customer requested"})
	if w.Code != http.StatusOK {
		t.Fatalf("bulk dnc: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["dnc_added"].(float64)) != 3 || int(resp["leads_marked"].(float64)) != 3 {
		t.Fatalf("unexpected: %v", resp)
	}

	// Verify dnc check returns blocked.
	w = s.do(t, "GET", "/tenant/dnc/check?phone=%2B15550000300", tok, nil)
	var chk map[string]any
	json.Unmarshal(w.Body.Bytes(), &chk)
	if chk["blocked"] != true {
		t.Fatalf("dnc check: %v", chk)
	}

	// Verify the lead status flipped.
	w = s.do(t, "GET", "/tenant/leads/"+strconv.FormatInt(ids[0], 10), tok, nil)
	var L map[string]any
	json.Unmarshal(w.Body.Bytes(), &L)
	if L["status"] != "dnc" {
		t.Fatalf("status: %v", L["status"])
	}

	// Re-running should be idempotent (no new dnc rows, no new lead status flips).
	w = s.do(t, "POST", "/tenant/leads/bulk/dnc", tok, map[string]any{"lead_ids": ids})
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["dnc_added"].(float64)) != 0 || int(resp["leads_marked"].(float64)) != 0 {
		t.Fatalf("idempotent: %v", resp)
	}
}

func TestBulkRejectsOverLimit(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "bro", Name: "x", SIPDomain: "bro.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	ids := make([]int64, 1001)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	if w := s.do(t, "POST", "/tenant/leads/bulk/delete", tok, map[string]any{"lead_ids": ids}); w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 over-limit, got %d", w.Code)
	}
	if w := s.do(t, "POST", "/tenant/leads/bulk/delete", tok, map[string]any{"lead_ids": []int64{}}); w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 empty, got %d", w.Code)
	}
	_ = ctx
}
