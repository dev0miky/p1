package lead

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type ClaimOptions struct {
	CampaignID int64
	NodeID     string
	LockFor    time.Duration
	Limit      int
	Constraint string // matches campaign.CallConstraint values
}

// callConstraintPredicate maps a call_constraint enum to the extra AND clause
// applied at lead claim time. Empty / "no_constraint" / unknown returns "".
// The clauses are static strings (no user input) so safe to inline.
func callConstraintPredicate(s string) string {
	switch s {
	case "only_answered", "only_human_answered":
		return " AND n_answered > 0"
	case "only_machine_answered":
		return " AND n_voicemail > 0"
	case "only_transfers":
		return " AND n_transferred > 0"
	case "only_failed_transfers":
		return " AND n_transferred > 0 AND n_transfer_completed = 0"
	case "only_successful_transfers":
		return " AND n_transfer_completed > 0"
	case "only_errors":
		return " AND n_error > 0"
	case "skip_answered", "skip_human_answered":
		return " AND n_answered = 0"
	case "skip_machine_answered":
		return " AND n_voicemail = 0"
	case "skip_successful_transfers":
		return " AND n_transfer_completed = 0"
	case "skip_errors":
		return " AND n_error = 0"
	}
	return ""
}

func (r *Repo) ClaimBatchTx(ctx context.Context, tx pgx.Tx, opts ClaimOptions) ([]Lead, error) {
	if opts.NodeID == "" {
		return nil, fmt.Errorf("ClaimBatch: NodeID required")
	}
	if opts.LockFor <= 0 {
		opts.LockFor = 2 * time.Minute
	}
	if opts.Limit <= 0 || opts.Limit > 1000 {
		opts.Limit = 50
	}

	rows, err := tx.Query(ctx, `
		WITH claim AS (
			SELECT id FROM leads
			WHERE campaign_id = $1
			  AND status IN ('new', 'queued')
			  AND (next_eligible_at IS NULL OR next_eligible_at <= now())
			  `+callConstraintPredicate(opts.Constraint)+`
			ORDER BY next_eligible_at NULLS FIRST, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE leads l
		SET status        = 'in_flight',
		    locked_by     = $3,
		    locked_until  = now() + ($4 || ' seconds')::interval,
		    attempts      = attempts + 1,
		    last_attempt_at = now(),
		    updated_at    = now()
		FROM claim
		WHERE l.id = claim.id
		RETURNING l.id, l.tenant_id, l.list_id, l.campaign_id, l.phone_e164, l.dial_destination,
		  l.first_name, l.last_name, l.email, l.timezone, l.state_code, l.status, l.attempts,
		  l.last_attempt_at, l.next_eligible_at,
		  l.custom_fields, l.created_at, l.updated_at,
		  l.n_calls, l.n_answered, l.n_ringed, l.n_voicemail, l.n_transferred, l.n_transfer_completed,
		  l.n_error, l.n_went_to_dnc, l.first_call_time, l.last_call_time`,
		opts.CampaignID, opts.Limit, opts.NodeID, fmt.Sprintf("%d", int(opts.LockFor.Seconds())))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Lead
	for rows.Next() {
		var l Lead
		if err := scanLead(rows, &l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *Repo) ReleaseExpiredLocksTx(ctx context.Context, tx pgx.Tx) (int64, error) {
	tag, err := tx.Exec(ctx, `
		UPDATE leads
		SET status = 'queued', locked_by = NULL, locked_until = NULL, updated_at = now()
		WHERE status = 'in_flight' AND locked_until IS NOT NULL AND locked_until < now()
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *Repo) MarkLeadOutcomeTx(ctx context.Context, tx pgx.Tx, leadID int64, terminalStatus string, retryAfter *time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE leads
		SET status            = $1,
		    locked_by         = NULL,
		    locked_until      = NULL,
		    next_eligible_at  = $2,
		    updated_at        = now()
		WHERE id = $3
	`, terminalStatus, retryAfter, leadID)
	return err
}
