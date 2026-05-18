package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func mustIssue(t *testing.T, iss *Issuer, c Claims) string {
	t.Helper()
	tok, err := iss.Issue(c)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestMiddlewareAttachesClaimsOnValidToken(t *testing.T) {
	iss := NewIssuer([]byte("test-secret-32-bytes-long-aaaaaa"), "p1", time.Hour)
	tok := mustIssue(t, iss, Claims{UserID: 5, TenantID: 9, Role: "tenant_admin"})

	var seen Claims
	h := RequireAuth(iss)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := ClaimsFromContext(r.Context())
		if !ok {
			t.Fatal("claims not in context")
		}
		seen = c
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	if seen.UserID != 5 || seen.TenantID != 9 || seen.Role != "tenant_admin" {
		t.Fatalf("claims mismatch: %+v", seen)
	}
}

func TestMiddlewareRejectsMissingHeader(t *testing.T) {
	iss := NewIssuer([]byte("test-secret-32-bytes-long-aaaaaa"), "p1", time.Hour)
	h := RequireAuth(iss)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestMiddlewareRejectsMalformedHeader(t *testing.T) {
	iss := NewIssuer([]byte("test-secret-32-bytes-long-aaaaaa"), "p1", time.Hour)
	cases := []string{"", "Token x", "Bearer", "Bearer ", "Basic x"}
	for _, hdr := range cases {
		h := RequireAuth(iss)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("handler called for header %q", hdr)
		}))
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", hdr)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("header %q: want 401, got %d", hdr, w.Code)
		}
	}
}

func TestMiddlewareRejectsInvalidToken(t *testing.T) {
	iss := NewIssuer([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), "p1", time.Hour)
	other := NewIssuer([]byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), "p1", time.Hour)
	tok := mustIssue(t, other, Claims{UserID: 1, TenantID: 1, Role: "agent"})

	h := RequireAuth(iss)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestRequireRoleAllowsAndDenies(t *testing.T) {
	iss := NewIssuer([]byte("test-secret-32-bytes-long-aaaaaa"), "p1", time.Hour)
	tokAgent := mustIssue(t, iss, Claims{UserID: 1, TenantID: 1, Role: "agent"})
	tokOwner := mustIssue(t, iss, Claims{UserID: 2, TenantID: 1, Role: "tenant_owner"})

	chain := func(next http.Handler) http.Handler {
		return RequireAuth(iss)(RequireAction(ActionManageBilling)(next))
	}

	called := false
	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))

	called = false
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokAgent)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("agent: want 403, got %d", w.Code)
	}
	if called {
		t.Fatal("agent: handler should not be called")
	}

	called = false
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokOwner)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("owner: want 200, got %d", w.Code)
	}
	if !called {
		t.Fatal("owner: handler should be called")
	}
}
