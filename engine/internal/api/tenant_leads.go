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
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context — super admins cannot create tenant resources without a tenant scope; sign in as a tenant user")
		return
	}

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
		slog.Error("lead create failed", "err", err, "tenant_id", tid, "phone", req.PhoneE164, "req_id", middleware.GetReqID(r.Context()))
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

type activityEvent struct {
	FromState *string `json:"from_state"`
	ToState   string  `json:"to_state"`
	Reason    *string `json:"reason"`
	At        string  `json:"at"`
}

type activityCall struct {
	UUID        string          `json:"uuid"`
	State       string          `json:"state"`
	StartedAt   string          `json:"started_at"`
	AnsweredAt  *string         `json:"answered_at,omitempty"`
	EndedAt     *string         `json:"ended_at,omitempty"`
	HangupCause *string         `json:"hangup_cause,omitempty"`
	Events      []activityEvent `json:"events"`
}

func (a *tenantLeads) activity(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var calls []activityCall
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		if _, err := a.lRepo.GetLeadTx(r.Context(), tx, id); err != nil {
			return err
		}
		rows, err := tx.Query(r.Context(), `
			SELECT uuid::text, state, started_at, answered_at, ended_at, hangup_cause
			  FROM call_state
			 WHERE lead_id = $1
			 ORDER BY started_at DESC
			 LIMIT 50
		`, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c activityCall
			var startedAt time.Time
			var answeredAt, endedAt *time.Time
			if err := rows.Scan(&c.UUID, &c.State, &startedAt, &answeredAt, &endedAt, &c.HangupCause); err != nil {
				return err
			}
			c.StartedAt = startedAt.Format(time.RFC3339Nano)
			if answeredAt != nil {
				s := answeredAt.Format(time.RFC3339Nano)
				c.AnsweredAt = &s
			}
			if endedAt != nil {
				s := endedAt.Format(time.RFC3339Nano)
				c.EndedAt = &s
			}
			calls = append(calls, c)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for i, c := range calls {
			eRows, err := tx.Query(r.Context(), `
				SELECT from_state, to_state, reason, at
				  FROM call_events
				 WHERE call_uuid = $1::uuid
				 ORDER BY at DESC, id DESC
			`, c.UUID)
			if err != nil {
				return err
			}
			var events []activityEvent
			for eRows.Next() {
				var e activityEvent
				var at time.Time
				if err := eRows.Scan(&e.FromState, &e.ToState, &e.Reason, &at); err != nil {
					eRows.Close()
					return err
				}
				e.At = at.Format(time.RFC3339Nano)
				events = append(events, e)
			}
			eRows.Close()
			calls[i].Events = events
		}
		return nil
	})
	if errors.Is(err, lead.ErrNotFound) {
		writeError(w, http.StatusNotFound, "lead not found")
		return
	}
	if err != nil {
		slog.Error("lead activity failed", "err", err, "lead_id", id, "tenant_id", claims.TenantID, "req_id", middleware.GetReqID(r.Context()))
		writeError(w, http.StatusInternalServerError, "activity failed")
		return
	}
	if calls == nil {
		calls = []activityCall{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"calls": calls})
}

type updateLeadRequest struct {
	CampaignID         *int64  `json:"campaign_id"`
	HasCampaignID      bool    `json:"-"`
	DialDestination    *string `json:"dial_destination"`
	HasDialDestination bool    `json:"-"`
}

func (req *updateLeadRequest) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["campaign_id"]; ok {
		req.HasCampaignID = true
		if string(v) != "null" {
			var n int64
			if err := json.Unmarshal(v, &n); err != nil {
				return err
			}
			req.CampaignID = &n
		}
	}
	if v, ok := raw["dial_destination"]; ok {
		req.HasDialDestination = true
		if string(v) != "null" {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			if s != "" {
				req.DialDestination = &s
			}
		}
	}
	return nil
}

func (a *tenantLeads) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req updateLeadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context — super admins cannot update tenant resources without a tenant scope; sign in as a tenant user")
		return
	}
	var out lead.Lead
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		before, err := a.lRepo.GetLeadTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		out, err = a.lRepo.UpdateLeadTx(r.Context(), tx, id, lead.LeadUpdate{
			CampaignID:         req.CampaignID,
			SetCampaign:        req.HasCampaignID,
			DialDestination:    req.DialDestination,
			SetDialDestination: req.HasDialDestination,
		})
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "lead",
			EntityID:   strconv.FormatInt(id, 10),
			Action:     "update",
			Before:     map[string]any{"campaign_id": before.CampaignID, "dial_destination": before.DialDestination},
			After:      map[string]any{"campaign_id": out.CampaignID, "dial_destination": out.DialDestination},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if errors.Is(err, lead.ErrNotFound) {
		writeError(w, http.StatusNotFound, "lead not found")
		return
	}
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "lead already exists in campaign")
			return
		}
		slog.Error("lead update failed", "err", err, "tenant_id", tid, "lead_id", id, "req_id", middleware.GetReqID(r.Context()))
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, leadToResponse(out))
}

func (a *tenantLeads) redial(w http.ResponseWriter, r *http.Request) {
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
	var out lead.Lead
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		before, err := a.lRepo.GetLeadTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		out, err = a.lRepo.RedialLeadTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "lead",
			EntityID:   strconv.FormatInt(id, 10),
			Action:     "redial",
			Before:     map[string]any{"status": before.Status, "attempts": before.Attempts},
			After:      map[string]any{"status": out.Status, "attempts": out.Attempts},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if errors.Is(err, lead.ErrNotFound) {
		writeError(w, http.StatusNotFound, "lead not found")
		return
	}
	if err != nil {
		slog.Error("lead redial failed", "err", err, "tenant_id", tid, "lead_id", id, "req_id", middleware.GetReqID(r.Context()))
		writeError(w, http.StatusInternalServerError, "redial failed")
		return
	}
	writeJSON(w, http.StatusOK, leadToResponse(out))
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
