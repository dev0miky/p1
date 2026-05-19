package fsm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound          = errors.New("call not found")
	ErrVersionConflict   = errors.New("call state version conflict")
	ErrInvalidTransition = errors.New("invalid state transition")
)

type Call struct {
	UUID          string
	TenantID      int64
	CampaignID    *int64
	CampaignRunNo int
	LeadID        *int64
	State         State
	Version       int
	DialedNumber  string
	CallerID      *string
	AMDResult     *string
	DTMF          *string
	HangupCause   *string
	Metadata      json.RawMessage
	StartedAt     time.Time
	AnsweredAt    *time.Time
	BridgedAt     *time.Time
	EndedAt       *time.Time
	UpdatedAt     time.Time
}

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

const callFields = `uuid, tenant_id, campaign_id, campaign_run_no, lead_id, state, version,
  dialed_number, caller_id, amd_result, dtmf, hangup_cause, metadata,
  started_at, answered_at, bridged_at, ended_at, updated_at`

func scanCall(row pgx.Row, c *Call) error {
	return row.Scan(
		&c.UUID, &c.TenantID, &c.CampaignID, &c.CampaignRunNo, &c.LeadID, &c.State, &c.Version,
		&c.DialedNumber, &c.CallerID, &c.AMDResult, &c.DTMF, &c.HangupCause, &c.Metadata,
		&c.StartedAt, &c.AnsweredAt, &c.BridgedAt, &c.EndedAt, &c.UpdatedAt,
	)
}

type CreateInput struct {
	UUID          string
	TenantID      int64
	CampaignID    *int64
	CampaignRunNo int
	LeadID        *int64
	DialedNumber  string
	CallerID      *string
	Metadata      json.RawMessage
}

func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, in CreateInput) (Call, error) {
	if in.Metadata == nil {
		in.Metadata = json.RawMessage(`{}`)
	}
	var out Call
	row := tx.QueryRow(ctx, `
		INSERT INTO call_state (uuid, tenant_id, campaign_id, campaign_run_no, lead_id, state, dialed_number, caller_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+callFields,
		in.UUID, in.TenantID, in.CampaignID, in.CampaignRunNo, in.LeadID, string(StateQueued),
		in.DialedNumber, in.CallerID, in.Metadata)
	if err := scanCall(row, &out); err != nil {
		return out, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO call_events (call_uuid, tenant_id, from_state, to_state, reason)
		VALUES ($1, $2, NULL, $3, 'created')
	`, in.UUID, in.TenantID, string(StateQueued)); err != nil {
		return out, err
	}
	return out, nil
}

func (r *Repo) GetTx(ctx context.Context, tx pgx.Tx, uuid string) (Call, error) {
	var out Call
	row := tx.QueryRow(ctx, `SELECT `+callFields+` FROM call_state WHERE uuid = $1`, uuid)
	err := scanCall(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

type TransitionInput struct {
	UUID          string
	FromVersion   int
	To            State
	Reason        string
	AMDResult     *string
	DTMF          *string
	HangupCause   *string
	StampAnswered bool
	StampBridged  bool
	StampEnded    bool
	Metadata      json.RawMessage
}

func (r *Repo) TransitionTx(ctx context.Context, tx pgx.Tx, in TransitionInput) (Call, error) {
	var current Call
	row := tx.QueryRow(ctx, `SELECT `+callFields+` FROM call_state WHERE uuid = $1 FOR UPDATE`, in.UUID)
	if err := scanCall(row, &current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return current, ErrNotFound
		}
		return current, err
	}
	if current.Version != in.FromVersion {
		return current, fmt.Errorf("%w: expected version %d, got %d", ErrVersionConflict, in.FromVersion, current.Version)
	}
	if !current.State.CanTransitionTo(in.To) {
		return current, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, current.State, in.To)
	}

	var updated Call
	row = tx.QueryRow(ctx, `
		UPDATE call_state SET
		  state         = $1,
		  version       = version + 1,
		  amd_result    = COALESCE($2, amd_result),
		  dtmf          = COALESCE($3, dtmf),
		  hangup_cause  = COALESCE($4, hangup_cause),
		  answered_at   = CASE WHEN $5 AND answered_at IS NULL THEN now() ELSE answered_at END,
		  bridged_at    = CASE WHEN $6 AND bridged_at  IS NULL THEN now() ELSE bridged_at  END,
		  ended_at      = CASE WHEN $7 AND ended_at    IS NULL THEN now() ELSE ended_at    END,
		  updated_at    = now()
		WHERE uuid = $8 AND version = $9
		RETURNING `+callFields,
		string(in.To), in.AMDResult, in.DTMF, in.HangupCause,
		in.StampAnswered, in.StampBridged, in.StampEnded || in.To.IsTerminal(),
		in.UUID, in.FromVersion)
	if err := scanCall(row, &updated); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return updated, ErrVersionConflict
		}
		return updated, err
	}

	meta := in.Metadata
	if meta == nil {
		meta = json.RawMessage(`{}`)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO call_events (call_uuid, tenant_id, from_state, to_state, reason, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, in.UUID, current.TenantID, string(current.State), string(in.To), nullStr(in.Reason), meta); err != nil {
		return updated, err
	}
	return updated, nil
}

func (r *Repo) ListRecentTx(ctx context.Context, tx pgx.Tx, limit int) ([]Call, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := tx.Query(ctx, `
		SELECT `+callFields+` FROM call_state
		ORDER BY started_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Call
	for rows.Next() {
		var c Call
		if err := scanCall(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type StateCount struct {
	State State
	Count int
}

func (r *Repo) CountByStateTx(ctx context.Context, tx pgx.Tx, sinceMinutes int) ([]StateCount, error) {
	rows, err := tx.Query(ctx, `
		SELECT state, COUNT(*) FROM call_state
		WHERE started_at > now() - ($1 || ' minutes')::interval
		GROUP BY state
	`, fmt.Sprintf("%d", sinceMinutes))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StateCount
	for rows.Next() {
		var sc StateCount
		if err := rows.Scan(&sc.State, &sc.Count); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (r *Repo) ListActiveTx(ctx context.Context, tx pgx.Tx, limit int) ([]Call, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	rows, err := tx.Query(ctx, `
		SELECT `+callFields+` FROM call_state
		WHERE state NOT IN ('completed','failed','no_answer','busy','voicemail','opt_out')
		ORDER BY started_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Call
	for rows.Next() {
		var c Call
		if err := scanCall(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
