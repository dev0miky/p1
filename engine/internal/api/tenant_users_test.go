package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"p1/engine/internal/tenant"
)

func TestTenantOwnerCanCreateAgent(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "to1", Name: "T1", SIPDomain: "t1.sip"})

	tok := s.tokenFor(t, 100, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/users/", tok, map[string]any{
		"email": "agent@t1.com", "role": "agent",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["tenant_id"] != float64(tn.ID) {
		t.Fatalf("tenant_id mismatch: got %v want %d", got["tenant_id"], tn.ID)
	}
}

func TestTenantOwnerCannotCreateSuperAdmin(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "tsa", Name: "T", SIPDomain: "tsa.sip"})

	tok := s.tokenFor(t, 100, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/users/", tok, map[string]any{
		"email": "root@x.com", "role": "super_admin", "password": "longenoughpw",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTenantAdminCannotCreateUser(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "tac", Name: "T", SIPDomain: "tac.sip"})

	tok := s.tokenFor(t, 100, tn.ID, "tenant_admin")
	w := s.do(t, "POST", "/tenant/users/", tok, map[string]any{
		"email": "a@x.com", "role": "agent",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("tenant_admin should not have ManageUsers: want 403, got %d", w.Code)
	}
}

func TestTenantUsersListExcludesOtherTenants(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tA, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "ta", Name: "A", SIPDomain: "ta.sip"})
	tB, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "tb", Name: "B", SIPDomain: "tb.sip"})
	s.repo.CreateUserAsSuperAdmin(ctx, tenant.User{TenantID: &tA.ID, Email: "a@a.com", Role: "agent", PasswordHash: "x"})
	s.repo.CreateUserAsSuperAdmin(ctx, tenant.User{TenantID: &tB.ID, Email: "b@b.com", Role: "agent", PasswordHash: "x"})

	tokA := s.tokenFor(t, 1, tA.ID, "tenant_owner")
	w := s.do(t, "GET", "/tenant/users/", tokA, nil)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var resp struct {
		Users []map[string]any `json:"users"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	for _, u := range resp.Users {
		tid, _ := u["tenant_id"].(float64)
		if int64(tid) != tA.ID {
			t.Fatalf("tenant A should not see tenant_id %v in users list: %v", tid, u)
		}
	}
}

func TestTenantUserCannotGetCrossTenantUser(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tA, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "ta2", Name: "A", SIPDomain: "ta2.sip"})
	tB, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "tb2", Name: "B", SIPDomain: "tb2.sip"})
	uB, _ := s.repo.CreateUserAsSuperAdmin(ctx, tenant.User{TenantID: &tB.ID, Email: "b@b2.com", Role: "agent", PasswordHash: "x"})

	tokA := s.tokenFor(t, 1, tA.ID, "tenant_owner")
	w := s.do(t, "GET", "/tenant/users/"+itoa(uB.ID), tokA, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant get should 404 via RLS, got %d", w.Code)
	}
}

func TestSuperAdminBlockedFromTenantEndpoints(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "GET", "/tenant/users/", tok, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("super_admin should be blocked by RequireTenant: want 403, got %d", w.Code)
	}
}
