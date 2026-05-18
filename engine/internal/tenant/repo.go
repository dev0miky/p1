package tenant

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"p1/engine/internal/db"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) CreateTenantAsSuperAdmin(ctx context.Context, t Tenant) (Tenant, error) {
	var out Tenant
	err := db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO tenants (slug, name, sip_domain)
			VALUES ($1, $2, $3)
			RETURNING id, slug, name, sip_domain, status, created_at, updated_at
		`, t.Slug, t.Name, t.SIPDomain)
		return row.Scan(&out.ID, &out.Slug, &out.Name, &out.SIPDomain, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	})
	return out, err
}

func (r *Repo) GetTenantAsSuperAdmin(ctx context.Context, id int64) (Tenant, error) {
	var out Tenant
	err := db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, slug, name, sip_domain, status, created_at, updated_at
			FROM tenants WHERE id = $1
		`, id)
		err := row.Scan(&out.ID, &out.Slug, &out.Name, &out.SIPDomain, &out.Status, &out.CreatedAt, &out.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	return out, err
}

func (r *Repo) CreateUserAsSuperAdmin(ctx context.Context, u User) (User, error) {
	var out User
	err := db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO users (tenant_id, email, role, password_hash, totp_secret)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, tenant_id, email, role, password_hash, totp_secret, status, last_login_at, created_at, updated_at
		`, u.TenantID, u.Email, u.Role, u.PasswordHash, u.TOTPSecret)
		return row.Scan(&out.ID, &out.TenantID, &out.Email, &out.Role, &out.PasswordHash, &out.TOTPSecret, &out.Status, &out.LastLoginAt, &out.CreatedAt, &out.UpdatedAt)
	})
	return out, err
}

func (r *Repo) CountAllUsersAsSuperAdmin(ctx context.Context) (int, error) {
	var n int
	err := db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	})
	return n, err
}

var ErrNotFound = errors.New("not found")
