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
	"p1/engine/internal/campaign"
	"p1/engine/internal/db"
	"p1/engine/internal/tenant"
)

type tenantCampaigns struct {
	repo  *tenant.Repo
	cRepo *campaign.Repo
}

type createCampaignRequest struct {
	Name          string          `json:"name"`
	Mode          string          `json:"mode"`
	DialRatio     *float64        `json:"dial_ratio"`
	MaxAbandonPct *float64        `json:"max_abandon_pct"`
	PromptAudio   *string         `json:"prompt_audio"`
	TransferDest  *string         `json:"transfer_dest"`
	CallerIDPool  json.RawMessage `json:"caller_id_pool"`
	RetryPolicy   json.RawMessage `json:"retry_policy"`
	CallingHours  json.RawMessage `json:"calling_hours"`
	TZStrategy    string          `json:"tz_strategy"`
}

type updateCampaignRequest = createCampaignRequest

type campaignResponse struct {
	ID            int64           `json:"id"`
	TenantID      int64           `json:"tenant_id"`
	Name          string          `json:"name"`
	Mode          string          `json:"mode"`
	Status        string          `json:"status"`
	DialRatio     float64         `json:"dial_ratio"`
	MaxAbandonPct float64         `json:"max_abandon_pct"`
	PromptAudio   *string         `json:"prompt_audio,omitempty"`
	TransferDest  *string         `json:"transfer_dest,omitempty"`
	CallerIDPool  json.RawMessage `json:"caller_id_pool"`
	RetryPolicy   json.RawMessage `json:"retry_policy"`
	CallingHours  json.RawMessage `json:"calling_hours"`
	TZStrategy    string          `json:"tz_strategy"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

func campaignToResponse(c campaign.Campaign) campaignResponse {
	return campaignResponse{
		ID:            c.ID,
		TenantID:      c.TenantID,
		Name:          c.Name,
		Mode:          string(c.Mode),
		Status:        string(c.Status),
		DialRatio:     c.DialRatio,
		MaxAbandonPct: c.MaxAbandonPct,
		PromptAudio:   c.PromptAudio,
		TransferDest:  c.TransferDest,
		CallerIDPool:  c.CallerIDPool,
		RetryPolicy:   c.RetryPolicy,
		CallingHours:  c.CallingHours,
		TZStrategy:    c.TZStrategy,
		CreatedAt:     c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     c.UpdatedAt.Format(time.RFC3339),
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

	patch := campaign.UpdatePatch{
		Name:          req.Name,
		Status:        req.Status,
		Mode:          req.Mode,
		DialRatio:     req.DialRatio,
		MaxAbandonPct: req.MaxAbandonPct,
		PromptAudio:   req.PromptAudio,
		TransferDest:  req.TransferDest,
		CallerIDPool:  req.CallerIDPool,
		RetryPolicy:   req.RetryPolicy,
		CallingHours:  req.CallingHours,
		TZStrategy:    req.TZStrategy,
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
