package leadimport

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"p1/engine/internal/db"
	"p1/engine/internal/lead"
)

// Runner kicks off background imports. One per process; safe to run goroutines off it.
type Runner struct {
	pool    *pgxpool.Pool
	storage *Storage
	repo    *Repo
	lRepo   *lead.Repo
	logger  *slog.Logger
}

func NewRunner(pool *pgxpool.Pool, storage *Storage, logger *slog.Logger) *Runner {
	return &Runner{
		pool:    pool,
		storage: storage,
		repo:    NewRepo(),
		lRepo:   lead.NewRepo(),
		logger:  logger,
	}
}

// Run launches the import in a fresh goroutine. The caller does not block.
func (r *Runner) Run(jobID, tenantID int64, listID *int64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := r.runJob(ctx, jobID, tenantID, listID); err != nil {
			r.logger.Error("import job failed", "job_id", jobID, "tenant", tenantID, "err", err)
			_ = db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin", TenantID: tenantID}, func(tx pgx.Tx) error {
				return r.repo.MarkFinishedTx(ctx, tx, jobID, StatusFailed, err.Error(), 0, 0)
			})
		}
	}()
}

func (r *Runner) runJob(ctx context.Context, jobID, tenantID int64, listID *int64) error {
	var job Job
	if err := db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin", TenantID: tenantID}, func(tx pgx.Tx) error {
		var err error
		job, err = r.repo.GetTx(ctx, tx, jobID)
		return err
	}); err != nil {
		return fmt.Errorf("load job: %w", err)
	}

	f, err := r.storage.Open(tenantID, job.FileKey)
	if err != nil {
		return fmt.Errorf("open csv: %w", err)
	}
	defer func() { _ = f.Close() }()

	cr := csv.NewReader(f)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err == io.EOF {
		return errors.New("csv is empty")
	}
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}

	mapping := buildColumnMap(header, job.ColumnMap)

	// For mid-sized csvs we just ReadAll the rows up-front so we know the total.
	// 25 MB cap on upload bounds this in practice.
	rest, err := cr.ReadAll()
	if err != nil {
		return fmt.Errorf("scan csv: %w", err)
	}
	total := len(rest)

	mapBytes, _ := json.Marshal(mapping.exportable())
	if err := db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin", TenantID: tenantID}, func(tx pgx.Tx) error {
		return r.repo.MarkRunningTx(ctx, tx, jobID, total, mapBytes)
	}); err != nil {
		return fmt.Errorf("mark running: %w", err)
	}

	processed, errs := 0, 0
	abortCheck := time.NewTicker(2 * time.Second)
	defer abortCheck.Stop()
	lastFlush := time.Now()

	for _, row := range rest {
		select {
		case <-abortCheck.C:
			st, e := r.checkStatus(ctx, tenantID, jobID)
			if e == nil && st == StatusAborted {
				return r.finalize(ctx, tenantID, jobID, StatusAborted, "", processed, errs)
			}
		default:
		}

		l, valid := mapping.toLead(row, tenantID, listID)
		if !valid {
			errs++
			processed++
			continue
		}

		if err := db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin", TenantID: tenantID}, func(tx pgx.Tx) error {
			_, e := r.lRepo.CreateLeadTx(ctx, tx, l)
			if e != nil && isUniqueViolation(e) {
				return nil // dedup ok
			}
			return e
		}); err != nil {
			errs++
		}
		processed++

		if time.Since(lastFlush) > 500*time.Millisecond {
			_ = db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin", TenantID: tenantID}, func(tx pgx.Tx) error {
				return r.repo.UpdateProgressTx(ctx, tx, jobID, processed, errs)
			})
			lastFlush = time.Now()
		}
	}

	return r.finalize(ctx, tenantID, jobID, StatusCompleted, "", processed, errs)
}

func (r *Runner) finalize(ctx context.Context, tenantID, jobID int64, status Status, lastErr string, processed, errs int) error {
	return db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin", TenantID: tenantID}, func(tx pgx.Tx) error {
		return r.repo.MarkFinishedTx(ctx, tx, jobID, status, lastErr, processed, errs)
	})
}

func (r *Runner) checkStatus(ctx context.Context, tenantID, jobID int64) (Status, error) {
	var st Status
	err := db.WithCtx(ctx, r.pool, db.Ctx{Role: "super_admin", TenantID: tenantID}, func(tx pgx.Tx) error {
		var e error
		st, e = r.repo.StatusTx(ctx, tx, jobID)
		return e
	})
	return st, err
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "23505")
}

// columnMap maps CSV column indices to lead fields.
type columnMap struct {
	Phone     int
	FirstName int
	LastName  int
	Email     int
	StateCode int
	Timezone  int
	Header    []string // raw header for round-tripping in column_map JSON
}

func (m columnMap) exportable() map[string]any {
	out := map[string]any{}
	if m.Phone >= 0 {
		out["phone_e164"] = m.Header[m.Phone]
	}
	if m.FirstName >= 0 {
		out["first_name"] = m.Header[m.FirstName]
	}
	if m.LastName >= 0 {
		out["last_name"] = m.Header[m.LastName]
	}
	if m.Email >= 0 {
		out["email"] = m.Header[m.Email]
	}
	if m.StateCode >= 0 {
		out["state_code"] = m.Header[m.StateCode]
	}
	if m.Timezone >= 0 {
		out["timezone"] = m.Header[m.Timezone]
	}
	return out
}

func (m columnMap) toLead(row []string, tenantID int64, listID *int64) (lead.Lead, bool) {
	get := func(idx int) string {
		if idx < 0 || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}
	rawPhone := get(m.Phone)
	phone, ok := normalizePhone(rawPhone)
	if !ok {
		return lead.Lead{}, false
	}
	l := lead.Lead{
		TenantID:  tenantID,
		ListID:    listID,
		PhoneE164: phone,
	}
	if v := get(m.FirstName); v != "" {
		l.FirstName = &v
	}
	if v := get(m.LastName); v != "" {
		l.LastName = &v
	}
	if v := get(m.Email); v != "" {
		l.Email = &v
	}
	if v := get(m.StateCode); v != "" {
		v = strings.ToUpper(v)
		if len(v) == 2 {
			l.StateCode = &v
		}
	}
	if v := get(m.Timezone); v != "" {
		l.Timezone = &v
	}
	return l, true
}

var phoneStrip = regexp.MustCompile(`[^0-9+]`)

func normalizePhone(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	s := phoneStrip.ReplaceAllString(raw, "")
	if strings.HasPrefix(s, "+") {
		if lead.ValidE164(s) {
			return s, true
		}
		return "", false
	}
	digits := s
	switch {
	case len(digits) == 11 && strings.HasPrefix(digits, "1"):
		s = "+" + digits
	case len(digits) == 10:
		s = "+1" + digits
	default:
		return "", false
	}
	if lead.ValidE164(s) {
		return s, true
	}
	return "", false
}

// buildColumnMap inspects the header row and any pre-supplied mapping from
// the job row. If the JSON map carries explicit header names (e.g.
// {"phone_e164": "Phone Number"}), we honor them. Otherwise we auto-detect
// by common header aliases.
func buildColumnMap(header []string, suppliedJSON json.RawMessage) columnMap {
	m := columnMap{
		Phone:     -1,
		FirstName: -1,
		LastName:  -1,
		Email:     -1,
		StateCode: -1,
		Timezone:  -1,
		Header:    header,
	}

	var supplied map[string]string
	_ = json.Unmarshal(suppliedJSON, &supplied)

	indexOf := func(name string) int {
		want := strings.ToLower(strings.TrimSpace(name))
		for i, h := range header {
			if strings.ToLower(strings.TrimSpace(h)) == want {
				return i
			}
		}
		return -1
	}

	pick := func(field string, aliases ...string) int {
		if v, ok := supplied[field]; ok && v != "" {
			if i := indexOf(v); i >= 0 {
				return i
			}
		}
		for _, a := range aliases {
			if i := indexOf(a); i >= 0 {
				return i
			}
		}
		return -1
	}

	m.Phone = pick("phone_e164", "phone", "phone_number", "phone number", "phonenumber", "mobile", "cell", "tel")
	m.FirstName = pick("first_name", "firstname", "first name", "first", "given_name", "given name", "fname")
	m.LastName = pick("last_name", "lastname", "last name", "last", "surname", "family_name", "lname")
	m.Email = pick("email", "email_address", "email address", "e-mail")
	m.StateCode = pick("state_code", "state", "st")
	m.Timezone = pick("timezone", "tz", "time_zone", "time zone")

	return m
}
