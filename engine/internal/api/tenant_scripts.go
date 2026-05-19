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
	"p1/engine/internal/script"
	"p1/engine/internal/tenant"
)

func normalizeTransferTo(s string) string {
	return strings.TrimSpace(s)
}

type tenantScripts struct {
	repo  *tenant.Repo
	sRepo *script.Repo
}

type createScriptRequest struct {
	Name             string   `json:"name"`
	Description      *string  `json:"description"`
	Type             string   `json:"type"`
	Body             string   `json:"body"`
	TransferTo       *string  `json:"transfer_to"`
	GreetingSoundID  *int64   `json:"greeting_sound_id"`
	PreBridgeSoundID *int64   `json:"pre_bridge_sound_id"`
	BridgeDigit      string   `json:"bridge_digit"`
	WaitTimeoutMS    int      `json:"wait_timeout_ms"`
	OptOutDigit      *string  `json:"opt_out_digit"`
	Tags             []string `json:"tags"`
}

type scriptResponse struct {
	ID               int64    `json:"id"`
	TenantID         int64    `json:"tenant_id"`
	Name             string   `json:"name"`
	Description      *string  `json:"description,omitempty"`
	Type             string   `json:"type"`
	Body             string   `json:"body"`
	TransferTo       *string  `json:"transfer_to,omitempty"`
	GreetingSoundID  *int64   `json:"greeting_sound_id,omitempty"`
	PreBridgeSoundID *int64   `json:"pre_bridge_sound_id,omitempty"`
	BridgeDigit      string   `json:"bridge_digit"`
	WaitTimeoutMS    int      `json:"wait_timeout_ms"`
	OptOutDigit      *string  `json:"opt_out_digit,omitempty"`
	Tags             []string `json:"tags"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

func scriptToResponse(s script.Script) scriptResponse {
	tags := s.Tags
	if tags == nil {
		tags = []string{}
	}
	return scriptResponse{
		ID:               s.ID,
		TenantID:         s.TenantID,
		Name:             s.Name,
		Description:      s.Description,
		Type:             string(s.Type),
		Body:             s.Body,
		TransferTo:       s.TransferTo,
		GreetingSoundID:  s.GreetingSoundID,
		PreBridgeSoundID: s.PreBridgeSoundID,
		BridgeDigit:      s.BridgeDigit,
		WaitTimeoutMS:    s.WaitTimeoutMS,
		OptOutDigit:      s.OptOutDigit,
		Tags:             tags,
		CreatedAt:        s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        s.UpdatedAt.Format(time.RFC3339),
	}
}

func (a *tenantScripts) create(w http.ResponseWriter, r *http.Request) {
	var req createScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if !script.ValidType(req.Type) {
		writeError(w, http.StatusBadRequest, "invalid type (press1|broadcast|survey|custom)")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	if req.TransferTo != nil {
		t := normalizeTransferTo(*req.TransferTo)
		if t == "" {
			req.TransferTo = nil
		} else {
			req.TransferTo = &t
		}
	}
	if req.BridgeDigit == "" {
		req.BridgeDigit = "1"
	}
	if !script.ValidDTMFDigit(req.BridgeDigit) {
		writeError(w, http.StatusBadRequest, "bridge_digit must be one of 0-9, *, #")
		return
	}
	if req.WaitTimeoutMS == 0 {
		req.WaitTimeoutMS = 8000
	}
	if req.WaitTimeoutMS < 1000 || req.WaitTimeoutMS > 60000 {
		writeError(w, http.StatusBadRequest, "wait_timeout_ms must be 1000–60000")
		return
	}
	if req.OptOutDigit != nil {
		if !script.ValidDTMFDigit(*req.OptOutDigit) {
			writeError(w, http.StatusBadRequest, "opt_out_digit must be one of 0-9, *, #")
			return
		}
		if *req.OptOutDigit == req.BridgeDigit {
			writeError(w, http.StatusBadRequest, "opt_out_digit must differ from bridge_digit")
			return
		}
	}
	s := script.Script{
		TenantID:         tid,
		Name:             req.Name,
		Description:      req.Description,
		Type:             script.Type(req.Type),
		Body:             req.Body,
		TransferTo:       req.TransferTo,
		GreetingSoundID:  req.GreetingSoundID,
		PreBridgeSoundID: req.PreBridgeSoundID,
		BridgeDigit:      req.BridgeDigit,
		WaitTimeoutMS:    req.WaitTimeoutMS,
		OptOutDigit:      req.OptOutDigit,
		Tags:             req.Tags,
	}
	var created script.Script
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		created, err = a.sRepo.CreateTx(r.Context(), tx, s)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "script",
			EntityID:   strconv.FormatInt(created.ID, 10),
			Action:     "create",
			After:      map[string]any{"name": created.Name, "type": created.Type},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "script name already exists")
			return
		}
		slog.Error("script create failed", "err", err, "tenant", tid)
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, scriptToResponse(created))
}

func (a *tenantScripts) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var list []script.Script
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		list, err = a.sRepo.ListTx(r.Context(), tx)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]scriptResponse, len(list))
	for i, s := range list {
		out[i] = scriptToResponse(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"scripts": out})
}

func (a *tenantScripts) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var s script.Script
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		s, err = a.sRepo.GetTx(r.Context(), tx, id)
		return err
	})
	if errors.Is(err, script.ErrNotFound) {
		writeError(w, http.StatusNotFound, "script not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}
	writeJSON(w, http.StatusOK, scriptToResponse(s))
}

func (a *tenantScripts) update(w http.ResponseWriter, r *http.Request) {
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
	patch := script.UpdatePatch{}
	if v, ok := raw["name"]; ok {
		_ = json.Unmarshal(v, &patch.Name)
	}
	if v, ok := raw["description"]; ok {
		_ = json.Unmarshal(v, &patch.Description)
	}
	if v, ok := raw["type"]; ok {
		_ = json.Unmarshal(v, &patch.Type)
	}
	if v, ok := raw["body"]; ok {
		_ = json.Unmarshal(v, &patch.Body)
	}
	if v, ok := raw["transfer_to"]; ok {
		patch.SetTransferTo = true
		var s *string
		_ = json.Unmarshal(v, &s)
		if s != nil {
			t := normalizeTransferTo(*s)
			if t == "" {
				patch.TransferTo = nil
			} else {
				patch.TransferTo = &t
			}
		}
	}
	if v, ok := raw["greeting_sound_id"]; ok {
		patch.SetGreetingSoundID = true
		_ = json.Unmarshal(v, &patch.GreetingSoundID)
	}
	if v, ok := raw["pre_bridge_sound_id"]; ok {
		patch.SetPreBridgeSoundID = true
		_ = json.Unmarshal(v, &patch.PreBridgeSoundID)
	}
	if v, ok := raw["bridge_digit"]; ok {
		_ = json.Unmarshal(v, &patch.BridgeDigit)
		if patch.BridgeDigit != "" && !script.ValidDTMFDigit(patch.BridgeDigit) {
			writeError(w, http.StatusBadRequest, "bridge_digit must be one of 0-9, *, #")
			return
		}
	}
	if v, ok := raw["wait_timeout_ms"]; ok {
		var t int
		_ = json.Unmarshal(v, &t)
		if t != 0 {
			if t < 1000 || t > 60000 {
				writeError(w, http.StatusBadRequest, "wait_timeout_ms must be 1000–60000")
				return
			}
			patch.WaitTimeoutMS = &t
		}
	}
	if v, ok := raw["opt_out_digit"]; ok {
		patch.SetOptOutDigit = true
		_ = json.Unmarshal(v, &patch.OptOutDigit)
		if patch.OptOutDigit != nil && !script.ValidDTMFDigit(*patch.OptOutDigit) {
			writeError(w, http.StatusBadRequest, "opt_out_digit must be one of 0-9, *, #")
			return
		}
	}
	if v, ok := raw["tags"]; ok {
		patch.SetTags = true
		_ = json.Unmarshal(v, &patch.Tags)
	}
	if patch.Type != "" && !script.ValidType(patch.Type) {
		writeError(w, http.StatusBadRequest, "invalid type")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	var out script.Script
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		out, err = a.sRepo.UpdateTx(r.Context(), tx, id, patch)
		return err
	})
	if errors.Is(err, script.ErrNotFound) {
		writeError(w, http.StatusNotFound, "script not found")
		return
	}
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "script name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, scriptToResponse(out))
}

func (a *tenantScripts) delete(w http.ResponseWriter, r *http.Request) {
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
		return a.sRepo.DeleteTx(r.Context(), tx, id)
	})
	if errors.Is(err, script.ErrNotFound) {
		writeError(w, http.StatusNotFound, "script not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
