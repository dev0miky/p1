package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"p1/engine/internal/audit"
	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/extagent"
	"p1/engine/internal/tenant"
)

type tenantExternalAgents struct {
	repo  *tenant.Repo
	aRepo *extagent.Repo
}

type createExternalAgentRequest struct {
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	DialString  string   `json:"dial_string"`
	Tags        []string `json:"tags"`
}

type externalAgentResponse struct {
	ID          int64    `json:"id"`
	TenantID    int64    `json:"tenant_id"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	DialString  string   `json:"dial_string"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

func extAgentToResponse(a extagent.Agent) externalAgentResponse {
	tags := a.Tags
	if tags == nil {
		tags = []string{}
	}
	return externalAgentResponse{
		ID:          a.ID,
		TenantID:    a.TenantID,
		Name:        a.Name,
		Description: a.Description,
		DialString:  a.DialString,
		Tags:        tags,
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   a.UpdatedAt.Format(time.RFC3339),
	}
}

func (a *tenantExternalAgents) create(w http.ResponseWriter, r *http.Request) {
	var req createExternalAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.DialString = strings.TrimSpace(req.DialString)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if req.DialString == "" {
		writeError(w, http.StatusBadRequest, "dial_string required")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	ag := extagent.Agent{
		TenantID:    tid,
		Name:        req.Name,
		Description: req.Description,
		DialString:  req.DialString,
		Tags:        req.Tags,
	}
	var created extagent.Agent
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		created, err = a.aRepo.CreateTx(r.Context(), tx, ag)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "external_agent",
			EntityID:   strconv.FormatInt(created.ID, 10),
			Action:     "create",
			After:      map[string]any{"name": created.Name, "dial_string": created.DialString},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "agent name already exists")
			return
		}
		slog.Error("external agent create failed", "err", err, "tenant", tid)
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, extAgentToResponse(created))
}

func (a *tenantExternalAgents) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var list []extagent.Agent
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		list, err = a.aRepo.ListTx(r.Context(), tx)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]externalAgentResponse, len(list))
	for i, ag := range list {
		out[i] = extAgentToResponse(ag)
	}
	writeJSON(w, http.StatusOK, map[string]any{"external_agents": out})
}

func (a *tenantExternalAgents) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var ag extagent.Agent
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		ag, err = a.aRepo.GetTx(r.Context(), tx, id)
		return err
	})
	if errors.Is(err, extagent.ErrNotFound) {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}
	writeJSON(w, http.StatusOK, extAgentToResponse(ag))
}

func (a *tenantExternalAgents) update(w http.ResponseWriter, r *http.Request) {
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
	patch := extagent.UpdatePatch{}
	if v, ok := raw["name"]; ok {
		_ = json.Unmarshal(v, &patch.Name)
		patch.Name = strings.TrimSpace(patch.Name)
	}
	if v, ok := raw["description"]; ok {
		_ = json.Unmarshal(v, &patch.Description)
	}
	if v, ok := raw["dial_string"]; ok {
		_ = json.Unmarshal(v, &patch.DialString)
		patch.DialString = strings.TrimSpace(patch.DialString)
	}
	if v, ok := raw["tags"]; ok {
		patch.SetTags = true
		_ = json.Unmarshal(v, &patch.Tags)
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	var out extagent.Agent
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		out, err = a.aRepo.UpdateTx(r.Context(), tx, id, patch)
		return err
	})
	if errors.Is(err, extagent.ErrNotFound) {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "agent name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, extAgentToResponse(out))
}

func (a *tenantExternalAgents) delete(w http.ResponseWriter, r *http.Request) {
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
		return a.aRepo.DeleteTx(r.Context(), tx, id)
	})
	if errors.Is(err, extagent.ErrNotFound) {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err != nil {
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusConflict, "agent is referenced by a script — detach it first")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "23503")
}
