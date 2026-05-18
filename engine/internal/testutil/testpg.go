package testutil

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"p1/engine/internal/db"
	"p1/engine/migrations"
)

func TestPool(t *testing.T) *pgxpool.Pool {
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
	if _, err := sqlDB.Exec(`DROP TABLE IF EXISTS goose_db_version, audit_log, call_events, call_state, opt_outs, dnc_entries, leads, lead_lists, campaigns, users, tenants CASCADE`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if _, err := sqlDB.Exec(`
		DO $$ BEGIN
			CREATE ROLE app_user LOGIN PASSWORD 'app_user_change_me' NOSUPERUSER;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;
	`); err != nil {
		t.Fatalf("role: %v", err)
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

	u, _ := url.Parse(connURL)
	u.User = url.UserPassword("app_user", "app_user_change_me")
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}
