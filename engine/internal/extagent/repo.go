package extagent

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

const fields = `id, tenant_id, name, description, dial_string, tags, created_at, updated_at`

func scan(row pgx.Row, a *Agent) error {
	return row.Scan(&a.ID, &a.TenantID, &a.Name, &a.Description, &a.DialString, &a.Tags, &a.CreatedAt, &a.UpdatedAt)
}

func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, a Agent) (Agent, error) {
	if a.Tags == nil {
		a.Tags = []string{}
	}
	var out Agent
	row := tx.QueryRow(ctx, `
		INSERT INTO external_agents (tenant_id, name, description, dial_string, tags)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+fields,
		a.TenantID, a.Name, a.Description, a.DialString, a.Tags,
	)
	return out, scan(row, &out)
}

func (r *Repo) GetTx(ctx context.Context, tx pgx.Tx, id int64) (Agent, error) {
	var out Agent
	row := tx.QueryRow(ctx, `SELECT `+fields+` FROM external_agents WHERE id = $1`, id)
	err := scan(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) ListTx(ctx context.Context, tx pgx.Tx) ([]Agent, error) {
	rows, err := tx.Query(ctx, `SELECT `+fields+` FROM external_agents ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Agent
	for rows.Next() {
		var a Agent
		if err := scan(rows, &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type UpdatePatch struct {
	Name        string
	Description *string
	DialString  string
	Tags        []string
	SetTags     bool
}

func (r *Repo) UpdateTx(ctx context.Context, tx pgx.Tx, id int64, patch UpdatePatch) (Agent, error) {
	var tagsPtr *[]string
	if patch.SetTags {
		tags := patch.Tags
		if tags == nil {
			tags = []string{}
		}
		tagsPtr = &tags
	}
	var out Agent
	row := tx.QueryRow(ctx, `
		UPDATE external_agents SET
		  name        = COALESCE(NULLIF($1, ''), name),
		  description = COALESCE($2, description),
		  dial_string = COALESCE(NULLIF($3, ''), dial_string),
		  tags        = COALESCE($4, tags),
		  updated_at  = now()
		WHERE id = $5
		RETURNING `+fields,
		patch.Name, patch.Description, patch.DialString, tagsPtr, id,
	)
	err := scan(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) DeleteTx(ctx context.Context, tx pgx.Tx, id int64) error {
	tag, err := tx.Exec(ctx, `DELETE FROM external_agents WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
