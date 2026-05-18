package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool = pgxpool.Pool

func Open(ctx context.Context, url string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

type Ctx struct {
	TenantID int64
	UserID   int64
	Role     string
}

func WithCtx(ctx context.Context, pool *Pool, c Ctx, fn func(pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if c.TenantID != 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.tenant_id = '%d'", c.TenantID)); err != nil {
			return err
		}
	}
	if c.UserID != 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.user_id = '%d'", c.UserID)); err != nil {
			return err
		}
	}
	if c.Role != "" {
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.role = '%s'", c.Role)); err != nil {
			return err
		}
	}

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
