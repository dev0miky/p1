package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"p1/engine/internal/audit"
	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/tenant"
)

type adminTenants struct {
	repo *tenant.Repo
}

type createTenantRequest struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	SIPDomain string `json:"sip_domain"`
}

type updateTenantRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type tenantResponse struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	SIPDomain string `json:"sip_domain"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toResponse(t tenant.Tenant) tenantResponse {
	return tenantResponse{
		ID:        t.ID,
		Slug:      t.Slug,
		Name:      t.Name,
		SIPDomain: t.SIPDomain,
		Status:    string(t.Status),
		CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (a *adminTenants) create(w http.ResponseWriter, r *http.Request) {
	var req createTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Slug == "" || req.Name == "" || req.SIPDomain == "" {
		writeError(w, http.StatusBadRequest, "slug, name, sip_domain required")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())

	var created tenant.Tenant
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role}, func(tx pgx.Tx) error {
		var err error
		created, err = a.repo.CreateTenantTx(r.Context(), tx, tenant.Tenant{
			Slug: req.Slug, Name: req.Name, SIPDomain: req.SIPDomain,
		})
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			EntityType: "tenant",
			EntityID:   strconv.FormatInt(created.ID, 10),
			Action:     "create",
			After:      created,
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "slug or sip_domain already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, toResponse(created))
}

func (a *adminTenants) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var list []tenant.Tenant
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role}, func(tx pgx.Tx) error {
		var err error
		list, err = a.repo.ListTenantsTx(r.Context(), tx)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]tenantResponse, len(list))
	for i, t := range list {
		out[i] = toResponse(t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenants": out})
}

func (a *adminTenants) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var t tenant.Tenant
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role}, func(tx pgx.Tx) error {
		var err error
		t, err = a.repo.GetTenantTx(r.Context(), tx, id)
		return err
	})
	if errors.Is(err, tenant.ErrNotFound) {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}
	writeJSON(w, http.StatusOK, toResponse(t))
}

func (a *adminTenants) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req updateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" && req.Status == "" {
		writeError(w, http.StatusBadRequest, "no fields to update")
		return
	}
	if req.Status != "" && req.Status != "active" && req.Status != "suspended" && req.Status != "deleted" {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())

	var before, after tenant.Tenant
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role}, func(tx pgx.Tx) error {
		var err error
		before, err = a.repo.GetTenantTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		after, err = a.repo.UpdateTenantTx(r.Context(), tx, id, req.Name, req.Status)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			EntityType: "tenant",
			EntityID:   strconv.FormatInt(id, 10),
			Action:     "update",
			Before:     before,
			After:      after,
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if errors.Is(err, tenant.ErrNotFound) {
		writeError(w, http.StatusNotFound, "tenant not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, toResponse(after))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	type sqlState interface{ SQLState() string }
	var s sqlState
	if errors.As(err, &s) && s.SQLState() == "23505" {
		return true
	}
	return false
}
