package audit_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"p1/engine/internal/audit"
	"p1/engine/internal/db"
	"p1/engine/internal/testutil"
)

func setupAuditDB(t *testing.T) *pgxpool.Pool {
	return testutil.TestPool(t)
}

func TestLogRoundtrip(t *testing.T) {
	pool := setupAuditDB(t)
	ctx := context.Background()

	tenantID := int64(42)
	err := db.WithCtx(ctx, pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		return audit.Log(ctx, tx, audit.Entry{
			RequestID:  "req-abc",
			ActorType:  "user",
			ActorID:    "1",
			TenantID:   &tenantID,
			EntityType: "tenant",
			EntityID:   "42",
			Action:     "create",
			After:      map[string]any{"slug": "acme", "name": "Acme"},
			IP:         "203.0.113.1",
			UserAgent:  "curl/8",
		})
	})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	var count int
	if err := db.WithCtx(ctx, pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE request_id='req-abc'`).Scan(&count)
	}); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
}

func TestAuditLogRejectsUpdateAttempt(t *testing.T) {
	pool := setupAuditDB(t)
	ctx := context.Background()

	if err := db.WithCtx(ctx, pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		return audit.Log(ctx, tx, audit.Entry{
			ActorType: "system", EntityType: "x", Action: "y",
		})
	}); err != nil {
		t.Fatal(err)
	}

	_, err := pool.Exec(ctx, `UPDATE audit_log SET action='tampered' WHERE id = 1`)
	if err == nil {
		t.Fatal("UPDATE on audit_log should be denied")
	}

	_, err = pool.Exec(ctx, `DELETE FROM audit_log WHERE id = 1`)
	if err == nil {
		t.Fatal("DELETE on audit_log should be denied")
	}
}

func TestTenantUserCannotReadOtherTenantAudit(t *testing.T) {
	pool := setupAuditDB(t)
	ctx := context.Background()

	a, b := int64(1), int64(2)
	err := db.WithCtx(ctx, pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		if err := audit.Log(ctx, tx, audit.Entry{TenantID: &a, ActorType: "user", EntityType: "campaign", EntityID: "1", Action: "create"}); err != nil {
			return err
		}
		return audit.Log(ctx, tx, audit.Entry{TenantID: &b, ActorType: "user", EntityType: "campaign", EntityID: "2", Action: "create"})
	})
	if err != nil {
		t.Fatal(err)
	}

	var ids []int64
	err = db.WithCtx(ctx, pool, db.Ctx{Role: "tenant_admin", TenantID: a}, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id FROM audit_log ORDER BY id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("tenant_admin should see only own audit rows, got %d (ids %v)", len(ids), ids)
	}
}
