package gateway

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"p1/engine/internal/db"
)

func RunStatusTicker(ctx context.Context, pool *db.Pool, repo *Repo, prov *Provisioner, logger *slog.Logger) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshAllStatuses(ctx, pool, repo, prov, logger)
		}
	}
}

func refreshAllStatuses(ctx context.Context, pool *db.Pool, repo *Repo, prov *Provisioner, logger *slog.Logger) {
	var gws []Gateway
	if err := db.WithCtx(ctx, pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		var e error
		gws, e = repo.ListTx(ctx, tx)
		return e
	}); err != nil {
		logger.Warn("gateway status sweep: list failed", "err", err)
		return
	}
	for _, g := range gws {
		if !g.Enabled {
			continue
		}
		status, err := prov.Status(ctx, g.Name)
		if err != nil {
			logger.Warn("gateway status sweep: status failed", "gateway", g.Name, "err", err)
			continue
		}
		if err := db.WithCtx(ctx, pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
			return repo.SetStatusTx(ctx, tx, g.Name, status)
		}); err != nil {
			logger.Warn("gateway status sweep: persist failed", "gateway", g.Name, "err", err)
		}
	}
}
