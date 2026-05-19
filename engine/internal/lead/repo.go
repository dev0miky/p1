package lead

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("lead not found")

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

const leadFields = `id, tenant_id, list_id, campaign_id, phone_e164, dial_destination,
  first_name, last_name, email, timezone, state_code, status, attempts,
  last_attempt_at, next_eligible_at, custom_fields, created_at, updated_at,
  n_calls, n_answered, n_ringed, n_voicemail, n_transferred, n_transfer_completed,
  n_error, n_went_to_dnc, first_call_time, last_call_time`

func scanLead(row pgx.Row, l *Lead) error {
	return row.Scan(
		&l.ID, &l.TenantID, &l.ListID, &l.CampaignID, &l.PhoneE164, &l.DialDestination,
		&l.FirstName, &l.LastName, &l.Email, &l.Timezone, &l.StateCode, &l.Status, &l.Attempts,
		&l.LastAttemptAt, &l.NextEligibleAt, &l.CustomFields, &l.CreatedAt, &l.UpdatedAt,
		&l.NCalls, &l.NAnswered, &l.NRinged, &l.NVoicemail, &l.NTransferred, &l.NTransferCompleted,
		&l.NError, &l.NWentToDNC, &l.FirstCallTime, &l.LastCallTime,
	)
}

func (r *Repo) CreateLeadTx(ctx context.Context, tx pgx.Tx, l Lead) (Lead, error) {
	if l.CustomFields == nil {
		l.CustomFields = json.RawMessage(`{}`)
	}
	if l.Status == "" {
		l.Status = StatusNew
	}
	var out Lead
	row := tx.QueryRow(ctx, `
		INSERT INTO leads
		  (tenant_id, list_id, campaign_id, phone_e164, dial_destination,
		   first_name, last_name, email, timezone, state_code, status, custom_fields)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+leadFields,
		l.TenantID, l.ListID, l.CampaignID, l.PhoneE164, l.DialDestination,
		l.FirstName, l.LastName, l.Email, l.Timezone, l.StateCode, l.Status, l.CustomFields)
	return out, scanLead(row, &out)
}

func (r *Repo) GetLeadTx(ctx context.Context, tx pgx.Tx, id int64) (Lead, error) {
	var out Lead
	row := tx.QueryRow(ctx, `SELECT `+leadFields+` FROM leads WHERE id = $1`, id)
	err := scanLead(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

type ListFilter struct {
	CampaignID *int64
	ListID     *int64
	Status     string
	Limit      int
	Offset     int
}

func (r *Repo) ListLeadsTx(ctx context.Context, tx pgx.Tx, f ListFilter) ([]Lead, int, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM leads
		WHERE ($1::bigint IS NULL OR campaign_id = $1)
		  AND ($2::bigint IS NULL OR list_id = $2)
		  AND ($3::text   IS NULL OR status = $3)
	`, f.CampaignID, f.ListID, nullStr(f.Status)).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := tx.Query(ctx, `
		SELECT `+leadFields+` FROM leads
		WHERE ($1::bigint IS NULL OR campaign_id = $1)
		  AND ($2::bigint IS NULL OR list_id = $2)
		  AND ($3::text   IS NULL OR status = $3)
		ORDER BY id
		LIMIT $4 OFFSET $5
	`, f.CampaignID, f.ListID, nullStr(f.Status), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Lead
	for rows.Next() {
		var l Lead
		if err := scanLead(rows, &l); err != nil {
			return nil, 0, err
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

type LeadUpdate struct {
	CampaignID         *int64
	SetCampaign        bool
	DialDestination    *string
	SetDialDestination bool
}

func (r *Repo) UpdateLeadTx(ctx context.Context, tx pgx.Tx, id int64, u LeadUpdate) (Lead, error) {
	if !u.SetCampaign && !u.SetDialDestination {
		return r.GetLeadTx(ctx, tx, id)
	}
	sets := []string{"updated_at = now()"}
	args := []any{id}
	if u.SetCampaign {
		args = append(args, u.CampaignID)
		sets = append(sets, fmt.Sprintf("campaign_id = $%d", len(args)))
	}
	if u.SetDialDestination {
		args = append(args, u.DialDestination)
		sets = append(sets, fmt.Sprintf("dial_destination = $%d", len(args)))
	}
	var out Lead
	row := tx.QueryRow(ctx, "UPDATE leads SET "+strings.Join(sets, ", ")+" WHERE id = $1 RETURNING "+leadFields, args...)
	err := scanLead(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) RedialLeadTx(ctx context.Context, tx pgx.Tx, id int64) (Lead, error) {
	var out Lead
	row := tx.QueryRow(ctx, `
		UPDATE leads
		   SET status = 'new', attempts = 0, next_eligible_at = NULL,
		       last_attempt_at = NULL, locked_by = NULL, locked_until = NULL,
		       updated_at = now()
		 WHERE id = $1
	 RETURNING `+leadFields, id)
	err := scanLead(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

type CounterDelta struct {
	NCalls             int
	NAnswered          int
	NRinged            int
	NVoicemail         int
	NTransferred       int
	NTransferCompleted int
	NError             int
	NWentToDNC         int
	SetCallTimes       bool // when true, sets last_call_time = now() and first_call_time = COALESCE(first_call_time, now())
}

func (d CounterDelta) IsZero() bool {
	return d.NCalls == 0 &&
		d.NAnswered == 0 &&
		d.NRinged == 0 &&
		d.NVoicemail == 0 &&
		d.NTransferred == 0 &&
		d.NTransferCompleted == 0 &&
		d.NError == 0 &&
		d.NWentToDNC == 0 &&
		!d.SetCallTimes
}

func (r *Repo) IncrementCountersTx(ctx context.Context, tx pgx.Tx, id int64, d CounterDelta) error {
	if d.IsZero() {
		return nil
	}
	sets := []string{}
	type bump struct {
		col   string
		delta int
	}
	for _, b := range []bump{
		{"n_calls", d.NCalls},
		{"n_answered", d.NAnswered},
		{"n_ringed", d.NRinged},
		{"n_voicemail", d.NVoicemail},
		{"n_transferred", d.NTransferred},
		{"n_transfer_completed", d.NTransferCompleted},
		{"n_error", d.NError},
		{"n_went_to_dnc", d.NWentToDNC},
	} {
		if b.delta != 0 {
			sets = append(sets, fmt.Sprintf("%s = %s + %d", b.col, b.col, b.delta))
		}
	}
	if d.SetCallTimes {
		sets = append(sets, "last_call_time = now()")
		sets = append(sets, "first_call_time = COALESCE(first_call_time, now())")
	}
	tag, err := tx.Exec(ctx, "UPDATE leads SET "+strings.Join(sets, ", ")+" WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) DeleteLeadTx(ctx context.Context, tx pgx.Tx, id int64) error {
	tag, err := tx.Exec(ctx, `DELETE FROM leads WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const listFields = `id, tenant_id, name, source, created_at, updated_at`

func scanList(row pgx.Row, l *List) error {
	return row.Scan(&l.ID, &l.TenantID, &l.Name, &l.Source, &l.CreatedAt, &l.UpdatedAt)
}

func (r *Repo) CreateListTx(ctx context.Context, tx pgx.Tx, l List) (List, error) {
	var out List
	row := tx.QueryRow(ctx, `
		INSERT INTO lead_lists (tenant_id, name, source)
		VALUES ($1, $2, $3)
		RETURNING `+listFields, l.TenantID, l.Name, l.Source)
	return out, scanList(row, &out)
}

func (r *Repo) ListListsTx(ctx context.Context, tx pgx.Tx) ([]List, error) {
	rows, err := tx.Query(ctx, `SELECT `+listFields+` FROM lead_lists ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []List
	for rows.Next() {
		var l List
		if err := scanList(rows, &l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
