package gateway

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Repo struct{ encKey string }

func NewRepo(encKey string) *Repo { return &Repo{encKey: encKey} }

const cols = `id, name, description, proxy, register, username,
  (password_enc IS NOT NULL) AS has_password, realm, from_user, from_domain,
  transport, expire_seconds, retry_seconds, caller_id_in_from, extra_params,
  enabled, is_active, register_status, register_status_at, created_at, updated_at`

func scan(row pgx.Row, g *Gateway, extra ...any) error {
	var raw []byte
	dest := []any{
		&g.ID, &g.Name, &g.Description, &g.Proxy, &g.Register, &g.Username,
		&g.HasPassword, &g.Realm, &g.FromUser, &g.FromDomain, &g.Transport,
		&g.ExpireSeconds, &g.RetrySeconds, &g.CallerIDInFrom, &raw,
		&g.Enabled, &g.IsActive, &g.RegisterStatus, &g.RegisterStatusAt,
		&g.CreatedAt, &g.UpdatedAt,
	}
	dest = append(dest, extra...)
	if err := row.Scan(dest...); err != nil {
		return err
	}
	if len(raw) > 0 {
		return json.Unmarshal(raw, &g.ExtraParams)
	}
	g.ExtraParams = map[string]string{}
	return nil
}

func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, g Gateway) (Gateway, error) {
	if g.ExtraParams == nil {
		g.ExtraParams = map[string]string{}
	}
	extra, _ := json.Marshal(g.ExtraParams)
	var out Gateway
	row := tx.QueryRow(ctx, `
		INSERT INTO gateways (name, description, proxy, register, username,
		  password_enc, realm, from_user, from_domain, transport, expire_seconds,
		  retry_seconds, caller_id_in_from, extra_params, enabled)
		VALUES ($1,$2,$3,$4,$5,
		  CASE WHEN $6::text IS NULL THEN NULL ELSE pgp_sym_encrypt($6, $15) END,
		  $7,$8,$9,$10,$11,$12,$13,$14,$16)
		RETURNING `+cols,
		g.Name, g.Description, g.Proxy, g.Register, g.Username,
		g.Password, g.Realm, g.FromUser, g.FromDomain, g.Transport,
		g.ExpireSeconds, g.RetrySeconds, g.CallerIDInFrom, extra, r.encKey, g.Enabled,
	)
	return out, scan(row, &out)
}

func (r *Repo) GetTx(ctx context.Context, tx pgx.Tx, id int64) (Gateway, error) {
	var out Gateway
	err := scan(tx.QueryRow(ctx, `SELECT `+cols+` FROM gateways WHERE id=$1`, id), &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) GetWithSecretTx(ctx context.Context, tx pgx.Tx, id int64) (Gateway, error) {
	g, err := r.GetTx(ctx, tx, id)
	if err != nil {
		return g, err
	}
	var pw *string
	if err := tx.QueryRow(ctx,
		`SELECT pgp_sym_decrypt(password_enc, $2) FROM gateways WHERE id=$1 AND password_enc IS NOT NULL`,
		id, r.encKey).Scan(&pw); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return g, err
	}
	g.Password = pw
	return g, nil
}

func (r *Repo) ListTx(ctx context.Context, tx pgx.Tx) ([]Gateway, error) {
	rows, err := tx.Query(ctx, `SELECT `+cols+` FROM gateways ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Gateway
	for rows.Next() {
		var g Gateway
		if err := scan(rows, &g); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *Repo) ListEnabledWithSecretTx(ctx context.Context, tx pgx.Tx) ([]Gateway, error) {
	rows, err := tx.Query(ctx, `SELECT `+cols+`, pgp_sym_decrypt(password_enc, $1) FROM gateways WHERE enabled ORDER BY id`, r.encKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Gateway
	for rows.Next() {
		var g Gateway
		var pw *string
		if err := scan(rows, &g, &pw); err != nil {
			return nil, err
		}
		g.Password = pw
		out = append(out, g)
	}
	return out, rows.Err()
}

type UpdatePatch struct {
	Description    *string
	Proxy          string
	Register       *bool
	Username       *string
	Password       *string
	Realm          *string
	FromUser       *string
	FromDomain     *string
	Transport      string
	ExpireSeconds  *int
	RetrySeconds   *int
	CallerIDInFrom *bool
	ExtraParams    map[string]string
	SetExtraParams bool
	Enabled        *bool
}

func (r *Repo) UpdateTx(ctx context.Context, tx pgx.Tx, id int64, p UpdatePatch) (Gateway, error) {
	var extra *[]byte
	if p.SetExtraParams {
		b, _ := json.Marshal(p.ExtraParams)
		extra = &b
	}
	var out Gateway
	row := tx.QueryRow(ctx, `
		UPDATE gateways SET
		  description       = COALESCE($1, description),
		  proxy             = COALESCE(NULLIF($2,''), proxy),
		  register          = COALESCE($3, register),
		  username          = COALESCE($4, username),
		  password_enc      = CASE WHEN $5::text IS NULL THEN password_enc
		                           ELSE pgp_sym_encrypt($5, $14) END,
		  realm             = COALESCE($6, realm),
		  from_user         = COALESCE($7, from_user),
		  from_domain       = COALESCE($8, from_domain),
		  transport         = COALESCE(NULLIF($9,''), transport),
		  expire_seconds    = COALESCE($10, expire_seconds),
		  retry_seconds     = COALESCE($11, retry_seconds),
		  caller_id_in_from = COALESCE($12, caller_id_in_from),
		  extra_params      = COALESCE($13, extra_params),
		  enabled           = COALESCE($15, enabled),
		  updated_at        = now()
		WHERE id=$16
		RETURNING `+cols,
		p.Description, p.Proxy, p.Register, p.Username, p.Password, p.Realm,
		p.FromUser, p.FromDomain, p.Transport, p.ExpireSeconds, p.RetrySeconds,
		p.CallerIDInFrom, extra, r.encKey, p.Enabled, id,
	)
	err := scan(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) DeleteTx(ctx context.Context, tx pgx.Tx, id int64) error {
	tag, err := tx.Exec(ctx, `DELETE FROM gateways WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) ActivateTx(ctx context.Context, tx pgx.Tx, id int64) error {
	if _, err := tx.Exec(ctx, `UPDATE gateways SET is_active=false WHERE is_active`); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE gateways SET is_active=true, updated_at=now() WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) SetStatusTx(ctx context.Context, tx pgx.Tx, name, status string) error {
	tag, err := tx.Exec(ctx,
		`UPDATE gateways SET register_status=$2, register_status_at=now() WHERE name=$1`, name, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) ActiveNameTx(ctx context.Context, tx pgx.Tx) (string, error) {
	var name string
	err := tx.QueryRow(ctx, `SELECT name FROM gateways WHERE is_active AND enabled LIMIT 1`).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return name, err
}
