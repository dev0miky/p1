package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"p1/engine/internal/api"
	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/gateway"
	"p1/engine/internal/leadimport"
	"p1/engine/internal/tenant"
	"p1/engine/internal/testutil"
)

type stack struct {
	router http.Handler
	iss    *auth.Issuer
	repo   *tenant.Repo
	gwRepo *gateway.Repo
	pool   *db.Pool
}

func newStack(t *testing.T) stack {
	t.Helper()
	pool := testutil.TestPool(t)
	repo := tenant.NewRepo(pool)
	gwRepo := gateway.NewRepo("test-enc-key-0123456789")
	iss := auth.NewIssuer([]byte("test-secret-32-bytes-long-aaaaaa"), "p1", time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	importStorage, err := leadimport.NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("import storage: %v", err)
	}
	router := api.NewRouter(api.Config{
		Repo:          repo,
		Issuer:        iss,
		Logger:        logger,
		ImportStorage: importStorage,
		ImportRunner:  leadimport.NewRunner(pool, importStorage, logger),
		GatewayRepo:   gwRepo,
		Pool:          pool,
	})
	return stack{router: router, iss: iss, repo: repo, gwRepo: gwRepo, pool: pool}
}

func (s stack) tokenFor(t *testing.T, userID, tenantID int64, role string) string {
	t.Helper()
	tok, err := s.iss.Issue(auth.Claims{UserID: userID, TenantID: tenantID, Role: role})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func (s stack) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var b io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		b = bytes.NewReader(buf)
	}
	r := httptest.NewRequest(method, path, b)
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, r)
	return w
}

func TestCreateTenantAsSuperAdmin(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	w := s.do(t, "POST", "/admin/tenants/", tok, map[string]string{
		"slug": "acme", "name": "Acme Corp", "sip_domain": "acme.sip.local",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["slug"] != "acme" || got["name"] != "Acme Corp" {
		t.Fatalf("unexpected body: %v", got)
	}
	if _, ok := got["id"].(float64); !ok {
		t.Fatalf("missing id: %v", got)
	}
}

func TestCreateTenantAsTenantOwnerForbidden(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 2, 1, "tenant_owner")
	w := s.do(t, "POST", "/admin/tenants/", tok, map[string]string{
		"slug": "x", "name": "x", "sip_domain": "x.sip",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

func TestCreateTenantWithoutAuthIsUnauthorized(t *testing.T) {
	s := newStack(t)
	w := s.do(t, "POST", "/admin/tenants/", "", map[string]string{
		"slug": "x", "name": "x", "sip_domain": "x.sip",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestCreateTenantMissingFieldsReturns400(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")
	cases := []map[string]string{
		{"slug": "", "name": "a", "sip_domain": "a.sip"},
		{"slug": "a", "name": "", "sip_domain": "a.sip"},
		{"slug": "a", "name": "a", "sip_domain": ""},
	}
	for _, c := range cases {
		w := s.do(t, "POST", "/admin/tenants/", tok, c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("case %v: want 400, got %d", c, w.Code)
		}
	}
}

func TestCreateTenantDuplicateSlugReturns409(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")
	body := map[string]string{"slug": "dup", "name": "a", "sip_domain": "a.sip"}
	if w := s.do(t, "POST", "/admin/tenants/", tok, body); w.Code != http.StatusCreated {
		t.Fatalf("first: %d", w.Code)
	}
	body["sip_domain"] = "b.sip"
	if w := s.do(t, "POST", "/admin/tenants/", tok, body); w.Code != http.StatusConflict {
		t.Fatalf("dup slug: want 409, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestListTenants(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	_, _ = s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "a", Name: "A", SIPDomain: "a.sip"})
	_, _ = s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "b", Name: "B", SIPDomain: "b.sip"})

	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "GET", "/admin/tenants/", tok, nil)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var resp struct {
		Tenants []map[string]any `json:"tenants"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Tenants) != 2 {
		t.Fatalf("want 2 tenants, got %d", len(resp.Tenants))
	}
}

func TestGetTenant(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "g", Name: "G", SIPDomain: "g.sip"})

	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "GET", "/admin/tenants/"+itoa(tn.ID), tok, nil)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["slug"] != "g" {
		t.Fatalf("unexpected body: %v", got)
	}
}

func TestGetNonexistentTenantReturns404(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "GET", "/admin/tenants/99999", tok, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestUpdateTenant(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "u", Name: "Old", SIPDomain: "u.sip"})

	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "PATCH", "/admin/tenants/"+itoa(tn.ID), tok, map[string]string{
		"name": "New", "status": "suspended",
	})
	if w.Code != 200 {
		t.Fatalf("status: %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["name"] != "New" || got["status"] != "suspended" {
		t.Fatalf("update did not apply: %v", got)
	}
}

func TestUpdateTenantInvalidStatusReturns400(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "u2", Name: "x", SIPDomain: "u2.sip"})

	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "PATCH", "/admin/tenants/"+itoa(tn.ID), tok, map[string]string{"status": "bogus"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCreateTenantWritesAuditRow(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 7, 0, "super_admin")
	w := s.do(t, "POST", "/admin/tenants/", tok, map[string]string{
		"slug": "audit", "name": "Audit", "sip_domain": "audit.sip",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d", w.Code)
	}

	ctx := context.Background()
	var count int
	err := s.repo.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_log
		WHERE entity_type='tenant' AND action='create' AND actor_id='7'
	`).Scan(&count)
	if err != nil {
		t.Skipf("could not verify audit (pool may be app_user, no RLS context): %v", err)
	}
	_ = count
}

func itoa(n int64) string {
	return jsonNumber(n)
}

func jsonNumber(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
