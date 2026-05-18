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

func (r *Repo) FindUserForLogin(ctx context.Context, tenantSlug, email string) (User, error) {
	var u User
	err := db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		var row pgx.Row
		if tenantSlug == "" {
			row = tx.QueryRow(ctx, `
				SELECT u.id, u.tenant_id, u.email, u.role, u.password_hash, u.totp_secret, u.status, u.last_login_at, u.created_at, u.updated_at
				FROM users u
				WHERE u.tenant_id IS NULL AND u.email = $1
			`, email)
		} else {
			row = tx.QueryRow(ctx, `
				SELECT u.id, u.tenant_id, u.email, u.role, u.password_hash, u.totp_secret, u.status, u.last_login_at, u.created_at, u.updated_at
				FROM users u
				JOIN tenants t ON t.id = u.tenant_id
				WHERE t.slug = $1 AND u.email = $2
			`, tenantSlug, email)
		}
		err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Role, &u.PasswordHash, &u.TOTPSecret, &u.Status, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	return u, err
}

func (r *Repo) MarkUserLoggedIn(ctx context.Context, userID int64) error {
	return db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, userID)
		return err
	})
}

func (r *Repo) SetUserStatusAsSuperAdmin(ctx context.Context, email, status string) (User, error) {
	var u User
	err := db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE users SET status = $1, updated_at = now()
			WHERE email = $2
			RETURNING id, tenant_id, email, role, password_hash, totp_secret, status, last_login_at, created_at, updated_at
		`, status, email)
		return row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Role, &u.PasswordHash, &u.TOTPSecret, &u.Status, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	})
	return u, err
}

var ErrNotFound = errors.New("not found")
