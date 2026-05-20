package recording

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

const fields = `id, tenant_id, call_uuid, campaign_id, lead_id, file_key, sha256, size_bytes, duration_ms, retention_until, created_at`

func scan(row pgx.Row, r *Recording) error {
	return row.Scan(
		&r.ID, &r.TenantID, &r.CallUUID, &r.CampaignID, &r.LeadID,
		&r.FileKey, &r.SHA256, &r.SizeBytes, &r.DurationMS, &r.RetentionUntil, &r.CreatedAt,
	)
}

func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, rec Recording) (Recording, error) {
	var out Recording
	row := tx.QueryRow(ctx, `
		INSERT INTO recordings (tenant_id, call_uuid, campaign_id, lead_id, file_key, sha256, size_bytes, duration_ms, retention_until)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, call_uuid) DO NOTHING
		RETURNING `+fields,
		rec.TenantID, rec.CallUUID, rec.CampaignID, rec.LeadID, rec.FileKey,
		rec.SHA256, rec.SizeBytes, rec.DurationMS, rec.RetentionUntil,
	)
	err := scan(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrAlreadyExists
	}
	return out, err
}

func (r *Repo) GetTx(ctx context.Context, tx pgx.Tx, id int64) (Recording, error) {
	var out Recording
	row := tx.QueryRow(ctx, `SELECT `+fields+` FROM recordings WHERE id = $1`, id)
	err := scan(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

type ListFilter struct {
	CampaignID *int64
	LeadID     *int64
	Limit      int
}

func (r *Repo) ListTx(ctx context.Context, tx pgx.Tx, f ListFilter) ([]Recording, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := tx.Query(ctx, `
		SELECT `+fields+` FROM recordings
		WHERE ($1::bigint IS NULL OR campaign_id = $1)
		  AND ($2::bigint IS NULL OR lead_id = $2)
		ORDER BY created_at DESC
		LIMIT $3
	`, f.CampaignID, f.LeadID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Recording
	for rows.Next() {
		var rec Recording
		if err := scan(rows, &rec); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

var ErrAlreadyExists = errors.New("recording already exists")
