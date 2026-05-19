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
  retry_policy, calling_hours, tz_strategy, run_no, call_constraint, created_at, updated_at`

func scanCampaign(row pgx.Row, c *Campaign) error {
	return row.Scan(
		&c.ID, &c.TenantID, &c.Name, &c.Mode, &c.Status, &c.DialRatio, &c.MaxAbandonPct,
		&c.RetryPolicy, &c.CallingHours, &c.TZStrategy, &c.RunNo, &c.CallConstraint, &c.CreatedAt, &c.UpdatedAt,
	)
}

func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, c Campaign) (Campaign, error) {
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

	var out Campaign
	row := tx.QueryRow(ctx, `
		INSERT INTO campaigns
		  (tenant_id, name, mode, status, dial_ratio, max_abandon_pct,
		   retry_policy, calling_hours, tz_strategy)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+fields, c.TenantID, c.Name, c.Mode, c.Status, c.DialRatio, c.MaxAbandonPct,
		c.RetryPolicy, c.CallingHours, c.TZStrategy)
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

func (r *Repo) ListActiveTx(ctx context.Context, tx pgx.Tx) ([]Campaign, error) {
	rows, err := tx.Query(ctx, `SELECT `+fields+` FROM campaigns WHERE status = 'active' ORDER BY id`)
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
	bumpRun := patch.Status == string(StatusActive)
	var out Campaign
	row := tx.QueryRow(ctx, `
		UPDATE campaigns SET
		  name = COALESCE(NULLIF($1, ''), name),
		  status = COALESCE(NULLIF($2, ''), status),
		  mode = COALESCE(NULLIF($3, ''), mode),
		  dial_ratio = COALESCE($4, dial_ratio),
		  max_abandon_pct = COALESCE($5, max_abandon_pct),
		  retry_policy = COALESCE($6, retry_policy),
		  calling_hours = COALESCE($7, calling_hours),
		  tz_strategy = COALESCE(NULLIF($8, ''), tz_strategy),
		  call_constraint = COALESCE(NULLIF($9, ''), call_constraint),
		  run_no = CASE WHEN $10::bool AND status != 'active' THEN run_no + 1 ELSE run_no END,
		  updated_at = now()
		WHERE id = $11
		RETURNING `+fields,
		patch.Name, patch.Status, patch.Mode, patch.DialRatio, patch.MaxAbandonPct,
		patch.RetryPolicy, patch.CallingHours, patch.TZStrategy, patch.CallConstraint, bumpRun, id)
	err := scanCampaign(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

type UpdatePatch struct {
	Name           string
	Status         string
	Mode           string
	DialRatio      *float64
	MaxAbandonPct  *float64
	RetryPolicy    json.RawMessage
	CallingHours   json.RawMessage
	TZStrategy     string
	CallConstraint string
}

// ---- resource attach helpers ----

const (
	SoundRoleGreeting      = "greeting"
	SoundRoleVoicemail     = "voicemail"
	SoundRoleHold          = "hold"
	SoundRoleWhisper       = "whisper"
	SoundRoleOptOutConfirm = "opt_out_confirm"
)

func ValidSoundRole(s string) bool {
	switch s {
	case SoundRoleGreeting, SoundRoleVoicemail, SoundRoleHold, SoundRoleWhisper, SoundRoleOptOutConfirm:
		return true
	}
	return false
}

type AttachedSound struct {
	SoundID    int64
	SoundName  string
	FileKey    string
	Role       string
	AttachedAt string
}

type AttachedScript struct {
	ScriptID   int64
	ScriptName string
	Type       string
	TransferTo *string
	AttachedAt string
}

type AttachedList struct {
	ListID     int64
	ListName   string
	LeadCount  int
	AttachedAt string
}

type AttachedCallerID struct {
	CallerIDID  int64
	Name        string
	E164Number  string
	Attestation string
	AttachedAt  string
}

func (r *Repo) AttachSoundTx(ctx context.Context, tx pgx.Tx, campaignID, soundID int64, role string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO campaign_sounds (campaign_id, sound_id, role) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, campaignID, soundID, role)
	return err
}

func (r *Repo) DetachSoundTx(ctx context.Context, tx pgx.Tx, campaignID, soundID int64, role string) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM campaign_sounds WHERE campaign_id=$1 AND sound_id=$2 AND role=$3
	`, campaignID, soundID, role)
	return err
}

func (r *Repo) ListAttachedSoundsTx(ctx context.Context, tx pgx.Tx, campaignID int64) ([]AttachedSound, error) {
	rows, err := tx.Query(ctx, `
		SELECT cs.sound_id, s.name, s.file_key, cs.role, cs.attached_at::text
		FROM campaign_sounds cs
		JOIN sounds s ON s.id = cs.sound_id
		WHERE cs.campaign_id = $1
		ORDER BY cs.role, cs.attached_at
	`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AttachedSound
	for rows.Next() {
		var a AttachedSound
		if err := rows.Scan(&a.SoundID, &a.SoundName, &a.FileKey, &a.Role, &a.AttachedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repo) AttachScriptTx(ctx context.Context, tx pgx.Tx, campaignID, scriptID int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO campaign_scripts (campaign_id, script_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, campaignID, scriptID)
	return err
}

func (r *Repo) DetachScriptTx(ctx context.Context, tx pgx.Tx, campaignID, scriptID int64) error {
	_, err := tx.Exec(ctx, `DELETE FROM campaign_scripts WHERE campaign_id=$1 AND script_id=$2`, campaignID, scriptID)
	return err
}

func (r *Repo) ListAttachedScriptsTx(ctx context.Context, tx pgx.Tx, campaignID int64) ([]AttachedScript, error) {
	rows, err := tx.Query(ctx, `
		SELECT cs.script_id, s.name, s.type, s.transfer_to, cs.attached_at::text
		FROM campaign_scripts cs
		JOIN scripts s ON s.id = cs.script_id
		WHERE cs.campaign_id = $1
		ORDER BY cs.attached_at
	`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AttachedScript
	for rows.Next() {
		var a AttachedScript
		if err := rows.Scan(&a.ScriptID, &a.ScriptName, &a.Type, &a.TransferTo, &a.AttachedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AttachListTx attaches a list and bulk-updates leads.campaign_id for every
// lead whose list_id matches. This way the dialer (which selects by
// leads.campaign_id) sees the new leads automatically.
func (r *Repo) AttachListTx(ctx context.Context, tx pgx.Tx, campaignID, listID int64) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO campaign_lists (campaign_id, list_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, campaignID, listID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		UPDATE leads SET campaign_id = $1, updated_at = now()
		WHERE list_id = $2 AND (campaign_id IS NULL OR campaign_id != $1)
	`, campaignID, listID)
	return err
}

// DetachListTx detaches a list. Leads whose list_id matches and were attached
// via this campaign get campaign_id cleared. Leads whose list_id is null but
// whose campaign_id is this campaign (manually attached) stay attached.
func (r *Repo) DetachListTx(ctx context.Context, tx pgx.Tx, campaignID, listID int64) error {
	if _, err := tx.Exec(ctx, `
		UPDATE leads SET campaign_id = NULL, updated_at = now()
		WHERE list_id = $1 AND campaign_id = $2
	`, listID, campaignID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM campaign_lists WHERE campaign_id=$1 AND list_id=$2`, campaignID, listID)
	return err
}

func (r *Repo) AttachCallerIDTx(ctx context.Context, tx pgx.Tx, campaignID, callerIDID int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO campaign_caller_ids (campaign_id, caller_id_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, campaignID, callerIDID)
	return err
}

func (r *Repo) DetachCallerIDTx(ctx context.Context, tx pgx.Tx, campaignID, callerIDID int64) error {
	_, err := tx.Exec(ctx, `DELETE FROM campaign_caller_ids WHERE campaign_id=$1 AND caller_id_id=$2`, campaignID, callerIDID)
	return err
}

func (r *Repo) ListAttachedCallerIDsTx(ctx context.Context, tx pgx.Tx, campaignID int64) ([]AttachedCallerID, error) {
	rows, err := tx.Query(ctx, `
		SELECT cci.caller_id_id, c.name, c.e164_number, c.attestation, cci.attached_at::text
		FROM campaign_caller_ids cci
		JOIN caller_ids c ON c.id = cci.caller_id_id
		WHERE cci.campaign_id = $1
		ORDER BY c.id
	`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AttachedCallerID
	for rows.Next() {
		var a AttachedCallerID
		if err := rows.Scan(&a.CallerIDID, &a.Name, &a.E164Number, &a.Attestation, &a.AttachedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repo) ListAttachedListsTx(ctx context.Context, tx pgx.Tx, campaignID int64) ([]AttachedList, error) {
	rows, err := tx.Query(ctx, `
		SELECT cl.list_id, ll.name, cl.attached_at::text,
		       COALESCE((SELECT COUNT(*)::int FROM leads WHERE list_id = ll.id), 0)
		FROM campaign_lists cl
		JOIN lead_lists ll ON ll.id = cl.list_id
		WHERE cl.campaign_id = $1
		ORDER BY cl.attached_at
	`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AttachedList
	for rows.Next() {
		var a AttachedList
		if err := rows.Scan(&a.ListID, &a.ListName, &a.AttachedAt, &a.LeadCount); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
