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
	"p1/engine/internal/dnc"
	"p1/engine/internal/lead"
	"p1/engine/internal/tenant"
)

type tenantDNC struct {
	repo  *tenant.Repo
	dRepo *dnc.Repo
}

type addDNCRequest struct {
	PhoneE164 string  `json:"phone_e164"`
	Source    *string `json:"source"`
	Reason    *string `json:"reason"`
	ExpiresAt *string `json:"expires_at"`
}

type dncResponse struct {
	ID        int64   `json:"id"`
	TenantID  *int64  `json:"tenant_id,omitempty"`
	Scope     string  `json:"scope"`
	PhoneE164 string  `json:"phone_e164"`
	Source    *string `json:"source,omitempty"`
	Reason    *string `json:"reason,omitempty"`
	AddedAt   string  `json:"added_at"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

func dncToResponse(e dnc.Entry) dncResponse {
	r := dncResponse{
		ID:        e.ID,
		TenantID:  e.TenantID,
		Scope:     string(e.Scope),
		PhoneE164: e.PhoneE164,
		Source:    e.Source,
		Reason:    e.Reason,
		AddedAt:   e.AddedAt.Format(time.RFC3339),
	}
	if e.ExpiresAt != nil {
		s := e.ExpiresAt.Format(time.RFC3339)
		r.ExpiresAt = &s
	}
	return r
}

func (a *tenantDNC) add(w http.ResponseWriter, r *http.Request) {
	var req addDNCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !lead.ValidE164(req.PhoneE164) {
		writeError(w, http.StatusBadRequest, "phone_e164 must be E.164 format")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "expires_at must be RFC3339")
			return
		}
		expiresAt = &t
	}

	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID

	entry := dnc.Entry{
		TenantID:  &tid,
		PhoneE164: req.PhoneE164,
		Source:    req.Source,
		Reason:    req.Reason,
		ExpiresAt: expiresAt,
	}

	var created dnc.Entry
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		created, err = a.dRepo.AddInternalTx(r.Context(), tx, entry)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "dnc_entry",
			EntityID:   strconv.FormatInt(created.ID, 10),
			Action:     "add",
			After:      map[string]any{"phone_e164": created.PhoneE164, "reason": created.Reason},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "phone already on internal DNC")
			return
		}
		writeError(w, http.StatusInternalServerError, "add failed")
		return
	}
	writeJSON(w, http.StatusCreated, dncToResponse(created))
}

func (a *tenantDNC) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	q := r.URL.Query()

	f := dnc.ListFilter{
		Scope:  q.Get("scope"),
		Search: q.Get("search"),
		Limit:  atoiOr(q.Get("limit"), 100),
		Offset: atoiOr(q.Get("offset"), 0),
	}

	var list []dnc.Entry
	var total int
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		list, total, err = a.dRepo.ListTx(r.Context(), tx, f)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]dncResponse, len(list))
	for i, e := range list {
		out[i] = dncToResponse(e)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"entries": out,
		"total":   total,
		"limit":   f.Limit,
		"offset":  f.Offset,
	})
}

func (a *tenantDNC) remove(w http.ResponseWriter, r *http.Request) {
	phone := chi.URLParam(r, "phone")
	if !lead.ValidE164(phone) {
		writeError(w, http.StatusBadRequest, "invalid phone")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		if err := a.dRepo.RemoveInternalTx(r.Context(), tx, tid, phone); err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "dnc_entry",
			EntityID:   phone,
			Action:     "remove",
			Before:     map[string]any{"phone_e164": phone},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if errors.Is(err, dnc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "phone not on internal DNC")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "remove failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type checkResponse struct {
	Blocked   bool   `json:"blocked"`
	Scope     string `json:"scope,omitempty"`
	PhoneE164 string `json:"phone_e164"`
}

func (a *tenantDNC) check(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	if !lead.ValidE164(phone) {
		writeError(w, http.StatusBadRequest, "phone must be E.164")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var blocked bool
	var scope string
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		blocked, scope, err = a.dRepo.IsBlockedTx(r.Context(), tx, phone)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "check failed")
		return
	}
	writeJSON(w, http.StatusOK, checkResponse{Blocked: blocked, Scope: scope, PhoneE164: phone})
}
