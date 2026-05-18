package campaign

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("campaign not found")

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

const fields = `id, tenant_id, name, mode, status, dial_ratio, max_abandon_pct,
  prompt_audio, transfer_dest, caller_id_pool, retry_policy, calling_hours,
  tz_strategy, dnc_list_ids, created_at, updated_at`

func scanCampaign(row pgx.Row, c *Campaign) error {
	return row.Scan(
		&c.ID, &c.TenantID, &c.Name, &c.Mode, &c.Status, &c.DialRatio, &c.MaxAbandonPct,
		&c.PromptAudio, &c.TransferDest, &c.CallerIDPool, &c.RetryPolicy, &c.CallingHours,
		&c.TZStrategy, &c.DNCListIDs, &c.CreatedAt, &c.UpdatedAt,
	)
}

func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, c Campaign) (Campaign, error) {
	if c.CallerIDPool == nil {
		c.CallerIDPool = json.RawMessage(`[]`)
	}
	if c.RetryPolicy == nil {
		c.RetryPolicy = json.RawMessage(`{}`)
	}
	if c.CallingHours == nil {
		c.CallingHours = json.RawMessage(`{}`)
	}
	if c.TZStrategy == "" {
		c.TZStrategy = "lead_local"
	}
	if c.DialRatio == 0 {
		c.DialRatio = 1.0
	}
	if c.MaxAbandonPct == 0 {
		c.MaxAbandonPct = 3.0
	}
	if c.DNCListIDs == nil {
		c.DNCListIDs = []int64{}
	}

	var out Campaign
	row := tx.QueryRow(ctx, `
		INSERT INTO campaigns
		  (tenant_id, name, mode, status, dial_ratio, max_abandon_pct,
		   prompt_audio, transfer_dest, caller_id_pool, retry_policy, calling_hours,
		   tz_strategy, dnc_list_ids)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING `+fields, c.TenantID, c.Name, c.Mode, c.Status, c.DialRatio, c.MaxAbandonPct,
		c.PromptAudio, c.TransferDest, c.CallerIDPool, c.RetryPolicy, c.CallingHours,
		c.TZStrategy, c.DNCListIDs)
	return out, scanCampaign(row, &out)
}

func (r *Repo) GetTx(ctx context.Context, tx pgx.Tx, id int64) (Campaign, error) {
	var out Campaign
	row := tx.QueryRow(ctx, `SELECT `+fields+` FROM campaigns WHERE id = $1`, id)
	err := scanCampaign(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) ListTx(ctx context.Context, tx pgx.Tx) ([]Campaign, error) {
	rows, err := tx.Query(ctx, `SELECT `+fields+` FROM campaigns ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Campaign
	for rows.Next() {
		var c Campaign
		if err := scanCampaign(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repo) UpdateTx(ctx context.Context, tx pgx.Tx, id int64, patch UpdatePatch) (Campaign, error) {
	var out Campaign
	row := tx.QueryRow(ctx, `
		UPDATE campaigns SET
		  name = COALESCE(NULLIF($1, ''), name),
		  status = COALESCE(NULLIF($2, ''), status),
		  mode = COALESCE(NULLIF($3, ''), mode),
		  dial_ratio = COALESCE($4, dial_ratio),
		  max_abandon_pct = COALESCE($5, max_abandon_pct),
		  prompt_audio = COALESCE($6, prompt_audio),
		  transfer_dest = COALESCE($7, transfer_dest),
		  caller_id_pool = COALESCE($8, caller_id_pool),
		  retry_policy = COALESCE($9, retry_policy),
		  calling_hours = COALESCE($10, calling_hours),
		  tz_strategy = COALESCE(NULLIF($11, ''), tz_strategy),
		  updated_at = now()
		WHERE id = $12
		RETURNING `+fields,
		patch.Name, patch.Status, patch.Mode, patch.DialRatio, patch.MaxAbandonPct,
		patch.PromptAudio, patch.TransferDest, patch.CallerIDPool, patch.RetryPolicy, patch.CallingHours,
		patch.TZStrategy, id)
	err := scanCampaign(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

type UpdatePatch struct {
	Name          string
	Status        string
	Mode          string
	DialRatio     *float64
	MaxAbandonPct *float64
	PromptAudio   *string
	TransferDest  *string
	CallerIDPool  json.RawMessage
	RetryPolicy   json.RawMessage
	CallingHours  json.RawMessage
	TZStrategy    string
}
