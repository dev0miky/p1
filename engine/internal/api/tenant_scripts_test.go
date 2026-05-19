package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"p1/engine/internal/tenant"
)

func TestScriptsCRUD(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "sc", Name: "x", SIPDomain: "sc.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")

	w := s.do(t, "POST", "/tenant/scripts/", tok, map[string]any{"name": "press1-spring", "type": "press1", "body": "welcome..."})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var c map[string]any
	json.Unmarshal(w.Body.Bytes(), &c)
	id := int64(c["id"].(float64))
	if c["type"] != "press1" || c["body"] != "welcome..." {
		t.Fatalf("create response: %v", c)
	}

	w = s.do(t, "POST", "/tenant/scripts/", tok, map[string]any{"name": "bad", "type": "telegram"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid type: want 400, got %d", w.Code)
	}

	w = s.do(t, "PATCH", "/tenant/scripts/"+strconv.FormatInt(id, 10), tok, map[string]any{"body": "updated body"})
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &c)
	if c["body"] != "updated body" {
		t.Fatalf("update didn't apply: %v", c)
	}

	w = s.do(t, "GET", "/tenant/scripts/", tok, nil)
	var lresp struct {
		Scripts []map[string]any `json:"scripts"`
	}
	json.Unmarshal(w.Body.Bytes(), &lresp)
	if len(lresp.Scripts) != 1 {
		t.Fatalf("list: %v", lresp)
	}

	w = s.do(t, "DELETE", "/tenant/scripts/"+strconv.FormatInt(id, 10), tok, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
}

func TestLeadListsCRUD(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "ll", Name: "x", SIPDomain: "ll.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")

	w := s.do(t, "POST", "/tenant/lists/", tok, map[string]any{"name": "spring-leads"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var L map[string]any
	json.Unmarshal(w.Body.Bytes(), &L)
	id := int64(L["id"].(float64))

	w = s.do(t, "GET", "/tenant/lists/", tok, nil)
	var lresp struct {
		Lists []map[string]any `json:"lists"`
	}
	json.Unmarshal(w.Body.Bytes(), &lresp)
	if len(lresp.Lists) != 1 || lresp.Lists[0]["lead_count"].(float64) != 0 {
		t.Fatalf("list: %v", lresp)
	}

	w = s.do(t, "PATCH", "/tenant/lists/"+strconv.FormatInt(id, 10), tok, map[string]any{"name": "renamed"})
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d", w.Code)
	}
	var g map[string]any
	json.Unmarshal(w.Body.Bytes(), &g)
	if g["name"] != "renamed" {
		t.Fatalf("update didn't apply: %v", g)
	}

	w = s.do(t, "DELETE", "/tenant/lists/"+strconv.FormatInt(id, 10), tok, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
}
