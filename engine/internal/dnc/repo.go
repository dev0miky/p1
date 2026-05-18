package dnc

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("dnc entry not found")

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

const fields = `id, tenant_id, scope, state_code, phone_e164, source, reason, added_at, expires_at`

func scanEntry(row pgx.Row, e *Entry) error {
	return row.Scan(&e.ID, &e.TenantID, &e.Scope, &e.StateCode, &e.PhoneE164, &e.Source, &e.Reason, &e.AddedAt, &e.ExpiresAt)
}

func (r *Repo) AddInternalTx(ctx context.Context, tx pgx.Tx, e Entry) (Entry, error) {
	var out Entry
	row := tx.QueryRow(ctx, `
		INSERT INTO dnc_entries (tenant_id, scope, phone_e164, source, reason, expires_at)
		VALUES ($1, 'internal', $2, $3, $4, $5)
		RETURNING `+fields, e.TenantID, e.PhoneE164, e.Source, e.Reason, e.ExpiresAt)
	return out, scanEntry(row, &out)
}

func (r *Repo) RemoveInternalTx(ctx context.Context, tx pgx.Tx, tenantID int64, phone string) error {
	tag, err := tx.Exec(ctx, `DELETE FROM dnc_entries WHERE scope='internal' AND tenant_id=$1 AND phone_e164=$2`, tenantID, phone)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type ListFilter struct {
	Scope  string
	Search string
	Limit  int
	Offset int
}

func (r *Repo) ListTx(ctx context.Context, tx pgx.Tx, f ListFilter) ([]Entry, int, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	scope := nullStr(f.Scope)
	search := nullStr(f.Search)

	var total int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM dnc_entries
		WHERE ($1::text IS NULL OR scope = $1)
		  AND ($2::text IS NULL OR phone_e164 LIKE '%' || $2 || '%')
	`, scope, search).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := tx.Query(ctx, `
		SELECT `+fields+` FROM dnc_entries
		WHERE ($1::text IS NULL OR scope = $1)
		  AND ($2::text IS NULL OR phone_e164 LIKE '%' || $2 || '%')
		ORDER BY added_at DESC, id DESC
		LIMIT $3 OFFSET $4
	`, scope, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := scanEntry(rows, &e); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func (r *Repo) IsBlockedTx(ctx context.Context, tx pgx.Tx, phone string) (bool, string, error) {
	var scope string
	err := tx.QueryRow(ctx, `
		SELECT scope FROM dnc_entries
		WHERE phone_e164 = $1
		  AND (expires_at IS NULL OR expires_at > now())
		LIMIT 1
	`, phone).Scan(&scope)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, scope, nil
}

func (r *Repo) RecordOptOutTx(ctx context.Context, tx pgx.Tx, o OptOut) (OptOut, error) {
	var out OptOut
	row := tx.QueryRow(ctx, `
		INSERT INTO opt_outs (tenant_id, campaign_id, phone_e164, channel, evidence_ref)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, tenant_id, campaign_id, phone_e164, channel, evidence_ref, captured_at
	`, o.TenantID, o.CampaignID, o.PhoneE164, o.Channel, o.EvidenceRef)
	err := row.Scan(&out.ID, &out.TenantID, &out.CampaignID, &out.PhoneE164, &out.Channel, &out.EvidenceRef, &out.CapturedAt)
	return out, err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
