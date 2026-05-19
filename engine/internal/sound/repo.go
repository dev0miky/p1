package sound

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

const fields = `id, tenant_id, name, description, file_key, mime_type, size_bytes,
  duration_ms, sha256, status, created_at, updated_at`

func scan(row pgx.Row, s *Sound) error {
	return row.Scan(
		&s.ID, &s.TenantID, &s.Name, &s.Description, &s.FileKey, &s.MimeType, &s.SizeBytes,
		&s.DurationMS, &s.SHA256, &s.Status, &s.CreatedAt, &s.UpdatedAt,
	)
}

func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, s Sound) (Sound, error) {
	if s.Status == "" {
		s.Status = StatusReady
	}
	var out Sound
	row := tx.QueryRow(ctx, `
		INSERT INTO sounds (tenant_id, name, description, file_key, mime_type, size_bytes, duration_ms, sha256, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+fields,
		s.TenantID, s.Name, s.Description, s.FileKey, s.MimeType, s.SizeBytes, s.DurationMS, s.SHA256, s.Status,
	)
	return out, scan(row, &out)
}

func (r *Repo) GetTx(ctx context.Context, tx pgx.Tx, id int64) (Sound, error) {
	var out Sound
	row := tx.QueryRow(ctx, `SELECT `+fields+` FROM sounds WHERE id = $1`, id)
	err := scan(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) ListTx(ctx context.Context, tx pgx.Tx) ([]Sound, error) {
	rows, err := tx.Query(ctx, `SELECT `+fields+` FROM sounds ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Sound
	for rows.Next() {
		var s Sound
		if err := scan(rows, &s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

type UpdatePatch struct {
	Name        string
	Description *string
}

func (r *Repo) UpdateTx(ctx context.Context, tx pgx.Tx, id int64, patch UpdatePatch) (Sound, error) {
	var out Sound
	row := tx.QueryRow(ctx, `
		UPDATE sounds SET
		  name        = COALESCE(NULLIF($1, ''), name),
		  description = COALESCE($2, description),
		  updated_at  = now()
		WHERE id = $3
		RETURNING `+fields, patch.Name, patch.Description, id)
	err := scan(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) DeleteTx(ctx context.Context, tx pgx.Tx, id int64) error {
	tag, err := tx.Exec(ctx, `DELETE FROM sounds WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
