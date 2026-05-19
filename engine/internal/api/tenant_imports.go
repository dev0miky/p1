package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"p1/engine/internal/audit"
	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/lead"
	"p1/engine/internal/leadimport"
	"p1/engine/internal/tenant"
)

type tenantImports struct {
	repo    *tenant.Repo
	lRepo   *lead.Repo
	iRepo   *leadimport.Repo
	storage *leadimport.Storage
	runner  *leadimport.Runner
}

type importJobResponse struct {
	ID            int64           `json:"id"`
	TenantID      int64           `json:"tenant_id"`
	ListID        *int64          `json:"list_id,omitempty"`
	Status        string          `json:"status"`
	CSVFilename   string          `json:"csv_filename"`
	ColumnMap     json.RawMessage `json:"column_map"`
	TotalRows     int             `json:"total_rows"`
	ProcessedRows int             `json:"processed_rows"`
	ErrorRows     int             `json:"error_rows"`
	LastError     *string         `json:"last_error,omitempty"`
	StartedAt     *string         `json:"started_at,omitempty"`
	FinishedAt    *string         `json:"finished_at,omitempty"`
	CreatedAt     string          `json:"created_at"`
}

func importJobToResponse(j leadimport.Job) importJobResponse {
	r := importJobResponse{
		ID:            j.ID,
		TenantID:      j.TenantID,
		ListID:        j.ListID,
		Status:        string(j.Status),
		CSVFilename:   j.CSVFilename,
		ColumnMap:     j.ColumnMap,
		TotalRows:     j.TotalRows,
		ProcessedRows: j.ProcessedRows,
		ErrorRows:     j.ErrorRows,
		LastError:     j.LastError,
		CreatedAt:     j.CreatedAt.Format(time.RFC3339),
	}
	if j.StartedAt != nil {
		s := j.StartedAt.Format(time.RFC3339)
		r.StartedAt = &s
	}
	if j.FinishedAt != nil {
		s := j.FinishedAt.Format(time.RFC3339)
		r.FinishedAt = &s
	}
	return r
}

const maxImportBytes = 25 << 20 // 25 MB

// POST /tenant/lists/:id/import — multipart upload
func (a *tenantImports) upload(w http.ResponseWriter, r *http.Request) {
	if a.runner == nil || a.storage == nil {
		writeError(w, http.StatusInternalServerError, "import not configured")
		return
	}
	listID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid list id")
		return
	}
	if err := r.ParseMultipartForm(maxImportBytes + (1 << 20)); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file required")
		return
	}
	defer func() { _ = file.Close() }()
	if header.Size > maxImportBytes {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("file too large (max %d bytes)", maxImportBytes))
		return
	}

	var colMapJSON json.RawMessage
	if raw := r.FormValue("column_map"); raw != "" {
		var tmp map[string]string
		if err := json.Unmarshal([]byte(raw), &tmp); err != nil {
			writeError(w, http.StatusBadRequest, "column_map must be a json object")
			return
		}
		b, _ := json.Marshal(tmp)
		colMapJSON = b
	}

	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}

	// Verify list belongs to tenant before staging the file.
	if err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		_, err := a.lRepo.GetListTx(r.Context(), tx, listID)
		return err
	}); err != nil {
		if errors.Is(err, lead.ErrNotFound) {
			writeError(w, http.StatusNotFound, "list not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "list lookup failed")
		return
	}

	fileKey := uuid.NewString() + ".csv"
	if _, err := a.storage.Write(tid, fileKey, io.LimitReader(file, maxImportBytes)); err != nil {
		slog.Error("import storage write failed", "err", err, "tenant", tid)
		writeError(w, http.StatusInternalServerError, "storage write failed")
		return
	}

	job := leadimport.Job{
		TenantID:    tid,
		ListID:      &listID,
		Status:      leadimport.StatusPending,
		CSVFilename: header.Filename,
		FileKey:     fileKey,
		ColumnMap:   colMapJSON,
	}
	var created leadimport.Job
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		created, err = a.iRepo.CreateTx(r.Context(), tx, job)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "lead_import_job",
			EntityID:   strconv.FormatInt(created.ID, 10),
			Action:     "create",
			After:      map[string]any{"list_id": listID, "filename": header.Filename, "size": header.Size},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		_ = a.storage.Delete(tid, fileKey)
		slog.Error("import job create failed", "err", err, "tenant", tid)
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}

	a.runner.Run(created.ID, tid, &listID)
	writeJSON(w, http.StatusAccepted, importJobToResponse(created))
}

// GET /tenant/lead-import-jobs
func (a *tenantImports) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var jobs []leadimport.Job
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		jobs, err = a.iRepo.ListTx(r.Context(), tx, 50)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]importJobResponse, len(jobs))
	for i, j := range jobs {
		out[i] = importJobToResponse(j)
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": out})
}

// GET /tenant/lead-import-jobs/:id
func (a *tenantImports) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var j leadimport.Job
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		j, err = a.iRepo.GetTx(r.Context(), tx, id)
		return err
	})
	if errors.Is(err, leadimport.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}
	writeJSON(w, http.StatusOK, importJobToResponse(j))
}

// POST /tenant/lead-import-jobs/:id/abort
func (a *tenantImports) abort(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		return a.iRepo.SetAbortedTx(r.Context(), tx, id)
	})
	if errors.Is(err, leadimport.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not active")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "abort failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
