package script

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

const fields = `id, tenant_id, name, description, type, body, transfer_to, tags, created_at, updated_at`

func scan(row pgx.Row, s *Script) error {
	return row.Scan(&s.ID, &s.TenantID, &s.Name, &s.Description, &s.Type, &s.Body, &s.TransferTo, &s.Tags, &s.CreatedAt, &s.UpdatedAt)
}

func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, s Script) (Script, error) {
	if s.Tags == nil {
		s.Tags = []string{}
	}
	var out Script
	row := tx.QueryRow(ctx, `
		INSERT INTO scripts (tenant_id, name, description, type, body, transfer_to, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+fields,
		s.TenantID, s.Name, s.Description, s.Type, s.Body, s.TransferTo, s.Tags,
	)
	return out, scan(row, &out)
}

func (r *Repo) GetTx(ctx context.Context, tx pgx.Tx, id int64) (Script, error) {
	var out Script
	row := tx.QueryRow(ctx, `SELECT `+fields+` FROM scripts WHERE id = $1`, id)
	err := scan(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) ListTx(ctx context.Context, tx pgx.Tx) ([]Script, error) {
	rows, err := tx.Query(ctx, `SELECT `+fields+` FROM scripts ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Script
	for rows.Next() {
		var s Script
		if err := scan(rows, &s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

type UpdatePatch struct {
	Name          string
	Description   *string
	Type          string
	Body          *string
	TransferTo    *string
	SetTransferTo bool
	Tags          []string
	SetTags       bool
}

func (r *Repo) UpdateTx(ctx context.Context, tx pgx.Tx, id int64, patch UpdatePatch) (Script, error) {
	var tagsPtr *[]string
	if patch.SetTags {
		tags := patch.Tags
		if tags == nil {
			tags = []string{}
		}
		tagsPtr = &tags
	}
	var out Script
	row := tx.QueryRow(ctx, `
		UPDATE scripts SET
		  name        = COALESCE(NULLIF($1, ''), name),
		  description = COALESCE($2, description),
		  type        = COALESCE(NULLIF($3, ''), type),
		  body        = COALESCE($4, body),
		  transfer_to = CASE WHEN $5::boolean THEN $6 ELSE transfer_to END,
		  tags        = COALESCE($7, tags),
		  updated_at  = now()
		WHERE id = $8
		RETURNING `+fields,
		patch.Name, patch.Description, patch.Type, patch.Body, patch.SetTransferTo, patch.TransferTo, tagsPtr, id,
	)
	err := scan(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) DeleteTx(ctx context.Context, tx pgx.Tx, id int64) error {
	tag, err := tx.Exec(ctx, `DELETE FROM scripts WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
