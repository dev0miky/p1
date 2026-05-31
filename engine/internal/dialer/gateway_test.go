package dialer

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"p1/engine/internal/db"
	"p1/engine/internal/gateway"
	"p1/engine/internal/testutil"
)

func TestResolveGatewayUsesDBActive(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo("")
	ctx := context.Background()

	var id int64
	if err := db.WithCtx(ctx, pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		g, err := repo.CreateTx(ctx, tx, gateway.Gateway{
			Name:          "carrierx",
			Proxy:         "sip.carrierx.com",
			Transport:     gateway.TransportUDP,
			Enabled:       true,
			ExpireSeconds: 3600,
			RetrySeconds:  30,
		})
		if err != nil {
			return err
		}
		id = g.ID
		return repo.ActivateTx(ctx, tx, id)
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	svc := &Service{
		cfg: Config{
			Pool:        pool,
			GatewayName: "loopback",
		},
		gwRepo: repo,
	}

	svc.resolveGateway(ctx)

	if got := svc.activeGateway; got != "carrierx" {
		t.Errorf("expected carrierx, got %q", got)
	}
}

func TestResolveGatewayFallsBackWhenNoneActive(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo("")
	ctx := context.Background()

	svc := &Service{
		cfg: Config{
			Pool:        pool,
			GatewayName: "loopback",
		},
		gwRepo: repo,
	}

	svc.resolveGateway(ctx)

	if got := svc.activeGateway; got != "loopback" {
		t.Errorf("expected loopback fallback, got %q", got)
	}
}
