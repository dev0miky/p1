package auth_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/tenant"
	"p1/engine/migrations"
)

func setupHandler(t *testing.T) (*auth.Handler, *auth.Issuer, *tenant.Repo) {
	t.Helper()
	connURL := os.Getenv("TEST_DATABASE_URL")
	if connURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	sqlDB, err := sql.Open("pgx", connURL)
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	if _, err := sqlDB.Exec(`DROP TABLE IF EXISTS goose_db_version, users, tenants CASCADE`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if _, err := sqlDB.Exec(`
		DO $$ BEGIN
			CREATE ROLE app_user LOGIN PASSWORD 'app_user_change_me' NOSUPERUSER;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;
		GRANT CONNECT ON DATABASE test TO app_user;
		GRANT USAGE ON SCHEMA public TO app_user;
	`); err != nil {
		t.Fatalf("role: %v", err)
	}
	if err := db.Migrate(sqlDB, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB.Close()

	u, _ := url.Parse(connURL)
	u.User = url.UserPassword("app_user", "app_user_change_me")
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	repo := tenant.NewRepo(pool)
	iss := auth.NewIssuer([]byte("test-secret-32-bytes-long-aaaaaa"), "p1", time.Hour)
	h := auth.NewHandler(repo, iss)
	return h, iss, repo
}

func seedUser(t *testing.T, repo *tenant.Repo, tenantID *int64, email, password, role string) {
	t.Helper()
	hash, err := auth.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateUserAsSuperAdmin(context.Background(), tenant.User{
		TenantID: tenantID, Email: email, Role: role, PasswordHash: hash,
	}); err != nil {
		t.Fatal(err)
	}
}

func postJSON(h http.Handler, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestLoginSuperAdminSuccess(t *testing.T) {
	h, iss, repo := setupHandler(t)
	seedUser(t, repo, nil, "admin@example.com", "supersecretpw", "super_admin")

	w := postJSON(http.HandlerFunc(h.Login), "/auth/login", map[string]string{
		"email":    "admin@example.com",
		"password": "supersecretpw",
	})
	if w.Code != 200 {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Token == "" {
		t.Fatal("token empty")
	}
	claims, err := iss.Verify(resp.Token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Role != "super_admin" {
		t.Fatalf("role: %s", claims.Role)
	}
}

func TestLoginTenantUserSuccess(t *testing.T) {
	h, _, repo := setupHandler(t)
	tn, err := repo.CreateTenantAsSuperAdmin(context.Background(), tenant.Tenant{
		Slug: "acme", Name: "Acme", SIPDomain: "acme.sip",
	})
	if err != nil {
		t.Fatal(err)
	}
	seedUser(t, repo, &tn.ID, "owner@acme.com", "pw1234567890", "tenant_owner")

	w := postJSON(http.HandlerFunc(h.Login), "/auth/login", map[string]string{
		"tenant_slug": "acme",
		"email":       "owner@acme.com",
		"password":    "pw1234567890",
	})
	if w.Code != 200 {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestLoginWrongPasswordReturns401(t *testing.T) {
	h, _, repo := setupHandler(t)
	seedUser(t, repo, nil, "admin@example.com", "rightpassword", "super_admin")

	w := postJSON(http.HandlerFunc(h.Login), "/auth/login", map[string]string{
		"email":    "admin@example.com",
		"password": "wrongpassword",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestLoginUnknownEmailReturns401(t *testing.T) {
	h, _, _ := setupHandler(t)
	w := postJSON(http.HandlerFunc(h.Login), "/auth/login", map[string]string{
		"email":    "nobody@example.com",
		"password": "anything12345",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestLoginMissingFieldsReturns400(t *testing.T) {
	h, _, _ := setupHandler(t)
	cases := []map[string]string{
		{"email": ""},
		{"password": "x"},
		{"email": "a@a.com"},
	}
	for _, body := range cases {
		w := postJSON(http.HandlerFunc(h.Login), "/auth/login", body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %v: want 400, got %d", body, w.Code)
		}
	}
}

func TestLoginSuspendedUserReturns403(t *testing.T) {
	h, _, repo := setupHandler(t)
	seedUser(t, repo, nil, "suspended@example.com", "pwpwpwpwpw", "super_admin")

	if _, err := repo.SetUserStatusAsSuperAdmin(context.Background(), "suspended@example.com", "suspended"); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	w := postJSON(http.HandlerFunc(h.Login), "/auth/login", map[string]string{
		"email":    "suspended@example.com",
		"password": "pwpwpwpwpw",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}
