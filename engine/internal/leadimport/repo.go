package leadimport

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

const fields = `id, tenant_id, list_id, status, csv_filename, file_key, column_map,
  total_rows, processed_rows, error_rows, last_error,
  started_at, finished_at, created_at, updated_at`

func scan(row pgx.Row, j *Job) error {
	return row.Scan(
		&j.ID, &j.TenantID, &j.ListID, &j.Status, &j.CSVFilename, &j.FileKey, &j.ColumnMap,
		&j.TotalRows, &j.ProcessedRows, &j.ErrorRows, &j.LastError,
		&j.StartedAt, &j.FinishedAt, &j.CreatedAt, &j.UpdatedAt,
	)
}

func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, j Job) (Job, error) {
	if j.ColumnMap == nil {
		j.ColumnMap = json.RawMessage(`{}`)
	}
	if j.Status == "" {
		j.Status = StatusPending
	}
	var out Job
	row := tx.QueryRow(ctx, `
		INSERT INTO lead_import_jobs (tenant_id, list_id, status, csv_filename, file_key, column_map)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+fields,
		j.TenantID, j.ListID, j.Status, j.CSVFilename, j.FileKey, j.ColumnMap)
	return out, scan(row, &out)
}

func (r *Repo) GetTx(ctx context.Context, tx pgx.Tx, id int64) (Job, error) {
	var out Job
	row := tx.QueryRow(ctx, `SELECT `+fields+` FROM lead_import_jobs WHERE id = $1`, id)
	err := scan(row, &out)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

func (r *Repo) ListTx(ctx context.Context, tx pgx.Tx, limit int) ([]Job, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := tx.Query(ctx, `SELECT `+fields+` FROM lead_import_jobs ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		var j Job
		if err := scan(rows, &j); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *Repo) MarkRunningTx(ctx context.Context, tx pgx.Tx, id int64, totalRows int, columnMap json.RawMessage) error {
	if columnMap == nil {
		columnMap = json.RawMessage(`{}`)
	}
	_, err := tx.Exec(ctx, `
		UPDATE lead_import_jobs
		   SET status='running', started_at=now(), total_rows=$2, column_map=$3, updated_at=now()
		 WHERE id=$1
	`, id, totalRows, columnMap)
	return err
}

func (r *Repo) UpdateProgressTx(ctx context.Context, tx pgx.Tx, id int64, processed, errs int) error {
	_, err := tx.Exec(ctx, `
		UPDATE lead_import_jobs
		   SET processed_rows=$2, error_rows=$3, updated_at=now()
		 WHERE id=$1
	`, id, processed, errs)
	return err
}

func (r *Repo) MarkFinishedTx(ctx context.Context, tx pgx.Tx, id int64, status Status, lastErr string, processed, errs int) error {
	var lePtr *string
	if lastErr != "" {
		lePtr = &lastErr
	}
	_, err := tx.Exec(ctx, `
		UPDATE lead_import_jobs
		   SET status=$2, finished_at=now(), processed_rows=$3, error_rows=$4, last_error=$5, updated_at=now()
		 WHERE id=$1
	`, id, status, processed, errs, lePtr)
	return err
}

func (r *Repo) SetAbortedTx(ctx context.Context, tx pgx.Tx, id int64) error {
	tag, err := tx.Exec(ctx, `
		UPDATE lead_import_jobs
		   SET status='aborted', finished_at=COALESCE(finished_at, now()), updated_at=now()
		 WHERE id=$1 AND status IN ('pending', 'running')
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) StatusTx(ctx context.Context, tx pgx.Tx, id int64) (Status, error) {
	var s Status
	err := tx.QueryRow(ctx, `SELECT status FROM lead_import_jobs WHERE id=$1`, id).Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return s, err
}
