package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/fsm"
	"p1/engine/internal/tenant"
)

type tenantCalls struct {
	repo *tenant.Repo
	fsm  *fsm.Repo
}

type callResponse struct {
	UUID         string          `json:"uuid"`
	TenantID     int64           `json:"tenant_id"`
	CampaignID   *int64          `json:"campaign_id,omitempty"`
	LeadID       *int64          `json:"lead_id,omitempty"`
	State        string          `json:"state"`
	DialedNumber string          `json:"dialed_number"`
	CallerID     *string         `json:"caller_id,omitempty"`
	HangupCause  *string         `json:"hangup_cause,omitempty"`
	AMDResult    *string         `json:"amd_result,omitempty"`
	StartedAt    string          `json:"started_at"`
	AnsweredAt   *string         `json:"answered_at,omitempty"`
	EndedAt      *string         `json:"ended_at,omitempty"`
	Metadata     json.RawMessage `json:"metadata"`
}

func callToResponse(c fsm.Call) callResponse {
	r := callResponse{
		UUID:         c.UUID,
		TenantID:     c.TenantID,
		CampaignID:   c.CampaignID,
		LeadID:       c.LeadID,
		State:        string(c.State),
		DialedNumber: c.DialedNumber,
		CallerID:     c.CallerID,
		HangupCause:  c.HangupCause,
		AMDResult:    c.AMDResult,
		StartedAt:    c.StartedAt.Format(time.RFC3339),
		Metadata:     c.Metadata,
	}
	if c.AnsweredAt != nil {
		s := c.AnsweredAt.Format(time.RFC3339)
		r.AnsweredAt = &s
	}
	if c.EndedAt != nil {
		s := c.EndedAt.Format(time.RFC3339)
		r.EndedAt = &s
	}
	return r
}

func (a *tenantCalls) recent(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	limit := atoiOr(r.URL.Query().Get("limit"), 50)

	var list []fsm.Call
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		list, err = a.fsm.ListRecentTx(r.Context(), tx, limit)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]callResponse, len(list))
	for i, c := range list {
		out[i] = callToResponse(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"calls": out, "total": len(out)})
}

func (a *tenantCalls) stats(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	since := atoiOr(r.URL.Query().Get("minutes"), 60)

	var stats []fsm.StateCount
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		stats, err = a.fsm.CountByStateTx(r.Context(), tx, since)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stats failed")
		return
	}
	byState := make(map[string]int, len(stats))
	total := 0
	for _, s := range stats {
		byState[string(s.State)] = s.Count
		total += s.Count
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"window_minutes": since,
		"total":          total,
		"by_state":       byState,
	})
}
