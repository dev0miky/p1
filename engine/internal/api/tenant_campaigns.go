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
	"p1/engine/internal/campaign"
	"p1/engine/internal/db"
	"p1/engine/internal/tenant"
)

type tenantCampaigns struct {
	repo  *tenant.Repo
	cRepo *campaign.Repo
}

type createCampaignRequest struct {
	Name           string          `json:"name"`
	Mode           string          `json:"mode"`
	DialRatio      *float64        `json:"dial_ratio"`
	MaxAbandonPct  *float64        `json:"max_abandon_pct"`
	PromptAudio    *string         `json:"prompt_audio"`
	TransferDest   *string         `json:"transfer_dest"`
	CallerIDPool   json.RawMessage `json:"caller_id_pool"`
	RetryPolicy    json.RawMessage `json:"retry_policy"`
	CallingHours   json.RawMessage `json:"calling_hours"`
	TZStrategy     string          `json:"tz_strategy"`
	CallConstraint string          `json:"call_constraint"`
}

type updateCampaignRequest = createCampaignRequest

type campaignResponse struct {
	ID             int64           `json:"id"`
	TenantID       int64           `json:"tenant_id"`
	Name           string          `json:"name"`
	Mode           string          `json:"mode"`
	Status         string          `json:"status"`
	DialRatio      float64         `json:"dial_ratio"`
	MaxAbandonPct  float64         `json:"max_abandon_pct"`
	PromptAudio    *string         `json:"prompt_audio,omitempty"`
	TransferDest   *string         `json:"transfer_dest,omitempty"`
	CallerIDPool   json.RawMessage `json:"caller_id_pool"`
	RetryPolicy    json.RawMessage `json:"retry_policy"`
	CallingHours   json.RawMessage `json:"calling_hours"`
	TZStrategy     string          `json:"tz_strategy"`
	RunNo          int             `json:"run_no"`
	CallConstraint string          `json:"call_constraint"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}

func campaignToResponse(c campaign.Campaign) campaignResponse {
	return campaignResponse{
		ID:             c.ID,
		TenantID:       c.TenantID,
		Name:           c.Name,
		Mode:           string(c.Mode),
		Status:         string(c.Status),
		DialRatio:      c.DialRatio,
		MaxAbandonPct:  c.MaxAbandonPct,
		PromptAudio:    c.PromptAudio,
		TransferDest:   c.TransferDest,
		CallerIDPool:   c.CallerIDPool,
		RetryPolicy:    c.RetryPolicy,
		CallingHours:   c.CallingHours,
		TZStrategy:     c.TZStrategy,
		RunNo:          c.RunNo,
		CallConstraint: c.CallConstraint,
		CreatedAt:      c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      c.UpdatedAt.Format(time.RFC3339),
	}
}

func (a *tenantCampaigns) create(w http.ResponseWriter, r *http.Request) {
	var req createCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if !campaign.ValidMode(req.Mode) {
		writeError(w, http.StatusBadRequest, "invalid mode (press1|broadcast|predictive|preview)")
		return
	}
	if req.TZStrategy != "" && req.TZStrategy != "lead_local" && req.TZStrategy != "campaign_local" {
		writeError(w, http.StatusBadRequest, "invalid tz_strategy")
		return
	}

	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID

	c := campaign.Campaign{
		TenantID:     tid,
		Name:         req.Name,
		Mode:         campaign.Mode(req.Mode),
		Status:       campaign.StatusPaused,
		CallerIDPool: req.CallerIDPool,
		RetryPolicy:  req.RetryPolicy,
		CallingHours: req.CallingHours,
		TZStrategy:   req.TZStrategy,
		PromptAudio:  req.PromptAudio,
		TransferDest: req.TransferDest,
	}
	if req.DialRatio != nil {
		c.DialRatio = *req.DialRatio
	}
	if req.MaxAbandonPct != nil {
		c.MaxAbandonPct = *req.MaxAbandonPct
	}

	var created campaign.Campaign
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
			EntityType: "campaign",
			EntityID:   strconv.FormatInt(created.ID, 10),
			Action:     "create",
			After:      map[string]any{"name": created.Name, "mode": created.Mode, "status": created.Status},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "campaign name already exists in tenant")
			return
		}
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, campaignToResponse(created))
}

func (a *tenantCampaigns) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var list []campaign.Campaign
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		list, err = a.cRepo.ListTx(r.Context(), tx)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]campaignResponse, len(list))
	for i, c := range list {
		out[i] = campaignToResponse(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"campaigns": out})
}

func (a *tenantCampaigns) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var c campaign.Campaign
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		c, err = a.cRepo.GetTx(r.Context(), tx, id)
		return err
	})
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}
	writeJSON(w, http.StatusOK, campaignToResponse(c))
}

func (a *tenantCampaigns) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		updateCampaignRequest
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Mode != "" && !campaign.ValidMode(req.Mode) {
		writeError(w, http.StatusBadRequest, "invalid mode")
		return
	}
	if req.Status != "" && !campaign.ValidStatus(req.Status) {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}
	if req.TZStrategy != "" && req.TZStrategy != "lead_local" && req.TZStrategy != "campaign_local" {
		writeError(w, http.StatusBadRequest, "invalid tz_strategy")
		return
	}
	if req.CallConstraint != "" && !campaign.ValidCallConstraint(req.CallConstraint) {
		writeError(w, http.StatusBadRequest, "invalid call_constraint")
		return
	}

	patch := campaign.UpdatePatch{
		Name:           req.Name,
		Status:         req.Status,
		Mode:           req.Mode,
		DialRatio:      req.DialRatio,
		MaxAbandonPct:  req.MaxAbandonPct,
		PromptAudio:    req.PromptAudio,
		TransferDest:   req.TransferDest,
		CallerIDPool:   req.CallerIDPool,
		RetryPolicy:    req.RetryPolicy,
		CallingHours:   req.CallingHours,
		TZStrategy:     req.TZStrategy,
		CallConstraint: req.CallConstraint,
	}

	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID

	var before, after campaign.Campaign
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		before, err = a.cRepo.GetTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		after, err = a.cRepo.UpdateTx(r.Context(), tx, id, patch)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "campaign",
			EntityID:   strconv.FormatInt(id, 10),
			Action:     "update",
			Before:     map[string]any{"name": before.Name, "status": before.Status, "mode": before.Mode},
			After:      map[string]any{"name": after.Name, "status": after.Status, "mode": after.Mode},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, campaignToResponse(after))
}

type campaignStatsResp struct {
	CampaignID     int64   `json:"campaign_id"`
	TotalCalls     int     `json:"total_calls"`
	Completed      int     `json:"completed"`
	Failed         int     `json:"failed"`
	NoAnswer       int     `json:"no_answer"`
	Busy           int     `json:"busy"`
	Voicemail      int     `json:"voicemail"`
	OptOut         int     `json:"opt_out"`
	InFlight       int     `json:"in_flight"`
	AvgDurationS   float64 `json:"avg_duration_seconds"`
	AbandonRatePct float64 `json:"abandon_rate_pct"`
	AbandonLimit   float64 `json:"abandon_limit_pct"`
	LeadCount      int     `json:"lead_count"`
}

func (a *tenantCampaigns) stats(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var resp campaignStatsResp
	resp.CampaignID = id
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		c, err := a.cRepo.GetTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		resp.AbandonLimit = c.MaxAbandonPct

		if err := tx.QueryRow(r.Context(), `
			SELECT
				COUNT(*) FILTER (WHERE TRUE),
				COUNT(*) FILTER (WHERE state = 'completed'),
				COUNT(*) FILTER (WHERE state = 'failed'),
				COUNT(*) FILTER (WHERE state = 'no_answer'),
				COUNT(*) FILTER (WHERE state = 'busy'),
				COUNT(*) FILTER (WHERE state = 'voicemail'),
				COUNT(*) FILTER (WHERE state = 'opt_out'),
				COUNT(*) FILTER (WHERE state NOT IN ('completed','failed','no_answer','busy','voicemail','opt_out')),
				COALESCE(AVG(EXTRACT(EPOCH FROM (ended_at - started_at))) FILTER (WHERE ended_at IS NOT NULL), 0)
			FROM call_state
			WHERE campaign_id = $1
		`, id).Scan(
			&resp.TotalCalls,
			&resp.Completed,
			&resp.Failed,
			&resp.NoAnswer,
			&resp.Busy,
			&resp.Voicemail,
			&resp.OptOut,
			&resp.InFlight,
			&resp.AvgDurationS,
		); err != nil {
			return err
		}

		var abandoned, answered int
		if err := tx.QueryRow(r.Context(), `
			SELECT
				COUNT(*) FILTER (WHERE answered_at IS NULL AND state IN ('completed','failed','no_answer','busy','voicemail','opt_out')
					AND started_at > now() - interval '30 days'),
				COUNT(*) FILTER (WHERE answered_at IS NOT NULL
					AND started_at > now() - interval '30 days')
			FROM call_state
			WHERE campaign_id = $1
		`, id).Scan(&abandoned, &answered); err != nil {
			return err
		}
		if answered > 0 {
			resp.AbandonRatePct = float64(abandoned) / float64(abandoned+answered) * 100.0
		}

		return tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM leads WHERE campaign_id = $1`, id).Scan(&resp.LeadCount)
	})
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}
	if err != nil {
		slog.Error("campaign stats failed", "err", err, "campaign_id", id, "tenant_id", claims.TenantID, "req_id", middleware.GetReqID(r.Context()))
		writeError(w, http.StatusInternalServerError, "stats failed")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *tenantCampaigns) leads(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	q := r.URL.Query()
	limit := atoiOr(q.Get("limit"), 50)
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := atoiOr(q.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var (
		out   []map[string]any
		total int
	)
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		if _, err := a.cRepo.GetTx(r.Context(), tx, id); err != nil {
			return err
		}
		if err := tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM leads WHERE campaign_id = $1`, id).Scan(&total); err != nil {
			return err
		}
		rows, err := tx.Query(r.Context(), `
			SELECT id, phone_e164, dial_destination, first_name, last_name, status, attempts,
			       last_attempt_at, next_eligible_at, created_at,
			       n_calls, n_answered, n_ringed, n_voicemail, n_transferred, n_transfer_completed, n_error, n_went_to_dnc,
			       last_call_time
			FROM leads
			WHERE campaign_id = $1
			ORDER BY id
			LIMIT $2 OFFSET $3
		`, id, limit, offset)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var (
				leadID                                                                                 int64
				phone                                                                                  string
				dialDest, firstName, lastName                                                          *string
				status                                                                                 string
				attempts                                                                               int
				lastAttempt, nextEligible, lastCallTime                                                *time.Time
				createdAt                                                                              time.Time
				nCalls, nAnswered, nRinged, nVoicemail, nTransferred, nTransferCompleted, nError, nDNC int
			)
			if err := rows.Scan(
				&leadID, &phone, &dialDest, &firstName, &lastName, &status, &attempts,
				&lastAttempt, &nextEligible, &createdAt,
				&nCalls, &nAnswered, &nRinged, &nVoicemail, &nTransferred, &nTransferCompleted, &nError, &nDNC,
				&lastCallTime,
			); err != nil {
				return err
			}
			row := map[string]any{
				"id":                   leadID,
				"phone_e164":           phone,
				"status":               status,
				"attempts":             attempts,
				"created_at":           createdAt.Format(time.RFC3339),
				"n_calls":              nCalls,
				"n_answered":           nAnswered,
				"n_ringed":             nRinged,
				"n_voicemail":          nVoicemail,
				"n_transferred":        nTransferred,
				"n_transfer_completed": nTransferCompleted,
				"n_error":              nError,
				"n_went_to_dnc":        nDNC,
			}
			if dialDest != nil {
				row["dial_destination"] = *dialDest
			}
			if firstName != nil {
				row["first_name"] = *firstName
			}
			if lastName != nil {
				row["last_name"] = *lastName
			}
			if lastAttempt != nil {
				row["last_attempt_at"] = lastAttempt.Format(time.RFC3339)
			}
			if nextEligible != nil {
				row["next_eligible_at"] = nextEligible.Format(time.RFC3339)
			}
			if lastCallTime != nil {
				row["last_call_time"] = lastCallTime.Format(time.RFC3339)
			}
			out = append(out, row)
		}
		return rows.Err()
	})
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}
	if err != nil {
		slog.Error("campaign leads failed", "err", err, "campaign_id", id, "req_id", middleware.GetReqID(r.Context()))
		writeError(w, http.StatusInternalServerError, "leads failed")
		return
	}
	if out == nil {
		out = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"leads":  out,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (a *tenantCampaigns) calls(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	limit := atoiOr(r.URL.Query().Get("limit"), 50)
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var out []map[string]any
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		if _, err := a.cRepo.GetTx(r.Context(), tx, id); err != nil {
			return err
		}
		rows, err := tx.Query(r.Context(), `
			SELECT uuid::text, lead_id, state, dialed_number, started_at, answered_at, ended_at, hangup_cause
			FROM call_state
			WHERE campaign_id = $1
			ORDER BY started_at DESC
			LIMIT $2
		`, id, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var (
				uuid        string
				leadID      *int64
				state       string
				phone       string
				startedAt   time.Time
				answeredAt  *time.Time
				endedAt     *time.Time
				hangupCause *string
			)
			if err := rows.Scan(&uuid, &leadID, &state, &phone, &startedAt, &answeredAt, &endedAt, &hangupCause); err != nil {
				return err
			}
			row := map[string]any{
				"uuid":          uuid,
				"state":         state,
				"dialed_number": phone,
				"started_at":    startedAt.Format(time.RFC3339),
			}
			if leadID != nil {
				row["lead_id"] = *leadID
			}
			if answeredAt != nil {
				row["answered_at"] = answeredAt.Format(time.RFC3339)
			}
			if endedAt != nil {
				row["ended_at"] = endedAt.Format(time.RFC3339)
			}
			if hangupCause != nil {
				row["hangup_cause"] = *hangupCause
			}
			out = append(out, row)
		}
		return rows.Err()
	})
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}
	if err != nil {
		slog.Error("campaign calls failed", "err", err, "campaign_id", id, "req_id", middleware.GetReqID(r.Context()))
		writeError(w, http.StatusInternalServerError, "calls failed")
		return
	}
	if out == nil {
		out = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"calls": out})
}
