package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"p1/engine/internal/audit"
	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/lead"
	"p1/engine/internal/tenant"
)

type tenantLeads struct {
	repo  *tenant.Repo
	lRepo *lead.Repo
}

type createLeadRequest struct {
	ListID          *int64          `json:"list_id"`
	CampaignID      *int64          `json:"campaign_id"`
	PhoneE164       string          `json:"phone_e164"`
	DialDestination *string         `json:"dial_destination"`
	FirstName       *string         `json:"first_name"`
	LastName        *string         `json:"last_name"`
	Email           *string         `json:"email"`
	Timezone        *string         `json:"timezone"`
	StateCode       *string         `json:"state_code"`
	CustomFields    json.RawMessage `json:"custom_fields"`
}

type leadResponse struct {
	ID              int64           `json:"id"`
	TenantID        int64           `json:"tenant_id"`
	ListID          *int64          `json:"list_id,omitempty"`
	CampaignID      *int64          `json:"campaign_id,omitempty"`
	PhoneE164       string          `json:"phone_e164"`
	DialDestination *string         `json:"dial_destination,omitempty"`
	FirstName       *string         `json:"first_name,omitempty"`
	LastName        *string         `json:"last_name,omitempty"`
	Email           *string         `json:"email,omitempty"`
	Timezone        *string         `json:"timezone,omitempty"`
	StateCode       *string         `json:"state_code,omitempty"`
	Status          string          `json:"status"`
	Attempts        int             `json:"attempts"`
	LastAttemptAt   *string         `json:"last_attempt_at,omitempty"`
	NextEligibleAt  *string         `json:"next_eligible_at,omitempty"`
	CustomFields    json.RawMessage `json:"custom_fields"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
}

func leadToResponse(l lead.Lead) leadResponse {
	r := leadResponse{
		ID:              l.ID,
		TenantID:        l.TenantID,
		ListID:          l.ListID,
		CampaignID:      l.CampaignID,
		PhoneE164:       l.PhoneE164,
		DialDestination: l.DialDestination,
		FirstName:       l.FirstName,
		LastName:        l.LastName,
		Email:           l.Email,
		Timezone:        l.Timezone,
		StateCode:       l.StateCode,
		Status:          string(l.Status),
		Attempts:        l.Attempts,
		CustomFields:    l.CustomFields,
		CreatedAt:       l.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       l.UpdatedAt.Format(time.RFC3339),
	}
	if l.LastAttemptAt != nil {
		s := l.LastAttemptAt.Format(time.RFC3339)
		r.LastAttemptAt = &s
	}
	if l.NextEligibleAt != nil {
		s := l.NextEligibleAt.Format(time.RFC3339)
		r.NextEligibleAt = &s
	}
	return r
}

func (a *tenantLeads) create(w http.ResponseWriter, r *http.Request) {
	var req createLeadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.PhoneE164 == "" {
		if req.DialDestination != nil && *req.DialDestination != "" {
			req.PhoneE164 = "+10000000000"
		} else {
			writeError(w, http.StatusBadRequest, "phone_e164 required")
			return
		}
	}
	if !lead.ValidE164(req.PhoneE164) {
		writeError(w, http.StatusBadRequest, "phone_e164 must be E.164 format (+1XXXXXXXXXX)")
		return
	}

	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID

	l := lead.Lead{
		TenantID:        tid,
		ListID:          req.ListID,
		CampaignID:      req.CampaignID,
		PhoneE164:       req.PhoneE164,
		DialDestination: req.DialDestination,
		FirstName:       req.FirstName,
		LastName:        req.LastName,
		Email:           req.Email,
		Timezone:        req.Timezone,
		StateCode:       req.StateCode,
		CustomFields:    req.CustomFields,
	}

	var created lead.Lead
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		created, err = a.lRepo.CreateLeadTx(r.Context(), tx, l)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "lead",
			EntityID:   strconv.FormatInt(created.ID, 10),
			Action:     "create",
			After:      map[string]any{"phone_e164": created.PhoneE164, "campaign_id": created.CampaignID},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "lead already exists in campaign")
			return
		}
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, leadToResponse(created))
}

func (a *tenantLeads) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	q := r.URL.Query()

	f := lead.ListFilter{
		Status: q.Get("status"),
		Limit:  atoiOr(q.Get("limit"), 100),
		Offset: atoiOr(q.Get("offset"), 0),
	}
	if v := q.Get("campaign_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.CampaignID = &n
		}
	}
	if v := q.Get("list_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.ListID = &n
		}
	}

	var list []lead.Lead
	var total int
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		list, total, err = a.lRepo.ListLeadsTx(r.Context(), tx, f)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]leadResponse, len(list))
	for i, l := range list {
		out[i] = leadToResponse(l)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"leads":  out,
		"total":  total,
		"limit":  f.Limit,
		"offset": f.Offset,
	})
}

func (a *tenantLeads) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var l lead.Lead
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		l, err = a.lRepo.GetLeadTx(r.Context(), tx, id)
		return err
	})
	if errors.Is(err, lead.ErrNotFound) {
		writeError(w, http.StatusNotFound, "lead not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}
	writeJSON(w, http.StatusOK, leadToResponse(l))
}

func (a *tenantLeads) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		before, err := a.lRepo.GetLeadTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		if err := a.lRepo.DeleteLeadTx(r.Context(), tx, id); err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "lead",
			EntityID:   strconv.FormatInt(id, 10),
			Action:     "delete",
			Before:     map[string]any{"phone_e164": before.PhoneE164, "status": before.Status},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if errors.Is(err, lead.ErrNotFound) {
		writeError(w, http.StatusNotFound, "lead not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func atoiOr(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
