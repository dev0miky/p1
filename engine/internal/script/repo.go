package script

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

const fields = `id, tenant_id, name, description, type, body, transfer_to, external_agent_id,
  greeting_sound_id, pre_bridge_sound_id,
  bridge_digit, wait_timeout_ms, opt_out_digit,
  tags, created_at, updated_at`

func scan(row pgx.Row, s *Script) error {
	return row.Scan(
		&s.ID, &s.TenantID, &s.Name, &s.Description, &s.Type, &s.Body,
		&s.TransferTo, &s.ExternalAgentID,
		&s.GreetingSoundID, &s.PreBridgeSoundID,
		&s.BridgeDigit, &s.WaitTimeoutMS, &s.OptOutDigit,
		&s.Tags, &s.CreatedAt, &s.UpdatedAt,
	)
}

func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, s Script) (Script, error) {
	if s.Tags == nil {
		s.Tags = []string{}
	}
	if s.BridgeDigit == "" {
		s.BridgeDigit = "1"
	}
	if s.WaitTimeoutMS <= 0 {
		s.WaitTimeoutMS = 8000
	}
	var out Script
	row := tx.QueryRow(ctx, `
		INSERT INTO scripts (
		  tenant_id, name, description, type, body, transfer_to, external_agent_id,
		  greeting_sound_id, pre_bridge_sound_id,
		  bridge_digit, wait_timeout_ms, opt_out_digit,
		  tags
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING `+fields,
		s.TenantID, s.Name, s.Description, s.Type, s.Body, s.TransferTo, s.ExternalAgentID,
		s.GreetingSoundID, s.PreBridgeSoundID,
		s.BridgeDigit, s.WaitTimeoutMS, s.OptOutDigit,
		s.Tags,
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
	Name                string
	Description         *string
	Type                string
	Body                *string
	TransferTo          *string
	SetTransferTo       bool
	ExternalAgentID     *int64
	SetExternalAgentID  bool
	GreetingSoundID     *int64
	SetGreetingSoundID  bool
	PreBridgeSoundID    *int64
	SetPreBridgeSoundID bool
	BridgeDigit         string
	WaitTimeoutMS       *int
	OptOutDigit         *string
	SetOptOutDigit      bool
	Tags                []string
	SetTags             bool
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
		  name                = COALESCE(NULLIF($1, ''), name),
		  description         = COALESCE($2, description),
		  type                = COALESCE(NULLIF($3, ''), type),
		  body                = COALESCE($4, body),
		  transfer_to         = CASE WHEN $5::boolean  THEN $6  ELSE transfer_to         END,
		  external_agent_id   = CASE WHEN $7::boolean  THEN $8  ELSE external_agent_id   END,
		  greeting_sound_id   = CASE WHEN $9::boolean  THEN $10 ELSE greeting_sound_id   END,
		  pre_bridge_sound_id = CASE WHEN $11::boolean THEN $12 ELSE pre_bridge_sound_id END,
		  bridge_digit        = COALESCE(NULLIF($13, ''), bridge_digit),
		  wait_timeout_ms     = COALESCE($14, wait_timeout_ms),
		  opt_out_digit       = CASE WHEN $15::boolean THEN $16 ELSE opt_out_digit       END,
		  tags                = COALESCE($17, tags),
		  updated_at          = now()
		WHERE id = $18
		RETURNING `+fields,
		patch.Name, patch.Description, patch.Type, patch.Body,
		patch.SetTransferTo, patch.TransferTo,
		patch.SetExternalAgentID, patch.ExternalAgentID,
		patch.SetGreetingSoundID, patch.GreetingSoundID,
		patch.SetPreBridgeSoundID, patch.PreBridgeSoundID,
		patch.BridgeDigit, patch.WaitTimeoutMS,
		patch.SetOptOutDigit, patch.OptOutDigit,
		tagsPtr, id,
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
