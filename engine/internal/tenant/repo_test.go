package tenant_test

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"p1/engine/internal/db"
	"p1/engine/internal/tenant"
	"p1/engine/migrations"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	if _, err := sqlDB.Exec(`DROP TABLE IF EXISTS goose_db_version, users, tenants CASCADE`); err != nil {
		t.Fatalf("drop tables: %v", err)
	}
	if _, err := sqlDB.Exec(`
		DO $$ BEGIN
			CREATE ROLE app_user LOGIN PASSWORD 'app_user_change_me' NOSUPERUSER;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;
	`); err != nil {
		t.Fatalf("create role: %v", err)
	}
	if _, err := sqlDB.Exec(`
		GRANT CONNECT ON DATABASE test TO app_user;
		GRANT USAGE ON SCHEMA public TO app_user;
	`); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if err := db.Migrate(sqlDB, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB.Close()

	appURL, err := rewriteUser(url, "app_user", "app_user_change_me")
	if err != nil {
		t.Fatalf("rewrite url: %v", err)
	}
	pool, err := pgxpool.New(ctx, appURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func rewriteUser(connURL, user, password string) (string, error) {
	u, err := url.Parse(connURL)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(user, password)
	return u.String(), nil
}

func TestCreateAndGetTenant(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := tenant.NewRepo(pool)

	created, err := repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{
		Slug: "acme", Name: "Acme Inc", SIPDomain: "acme.sip.local",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("created.ID is zero")
	}
	got, err := repo.GetTenantAsSuperAdmin(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Slug != "acme" || got.Name != "Acme Inc" {
		t.Fatalf("mismatch: %+v", got)
	}
}

func TestTenantSlugUniqueness(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := tenant.NewRepo(pool)

	_, err := repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "dup", Name: "a", SIPDomain: "a.sip"})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "dup", Name: "b", SIPDomain: "b.sip"})
	if err == nil {
		t.Fatal("expected slug uniqueness violation")
	}
}

func TestRLSPreventsCrossTenantUserRead(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := tenant.NewRepo(pool)

	tA, _ := repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "ta", Name: "A", SIPDomain: "a.sip.x"})
	tB, _ := repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "tb", Name: "B", SIPDomain: "b.sip.x"})

	if _, err := repo.CreateUserAsSuperAdmin(ctx, tenant.User{
		TenantID: &tA.ID, Email: "a@a.com", Role: "tenant_owner", PasswordHash: "x",
	}); err != nil {
		t.Fatalf("user A: %v", err)
	}
	if _, err := repo.CreateUserAsSuperAdmin(ctx, tenant.User{
		TenantID: &tB.ID, Email: "b@b.com", Role: "tenant_owner", PasswordHash: "x",
	}); err != nil {
		t.Fatalf("user B: %v", err)
	}

	asTenantA := db.Ctx{TenantID: tA.ID, Role: "tenant_owner"}
	var seen []string
	err := db.WithCtx(ctx, pool, asTenantA, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, "SELECT email FROM users ORDER BY email")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e string
			if err := rows.Scan(&e); err != nil {
				return err
			}
			seen = append(seen, e)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(seen) != 1 || seen[0] != "a@a.com" {
		t.Fatalf("RLS leak: tenant A should see only own users, got %v", seen)
	}
}

func TestSuperAdminSeesAllUsers(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := tenant.NewRepo(pool)

	tA, _ := repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "saA", Name: "A", SIPDomain: "saA.sip"})
	tB, _ := repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "saB", Name: "B", SIPDomain: "saB.sip"})
	repo.CreateUserAsSuperAdmin(ctx, tenant.User{TenantID: &tA.ID, Email: "a@x.com", Role: "agent", PasswordHash: "x"})
	repo.CreateUserAsSuperAdmin(ctx, tenant.User{TenantID: &tB.ID, Email: "b@x.com", Role: "agent", PasswordHash: "x"})

	count, err := repo.CountAllUsersAsSuperAdmin(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count < 2 {
		t.Fatalf("expected >=2 users visible to super_admin, got %d", count)
	}
}

func TestSuperAdminUserRequiresNilTenant(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := tenant.NewRepo(pool)

	t1, _ := repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "z", Name: "z", SIPDomain: "z.sip"})
	_, err := repo.CreateUserAsSuperAdmin(ctx, tenant.User{
		TenantID: &t1.ID, Email: "bad@admin.com", Role: "super_admin", PasswordHash: "x",
	})
	if err == nil {
		t.Fatal("super_admin user with tenant_id should be rejected by CHECK constraint")
	}
	var pgErr interface{ SQLState() string }
	if !errors.As(err, &pgErr) || pgErr.SQLState() != "23514" {
		t.Logf("expected check_violation, got %v", err)
	}
}
