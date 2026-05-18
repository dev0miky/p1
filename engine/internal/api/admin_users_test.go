package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"p1/engine/internal/tenant"
)

func TestCreateUserAsSuperAdmin(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "u", Name: "U", SIPDomain: "u.sip"})

	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "POST", "/admin/users/", tok, map[string]any{
		"tenant_id": tn.ID,
		"email":     "owner@u.com",
		"role":      "tenant_owner",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["email"] != "owner@u.com" || got["role"] != "tenant_owner" {
		t.Fatalf("unexpected: %v", got)
	}
	if got["temp_password"] == nil || got["temp_password"] == "" {
		t.Fatalf("expected temp_password in response when no password provided")
	}
}

func TestCreateSuperAdminUser(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "POST", "/admin/users/", tok, map[string]any{
		"email":    "root@example.com",
		"role":     "super_admin",
		"password": "supersecret123",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["tenant_id"] != nil {
		t.Fatalf("super_admin should have nil tenant_id, got %v", got["tenant_id"])
	}
	if got["temp_password"] != nil && got["temp_password"] != "" {
		t.Fatalf("explicit password should not return temp_password, got %v", got["temp_password"])
	}
}

func TestCreateSuperAdminWithTenantIDRejected(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "x", Name: "X", SIPDomain: "x.sip"})

	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "POST", "/admin/users/", tok, map[string]any{
		"tenant_id": tn.ID,
		"email":     "bad@root.com",
		"role":      "super_admin",
		"password":  "supersecret123",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCreateNonAdminWithoutTenantRejected(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "POST", "/admin/users/", tok, map[string]any{
		"email":    "ten@noT.com",
		"role":     "tenant_owner",
		"password": "supersecret123",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCreateUserInvalidRoleRejected(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "POST", "/admin/users/", tok, map[string]any{
		"email":    "x@y.com",
		"role":     "nonsense",
		"password": "supersecret123",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCreateUserInvalidEmailRejected(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "POST", "/admin/users/", tok, map[string]any{
		"email":    "not-an-email",
		"role":     "super_admin",
		"password": "supersecret123",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCreateUserShortPasswordRejected(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "POST", "/admin/users/", tok, map[string]any{
		"email":    "short@p.com",
		"role":     "super_admin",
		"password": "abc",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCreateUserDuplicateEmailReturns409(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "d", Name: "D", SIPDomain: "d.sip"})
	tok := s.tokenFor(t, 1, 0, "super_admin")

	body := map[string]any{"tenant_id": tn.ID, "email": "dup@x.com", "role": "agent"}
	if w := s.do(t, "POST", "/admin/users/", tok, body); w.Code != http.StatusCreated {
		t.Fatalf("first: %d body=%s", w.Code, w.Body.String())
	}
	if w := s.do(t, "POST", "/admin/users/", tok, body); w.Code != http.StatusConflict {
		t.Fatalf("dup: want 409, got %d", w.Code)
	}
}

func TestListUsersAsSuperAdmin(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "l", Name: "L", SIPDomain: "l.sip"})
	s.repo.CreateUserAsSuperAdmin(ctx, tenant.User{TenantID: &tn.ID, Email: "a@l.com", Role: "agent", PasswordHash: "x"})
	s.repo.CreateUserAsSuperAdmin(ctx, tenant.User{TenantID: nil, Email: "root@l.com", Role: "super_admin", PasswordHash: "x"})

	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "GET", "/admin/users/", tok, nil)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var resp struct {
		Users []map[string]any `json:"users"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Users) < 2 {
		t.Fatalf("want >=2 users, got %d", len(resp.Users))
	}
}

func TestUpdateUserStatus(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "us", Name: "US", SIPDomain: "us.sip"})
	u, _ := s.repo.CreateUserAsSuperAdmin(ctx, tenant.User{TenantID: &tn.ID, Email: "u@us.com", Role: "agent", PasswordHash: "x"})

	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "PATCH", "/admin/users/"+itoa(u.ID), tok, map[string]string{"status": "suspended"})
	if w.Code != 200 {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["status"] != "suspended" {
		t.Fatalf("status not updated: %v", got)
	}
}

func TestCreateUserAsTenantOwnerForbidden(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 2, 1, "tenant_owner")
	w := s.do(t, "POST", "/admin/users/", tok, map[string]any{
		"email": "x@x.com", "role": "agent",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}
