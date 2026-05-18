package fsm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"p1/engine/internal/db"
	"p1/engine/internal/fsm"
	"p1/engine/internal/tenant"
	"p1/engine/internal/testutil"
)

func setup(t *testing.T) (*pgxpool.Pool, int64) {
	pool := testutil.TestPool(t)
	tn, err := tenant.NewRepo(pool).CreateTenantAsSuperAdmin(context.Background(), tenant.Tenant{
		Slug: "fsm", Name: "FSM", SIPDomain: "fsm.sip",
	})
	if err != nil {
		t.Fatal(err)
	}
	return pool, tn.ID
}

func tenantCtx(tid int64) db.Ctx {
	return db.Ctx{Role: "tenant_owner", TenantID: tid}
}

func TestCreateAndGetCall(t *testing.T) {
	pool, tid := setup(t)
	r := fsm.NewRepo()
	ctx := context.Background()
	callUUID := uuid.NewString()

	var created fsm.Call
	must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		var err error
		created, err = r.CreateTx(ctx, tx, fsm.CreateInput{
			UUID: callUUID, TenantID: tid, DialedNumber: "+15551234567",
		})
		return err
	}))
	if created.State != fsm.StateQueued {
		t.Fatalf("initial state: %s", created.State)
	}
	if created.Version != 1 {
		t.Fatalf("initial version: %d", created.Version)
	}

	var got fsm.Call
	must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		var err error
		got, err = r.GetTx(ctx, tx, callUUID)
		return err
	}))
	if got.UUID != callUUID {
		t.Fatalf("uuid mismatch")
	}
}

func TestTransitionHappyPath(t *testing.T) {
	pool, tid := setup(t)
	r := fsm.NewRepo()
	ctx := context.Background()
	callUUID := uuid.NewString()

	var c fsm.Call
	must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		var err error
		c, err = r.CreateTx(ctx, tx, fsm.CreateInput{UUID: callUUID, TenantID: tid, DialedNumber: "+15551234567"})
		return err
	}))

	steps := []fsm.State{
		fsm.StateOriginating, fsm.StateAnswered, fsm.StateAMDRunning,
		fsm.StateHuman, fsm.StatePlayingMsg, fsm.StateCompleted,
	}
	for _, next := range steps {
		var nextCall fsm.Call
		must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
			var err error
			nextCall, err = r.TransitionTx(ctx, tx, fsm.TransitionInput{
				UUID: callUUID, FromVersion: c.Version, To: next, Reason: "test",
				StampAnswered: next == fsm.StateAnswered,
			})
			return err
		}))
		if nextCall.State != next {
			t.Fatalf("expected state %s, got %s", next, nextCall.State)
		}
		if nextCall.Version != c.Version+1 {
			t.Fatalf("expected version %d, got %d", c.Version+1, nextCall.Version)
		}
		c = nextCall
	}

	if c.EndedAt == nil {
		t.Fatal("ended_at should be set after terminal transition")
	}
	if c.AnsweredAt == nil {
		t.Fatal("answered_at should be set after StampAnswered transition")
	}
}

func TestVersionConflictRejected(t *testing.T) {
	pool, tid := setup(t)
	r := fsm.NewRepo()
	ctx := context.Background()
	callUUID := uuid.NewString()

	var c fsm.Call
	must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		var err error
		c, err = r.CreateTx(ctx, tx, fsm.CreateInput{UUID: callUUID, TenantID: tid, DialedNumber: "+15551234567"})
		return err
	}))

	must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		_, err := r.TransitionTx(ctx, tx, fsm.TransitionInput{UUID: callUUID, FromVersion: c.Version, To: fsm.StateOriginating})
		return err
	}))

	err := db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		_, err := r.TransitionTx(ctx, tx, fsm.TransitionInput{UUID: callUUID, FromVersion: c.Version, To: fsm.StateRinging})
		return err
	})
	if !errors.Is(err, fsm.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
}

func TestInvalidTransitionRejected(t *testing.T) {
	pool, tid := setup(t)
	r := fsm.NewRepo()
	ctx := context.Background()
	callUUID := uuid.NewString()

	var c fsm.Call
	must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		var err error
		c, err = r.CreateTx(ctx, tx, fsm.CreateInput{UUID: callUUID, TenantID: tid, DialedNumber: "+15551234567"})
		return err
	}))

	err := db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		_, err := r.TransitionTx(ctx, tx, fsm.TransitionInput{UUID: callUUID, FromVersion: c.Version, To: fsm.StateBridged})
		return err
	})
	if !errors.Is(err, fsm.ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestCallEventsAppendedOnEachTransition(t *testing.T) {
	pool, tid := setup(t)
	r := fsm.NewRepo()
	ctx := context.Background()
	callUUID := uuid.NewString()

	var c fsm.Call
	must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		var err error
		c, err = r.CreateTx(ctx, tx, fsm.CreateInput{UUID: callUUID, TenantID: tid, DialedNumber: "+15551234567"})
		return err
	}))
	for _, next := range []fsm.State{fsm.StateOriginating, fsm.StateAnswered, fsm.StateCompleted} {
		must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
			var err error
			c, err = r.TransitionTx(ctx, tx, fsm.TransitionInput{UUID: callUUID, FromVersion: c.Version, To: next, Reason: "step"})
			return err
		}))
	}

	var count int
	must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM call_events WHERE call_uuid = $1`, callUUID).Scan(&count)
	}))
	if count != 4 {
		t.Fatalf("expected 4 events (created + 3 transitions), got %d", count)
	}
}

func TestListActiveExcludesTerminal(t *testing.T) {
	pool, tid := setup(t)
	r := fsm.NewRepo()
	ctx := context.Background()

	active := uuid.NewString()
	done := uuid.NewString()

	must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		if _, err := r.CreateTx(ctx, tx, fsm.CreateInput{UUID: active, TenantID: tid, DialedNumber: "+15551000001"}); err != nil {
			return err
		}
		_, err := r.CreateTx(ctx, tx, fsm.CreateInput{UUID: done, TenantID: tid, DialedNumber: "+15551000002"})
		return err
	}))

	must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		c, err := r.GetTx(ctx, tx, done)
		if err != nil {
			return err
		}
		_, err = r.TransitionTx(ctx, tx, fsm.TransitionInput{UUID: done, FromVersion: c.Version, To: fsm.StateFailed})
		return err
	}))

	var list []fsm.Call
	must(t, db.WithCtx(ctx, pool, tenantCtx(tid), func(tx pgx.Tx) error {
		var err error
		list, err = r.ListActiveTx(ctx, tx, 100)
		return err
	}))
	if len(list) != 1 || list[0].UUID != active {
		t.Fatalf("expected only active call, got %d", len(list))
	}
}

func TestCrossTenantCannotReadCalls(t *testing.T) {
	pool, tA := setup(t)
	r := fsm.NewRepo()
	ctx := context.Background()

	tB, err := tenant.NewRepo(pool).CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "fsmB", Name: "B", SIPDomain: "fsmb.sip"})
	if err != nil {
		t.Fatal(err)
	}

	callUUID := uuid.NewString()
	must(t, db.WithCtx(ctx, pool, tenantCtx(tA), func(tx pgx.Tx) error {
		_, err := r.CreateTx(ctx, tx, fsm.CreateInput{UUID: callUUID, TenantID: tA, DialedNumber: "+15551234567"})
		return err
	}))

	err = db.WithCtx(ctx, pool, tenantCtx(tB.ID), func(tx pgx.Tx) error {
		_, err := r.GetTx(ctx, tx, callUUID)
		return err
	})
	if !errors.Is(err, fsm.ErrNotFound) {
		t.Fatalf("tenant B should not see tenant A's call: got %v", err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
