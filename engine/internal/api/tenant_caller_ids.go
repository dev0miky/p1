package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"p1/engine/internal/audit"
	"p1/engine/internal/auth"
	"p1/engine/internal/callerid"
	"p1/engine/internal/db"
	"p1/engine/internal/lead"
	"p1/engine/internal/tenant"
)

type tenantCallerIDs struct {
	repo  *tenant.Repo
	cRepo *callerid.Repo
}

type callerIDResponse struct {
	ID          int64    `json:"id"`
	TenantID    int64    `json:"tenant_id"`
	Name        string   `json:"name"`
	E164Number  string   `json:"e164_number"`
	DisplayName *string  `json:"display_name,omitempty"`
	Attestation string   `json:"attestation"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

func callerIDToResponse(c callerid.CallerID) callerIDResponse {
	tags := c.Tags
	if tags == nil {
		tags = []string{}
	}
	return callerIDResponse{
		ID:          c.ID,
		TenantID:    c.TenantID,
		Name:        c.Name,
		E164Number:  c.E164Number,
		DisplayName: c.DisplayName,
		Attestation: string(c.Attestation),
		Description: c.Description,
		Tags:        tags,
		CreatedAt:   c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   c.UpdatedAt.Format(time.RFC3339),
	}
}

func (a *tenantCallerIDs) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		E164Number  string   `json:"e164_number"`
		DisplayName *string  `json:"display_name"`
		Attestation string   `json:"attestation"`
		Description *string  `json:"description"`
		Tags        []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if !lead.ValidE164(req.E164Number) {
		writeError(w, http.StatusBadRequest, "e164_number must be E.164 (+1XXXXXXXXXX)")
		return
	}
	if req.Attestation != "" && !callerid.ValidAttestation(req.Attestation) {
		writeError(w, http.StatusBadRequest, "attestation must be a, b, c, or none")
		return
	}
	if req.Attestation == "" {
		req.Attestation = string(callerid.AttestationNone)
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	c := callerid.CallerID{
		TenantID:    tid,
		Name:        req.Name,
		E164Number:  req.E164Number,
		DisplayName: req.DisplayName,
		Attestation: callerid.Attestation(req.Attestation),
		Description: req.Description,
		Tags:        req.Tags,
	}
	var created callerid.CallerID
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		created, err = a.cRepo.CreateTx(r.Context(), tx, c)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "caller_id",
			EntityID:   strconv.FormatInt(created.ID, 10),
			Action:     "create",
			After:      map[string]any{"name": created.Name, "e164": created.E164Number, "attestation": created.Attestation},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "name or number already exists")
			return
		}
		slog.Error("caller_id create failed", "err", err, "tenant", tid)
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, callerIDToResponse(created))
}

func (a *tenantCallerIDs) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var list []callerid.CallerID
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		list, err = a.cRepo.ListTx(r.Context(), tx)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]callerIDResponse, len(list))
	for i, c := range list {
		out[i] = callerIDToResponse(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"caller_ids": out})
}

func (a *tenantCallerIDs) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var c callerid.CallerID
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		c, err = a.cRepo.GetTx(r.Context(), tx, id)
		return err
	})
	if errors.Is(err, callerid.ErrNotFound) {
		writeError(w, http.StatusNotFound, "caller_id not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}
	writeJSON(w, http.StatusOK, callerIDToResponse(c))
}

func (a *tenantCallerIDs) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	patch := callerid.UpdatePatch{}
	if v, ok := raw["name"]; ok {
		_ = json.Unmarshal(v, &patch.Name)
	}
	if v, ok := raw["display_name"]; ok {
		_ = json.Unmarshal(v, &patch.DisplayName)
	}
	if v, ok := raw["attestation"]; ok {
		_ = json.Unmarshal(v, &patch.Attestation)
	}
	if v, ok := raw["description"]; ok {
		_ = json.Unmarshal(v, &patch.Description)
	}
	if v, ok := raw["tags"]; ok {
		patch.SetTags = true
		_ = json.Unmarshal(v, &patch.Tags)
	}
	if patch.Attestation != "" && !callerid.ValidAttestation(patch.Attestation) {
		writeError(w, http.StatusBadRequest, "invalid attestation")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	var out callerid.CallerID
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		out, err = a.cRepo.UpdateTx(r.Context(), tx, id, patch)
		return err
	})
	if errors.Is(err, callerid.ErrNotFound) {
		writeError(w, http.StatusNotFound, "caller_id not found")
		return
	}
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, callerIDToResponse(out))
}

func (a *tenantCallerIDs) delete(w http.ResponseWriter, r *http.Request) {
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
		return a.cRepo.DeleteTx(r.Context(), tx, id)
	})
	if errors.Is(err, callerid.ErrNotFound) {
		writeError(w, http.StatusNotFound, "caller_id not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
