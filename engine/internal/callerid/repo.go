package callerid

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

const fields = `id, tenant_id, name, e164_number, display_name, attestation,
  description, tags, created_at, updated_at`

func scan(row pgx.Row, c *CallerID) error {
	return row.Scan(
		&c.ID, &c.TenantID, &c.Name, &c.E164Number, &c.DisplayName, &c.Attestation,
		&c.Description, &c.Tags, &c.CreatedAt, &c.UpdatedAt,
	)
}

func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, c CallerID) (CallerID, error) {
	if c.Attestation == "" {
		c.Attestation = AttestationNone
	}
	if c.Tags == nil {
		c.Tags = []string{}
	}
	var out CallerID
	row := tx.QueryRow(ctx, `
		INSERT INTO caller_ids (tenant_id, name, e164_number, display_name, attestation, description, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+fields,
		c.TenantID, c.Name, c.E164Number, c.DisplayName, c.Attestation, c.Description, c.Tags,
	)
	return out, scan(row, &out)
}

func (r *Repo) GetTx(ctx context.Context, tx pgx.Tx, id int64) (CallerID, error) {
	var out CallerID
	row := tx.QueryRow(ctx, `SELECT `+fields+` FROM caller_ids WHERE id = $1`, id)
	err := scan(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) ListTx(ctx context.Context, tx pgx.Tx) ([]CallerID, error) {
	rows, err := tx.Query(ctx, `SELECT `+fields+` FROM caller_ids ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CallerID
	for rows.Next() {
		var c CallerID
		if err := scan(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListForCampaignTx returns the caller_ids attached to a campaign, ordered by id
// for deterministic rotation. Empty result is not an error.
func (r *Repo) ListForCampaignTx(ctx context.Context, tx pgx.Tx, campaignID int64) ([]CallerID, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+fields+`
		FROM caller_ids c
		JOIN campaign_caller_ids cci ON cci.caller_id_id = c.id
		WHERE cci.campaign_id = $1
		ORDER BY c.id
	`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CallerID
	for rows.Next() {
		var c CallerID
		if err := scan(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type UpdatePatch struct {
	Name        string
	DisplayName *string
	Attestation string
	Description *string
	Tags        []string
	SetTags     bool
}

func (r *Repo) UpdateTx(ctx context.Context, tx pgx.Tx, id int64, patch UpdatePatch) (CallerID, error) {
	var tagsPtr *[]string
	if patch.SetTags {
		tags := patch.Tags
		if tags == nil {
			tags = []string{}
		}
		tagsPtr = &tags
	}
	var out CallerID
	row := tx.QueryRow(ctx, `
		UPDATE caller_ids SET
		  name        = COALESCE(NULLIF($1, ''), name),
		  display_name = COALESCE($2, display_name),
		  attestation = COALESCE(NULLIF($3, ''), attestation),
		  description = COALESCE($4, description),
		  tags        = COALESCE($5, tags),
		  updated_at  = now()
		WHERE id = $6
		RETURNING `+fields,
		patch.Name, patch.DisplayName, patch.Attestation, patch.Description, tagsPtr, id,
	)
	err := scan(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) DeleteTx(ctx context.Context, tx pgx.Tx, id int64) error {
	tag, err := tx.Exec(ctx, `DELETE FROM caller_ids WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
