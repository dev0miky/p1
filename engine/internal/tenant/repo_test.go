package tenant_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"p1/engine/internal/db"
	"p1/engine/internal/tenant"
	"p1/engine/internal/testutil"
)

func testPool(t *testing.T) *pgxpool.Pool {
	return testutil.TestPool(t)
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
