package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"p1/engine/internal/audit"
	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/dnc"
)

const maxBulkIDs = 1000

type tenantLeadsBulk struct {
	tenantLeads *tenantLeads
	dRepo       *dnc.Repo
}

type bulkIDsReq struct {
	LeadIDs []int64 `json:"lead_ids"`
}

func decodeBulkIDs(r *http.Request, dst any) (bool, string) {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return false, "invalid json"
	}
	// Reflectively validate via type assertion against the embedded slice.
	type idsHolder interface {
		ids() []int64
	}
	if h, ok := dst.(idsHolder); ok {
		ids := h.ids()
		if len(ids) == 0 {
			return false, "lead_ids required"
		}
		if len(ids) > maxBulkIDs {
			return false, "too many lead_ids (max " + strconv.Itoa(maxBulkIDs) + ")"
		}
	}
	return true, ""
}

func (b *bulkIDsReq) ids() []int64 { return b.LeadIDs }

// POST /tenant/leads/bulk/delete
func (a *tenantLeadsBulk) delete(w http.ResponseWriter, r *http.Request) {
	var req bulkIDsReq
	if ok, msg := decodeBulkIDs(r, &req); !ok {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}

	var count int64
	err := db.WithCtx(r.Context(), a.tenantLeads.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `DELETE FROM leads WHERE id = ANY($1)`, req.LeadIDs)
		if err != nil {
			return err
		}
		count = tag.RowsAffected()
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "lead",
			EntityID:   "bulk",
			Action:     "bulk_delete",
			After:      map[string]any{"requested": len(req.LeadIDs), "deleted": count},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		slog.Error("bulk delete failed", "err", err, "tenant", tid)
		writeError(w, http.StatusInternalServerError, "bulk delete failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": count, "requested": len(req.LeadIDs)})
}

type bulkAttachReq struct {
	LeadIDs       []int64 `json:"lead_ids"`
	CampaignID    *int64  `json:"campaign_id"`
	HasCampaignID bool    `json:"-"`
}

func (b *bulkAttachReq) ids() []int64 { return b.LeadIDs }

func (req *bulkAttachReq) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["lead_ids"]; ok {
		if err := json.Unmarshal(v, &req.LeadIDs); err != nil {
			return err
		}
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
	return nil
}

// POST /tenant/leads/bulk/attach
func (a *tenantLeadsBulk) attach(w http.ResponseWriter, r *http.Request) {
	var req bulkAttachReq
	if ok, msg := decodeBulkIDs(r, &req); !ok {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if !req.HasCampaignID {
		writeError(w, http.StatusBadRequest, "campaign_id required (use null to detach)")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}

	var count int64
	err := db.WithCtx(r.Context(), a.tenantLeads.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `
			UPDATE leads SET campaign_id = $1, updated_at = now()
			WHERE id = ANY($2)
		`, req.CampaignID, req.LeadIDs)
		if err != nil {
			return err
		}
		count = tag.RowsAffected()
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "lead",
			EntityID:   "bulk",
			Action:     "bulk_attach",
			After:      map[string]any{"requested": len(req.LeadIDs), "updated": count, "campaign_id": req.CampaignID},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		slog.Error("bulk attach failed", "err", err, "tenant", tid)
		writeError(w, http.StatusInternalServerError, "bulk attach failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": count, "requested": len(req.LeadIDs)})
}

type bulkDNCReq struct {
	LeadIDs []int64 `json:"lead_ids"`
	Reason  string  `json:"reason"`
}

func (b *bulkDNCReq) ids() []int64 { return b.LeadIDs }

// POST /tenant/leads/bulk/dnc
func (a *tenantLeadsBulk) markDNC(w http.ResponseWriter, r *http.Request) {
	var req bulkDNCReq
	if ok, msg := decodeBulkIDs(r, &req); !ok {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	reason := req.Reason
	if reason == "" {
		reason = "bulk-marked via console"
	}

	var added, marked int64
	err := db.WithCtx(r.Context(), a.tenantLeads.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		// Insert a dnc_entries row (scope=internal) per distinct phone, idempotent on the unique index.
		tag, err := tx.Exec(r.Context(), `
			INSERT INTO dnc_entries (tenant_id, scope, phone_e164, source, reason)
			SELECT DISTINCT $1::bigint, 'internal', l.phone_e164, 'console', $2::text
			FROM leads l
			WHERE l.id = ANY($3::bigint[])
			ON CONFLICT DO NOTHING
		`, tid, reason, req.LeadIDs)
		if err != nil {
			return err
		}
		added = tag.RowsAffected()

		// Mark the leads themselves as status=dnc so the dialer skips them.
		tag2, err := tx.Exec(r.Context(), `
			UPDATE leads SET status='dnc', updated_at=now()
			WHERE id = ANY($1) AND status != 'dnc'
		`, req.LeadIDs)
		if err != nil {
			return err
		}
		marked = tag2.RowsAffected()

		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "lead",
			EntityID:   "bulk",
			Action:     "bulk_dnc",
			After:      map[string]any{"requested": len(req.LeadIDs), "dnc_added": added, "leads_marked": marked, "reason": reason},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		slog.Error("bulk dnc failed", "err", err, "tenant", tid)
		writeError(w, http.StatusInternalServerError, "bulk dnc failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"requested":    len(req.LeadIDs),
		"dnc_added":    added,
		"leads_marked": marked,
	})
}

// Use errors.Is to keep golangci happy (no unused import warnings).
var _ = errors.Is
