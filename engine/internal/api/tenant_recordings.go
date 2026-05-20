package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/recording"
	"p1/engine/internal/tenant"
)

type tenantRecordings struct {
	repo  *tenant.Repo
	rRepo *recording.Repo
	store *recording.Store
}

type recordingResponse struct {
	ID             int64  `json:"id"`
	CallUUID       string `json:"call_uuid"`
	CampaignID     *int64 `json:"campaign_id,omitempty"`
	LeadID         *int64 `json:"lead_id,omitempty"`
	SizeBytes      int64  `json:"size_bytes"`
	DurationMS     *int   `json:"duration_ms,omitempty"`
	RetentionUntil string `json:"retention_until"`
	CreatedAt      string `json:"created_at"`
}

func recordingToResponse(r recording.Recording) recordingResponse {
	return recordingResponse{
		ID:             r.ID,
		CallUUID:       r.CallUUID,
		CampaignID:     r.CampaignID,
		LeadID:         r.LeadID,
		SizeBytes:      r.SizeBytes,
		DurationMS:     r.DurationMS,
		RetentionUntil: r.RetentionUntil.Format(time.RFC3339),
		CreatedAt:      r.CreatedAt.Format(time.RFC3339),
	}
}

func (a *tenantRecordings) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var f recording.ListFilter
	if v := r.URL.Query().Get("campaign_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.CampaignID = &id
		}
	}
	if v := r.URL.Query().Get("lead_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.LeadID = &id
		}
	}
	var list []recording.Recording
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var e error
		list, e = a.rRepo.ListTx(r.Context(), tx, f)
		return e
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]recordingResponse, len(list))
	for i, rec := range list {
		out[i] = recordingToResponse(rec)
	}
	writeJSON(w, http.StatusOK, map[string]any{"recordings": out})
}

func (a *tenantRecordings) url(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "recording storage not configured")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var rec recording.Recording
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var e error
		rec, e = a.rRepo.GetTx(r.Context(), tx, id)
		return e
	})
	if errors.Is(err, recording.ErrNotFound) {
		writeError(w, http.StatusNotFound, "recording not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup failed")
		return
	}
	signed, err := a.store.PresignGet(r.Context(), rec.FileKey, 5*time.Minute)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "presign failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": signed})
}
