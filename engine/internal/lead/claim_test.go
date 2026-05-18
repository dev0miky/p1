package lead_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"p1/engine/internal/db"
	"p1/engine/internal/lead"
	"p1/engine/internal/tenant"
	"p1/engine/internal/testutil"
)

func setupLeads(t *testing.T) (*db.Pool, int64, int64) {
	t.Helper()
	pool := testutil.TestPool(t)
	ctx := context.Background()
	tn, err := tenant.NewRepo(pool).CreateTenantAsSuperAdmin(ctx, tenant.Tenant{
		Slug: "lc", Name: "LC", SIPDomain: "lc.sip",
	})
	if err != nil {
		t.Fatal(err)
	}
	var campID int64
	err = db.WithCtx(ctx, pool, db.Ctx{Role: "tenant_owner", TenantID: tn.ID}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO campaigns (tenant_id, name, mode)
			VALUES ($1, 'c1', 'press1') RETURNING id
		`, tn.ID).Scan(&campID)
	})
	if err != nil {
		t.Fatal(err)
	}
	return pool, tn.ID, campID
}

func seedLead(t *testing.T, pool *db.Pool, tid, campID int64, phone string) int64 {
	t.Helper()
	repo := lead.NewRepo()
	var id int64
	err := db.WithCtx(context.Background(), pool, db.Ctx{Role: "tenant_owner", TenantID: tid}, func(tx pgx.Tx) error {
		l, err := repo.CreateLeadTx(context.Background(), tx, lead.Lead{
			TenantID: tid, CampaignID: &campID, PhoneE164: phone,
		})
		if err != nil {
			return err
		}
		id = l.ID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestClaimBatchReturnsLeadsAndMarksInFlight(t *testing.T) {
	pool, tid, campID := setupLeads(t)
	repo := lead.NewRepo()
	ctx := context.Background()

	seedLead(t, pool, tid, campID, "+15550000001")
	seedLead(t, pool, tid, campID, "+15550000002")
	seedLead(t, pool, tid, campID, "+15550000003")

	var claimed []lead.Lead
	err := db.WithCtx(ctx, pool, db.Ctx{Role: "tenant_owner", TenantID: tid}, func(tx pgx.Tx) error {
		var err error
		claimed, err = repo.ClaimBatchTx(ctx, tx, lead.ClaimOptions{
			CampaignID: campID, NodeID: "node-1", Limit: 2, LockFor: time.Minute,
		})
		return err
	})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 2 {
		t.Fatalf("expected 2 claimed leads, got %d", len(claimed))
	}
	for _, l := range claimed {
		if l.Status != "in_flight" {
			t.Errorf("lead %d not in_flight: %s", l.ID, l.Status)
		}
		if l.Attempts != 1 {
			t.Errorf("lead %d attempts: %d", l.ID, l.Attempts)
		}
	}
}

func TestClaimBatchSkipsAlreadyLocked(t *testing.T) {
	pool, tid, campID := setupLeads(t)
	repo := lead.NewRepo()
	ctx := context.Background()

	seedLead(t, pool, tid, campID, "+15550000001")
	seedLead(t, pool, tid, campID, "+15550000002")

	err := db.WithCtx(ctx, pool, db.Ctx{Role: "tenant_owner", TenantID: tid}, func(tx pgx.Tx) error {
		_, err := repo.ClaimBatchTx(ctx, tx, lead.ClaimOptions{CampaignID: campID, NodeID: "node-1", Limit: 10})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	var second []lead.Lead
	err = db.WithCtx(ctx, pool, db.Ctx{Role: "tenant_owner", TenantID: tid}, func(tx pgx.Tx) error {
		var err error
		second, err = repo.ClaimBatchTx(ctx, tx, lead.ClaimOptions{CampaignID: campID, NodeID: "node-2", Limit: 10})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("second node should claim 0 (all in_flight), got %d", len(second))
	}
}

func TestClaimBatchRespectsNextEligibleAt(t *testing.T) {
	pool, tid, campID := setupLeads(t)
	repo := lead.NewRepo()
	ctx := context.Background()

	id := seedLead(t, pool, tid, campID, "+15550000001")

	future := time.Now().Add(1 * time.Hour)
	err := db.WithCtx(ctx, pool, db.Ctx{Role: "tenant_owner", TenantID: tid}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE leads SET next_eligible_at = $1, status = 'queued' WHERE id = $2`, future, id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	var claimed []lead.Lead
	err = db.WithCtx(ctx, pool, db.Ctx{Role: "tenant_owner", TenantID: tid}, func(tx pgx.Tx) error {
		var err error
		claimed, err = repo.ClaimBatchTx(ctx, tx, lead.ClaimOptions{CampaignID: campID, NodeID: "n", Limit: 10})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 0 {
		t.Fatalf("future-eligible lead should not be claimed, got %d", len(claimed))
	}
}

func TestReleaseExpiredLocks(t *testing.T) {
	pool, tid, campID := setupLeads(t)
	repo := lead.NewRepo()
	ctx := context.Background()

	id := seedLead(t, pool, tid, campID, "+15550000001")

	err := db.WithCtx(ctx, pool, db.Ctx{Role: "tenant_owner", TenantID: tid}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE leads SET status = 'in_flight', locked_by = 'gone', locked_until = now() - interval '1 minute'
			WHERE id = $1
		`, id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	var n int64
	err = db.WithCtx(ctx, pool, db.Ctx{Role: "tenant_owner", TenantID: tid}, func(tx pgx.Tx) error {
		var err error
		n, err = repo.ReleaseExpiredLocksTx(ctx, tx)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected to release 1 lead, got %d", n)
	}
}
