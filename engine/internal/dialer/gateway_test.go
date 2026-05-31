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
	if got := svc.activeGatewayPrefix; got != "" {
		t.Errorf("expected empty prefix, got %q", got)
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

func TestResolveGatewayPrefixCached(t *testing.T) {
	pool := testutil.TestPool(t)
	repo := gateway.NewRepo("")
	ctx := context.Background()

	var id int64
	if err := db.WithCtx(ctx, pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		g, err := repo.CreateTx(ctx, tx, gateway.Gateway{
			Name:          "voxtelesys",
			Proxy:         "sip.voxtelesys.net",
			Transport:     gateway.TransportUDP,
			DialPrefix:    "777",
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

	if got := svc.activeGateway; got != "voxtelesys" {
		t.Errorf("gateway: expected voxtelesys, got %q", got)
	}
	if got := svc.activeGatewayPrefix; got != "777" {
		t.Errorf("prefix: expected 777, got %q", got)
	}

	dialDest := svc.activeGatewayPrefix + "14242251420"
	if dialDest != "77714242251420" {
		t.Errorf("dial dest: expected 77714242251420, got %q", dialDest)
	}

	_ = id
}

func TestDialDestPrependsPrefix(t *testing.T) {
	svc := &Service{
		activeGatewayPrefix: "777",
	}
	dest := svc.activeGatewayPrefix + "14242251420"
	if dest != "77714242251420" {
		t.Errorf("expected 77714242251420, got %q", dest)
	}
}

func TestDialDestEmptyPrefix(t *testing.T) {
	svc := &Service{
		activeGatewayPrefix: "",
	}
	dest := svc.activeGatewayPrefix + "14242251420"
	if dest != "14242251420" {
		t.Errorf("expected 14242251420, got %q", dest)
	}
}
