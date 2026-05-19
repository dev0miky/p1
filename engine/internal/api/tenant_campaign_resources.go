package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"p1/engine/internal/audit"
	"p1/engine/internal/auth"
	"p1/engine/internal/campaign"
	"p1/engine/internal/db"
)

type resourcesResponse struct {
	Sounds  []attachedSoundResp  `json:"sounds"`
	Scripts []attachedScriptResp `json:"scripts"`
	Lists   []attachedListResp   `json:"lists"`
}

type attachedSoundResp struct {
	SoundID    int64  `json:"sound_id"`
	SoundName  string `json:"sound_name"`
	Role       string `json:"role"`
	AttachedAt string `json:"attached_at"`
}

type attachedScriptResp struct {
	ScriptID   int64  `json:"script_id"`
	ScriptName string `json:"script_name"`
	Type       string `json:"type"`
	AttachedAt string `json:"attached_at"`
}

type attachedListResp struct {
	ListID     int64  `json:"list_id"`
	ListName   string `json:"list_name"`
	LeadCount  int    `json:"lead_count"`
	AttachedAt string `json:"attached_at"`
}

func (a *tenantCampaigns) listResources(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var resp resourcesResponse
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		if _, err := a.cRepo.GetTx(r.Context(), tx, id); err != nil {
			return err
		}
		sounds, err := a.cRepo.ListAttachedSoundsTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		for _, s := range sounds {
			resp.Sounds = append(resp.Sounds, attachedSoundResp{SoundID: s.SoundID, SoundName: s.SoundName, Role: s.Role, AttachedAt: s.AttachedAt})
		}
		scripts, err := a.cRepo.ListAttachedScriptsTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		for _, s := range scripts {
			resp.Scripts = append(resp.Scripts, attachedScriptResp{ScriptID: s.ScriptID, ScriptName: s.ScriptName, Type: s.Type, AttachedAt: s.AttachedAt})
		}
		lists, err := a.cRepo.ListAttachedListsTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		for _, l := range lists {
			resp.Lists = append(resp.Lists, attachedListResp{ListID: l.ListID, ListName: l.ListName, LeadCount: l.LeadCount, AttachedAt: l.AttachedAt})
		}
		return nil
	})
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}
	if err != nil {
		slog.Error("campaign resources failed", "err", err, "campaign_id", id, "req_id", middleware.GetReqID(r.Context()))
		writeError(w, http.StatusInternalServerError, "resources failed")
		return
	}
	if resp.Sounds == nil {
		resp.Sounds = []attachedSoundResp{}
	}
	if resp.Scripts == nil {
		resp.Scripts = []attachedScriptResp{}
	}
	if resp.Lists == nil {
		resp.Lists = []attachedListResp{}
	}
	writeJSON(w, http.StatusOK, resp)
}

type attachSoundReq struct {
	SoundID int64  `json:"sound_id"`
	Role    string `json:"role"`
}

func (a *tenantCampaigns) attachSound(w http.ResponseWriter, r *http.Request) {
	campID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req attachSoundReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.SoundID == 0 {
		writeError(w, http.StatusBadRequest, "sound_id required")
		return
	}
	if !campaign.ValidSoundRole(req.Role) {
		writeError(w, http.StatusBadRequest, "invalid role (greeting|voicemail|hold|whisper|opt_out_confirm)")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		if _, err := a.cRepo.GetTx(r.Context(), tx, campID); err != nil {
			return err
		}
		if err := a.cRepo.AttachSoundTx(r.Context(), tx, campID, req.SoundID, req.Role); err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "campaign",
			EntityID:   strconv.FormatInt(campID, 10),
			Action:     "attach_sound",
			After:      map[string]any{"sound_id": req.SoundID, "role": req.Role},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}
	if err != nil {
		slog.Error("attach sound failed", "err", err, "campaign_id", campID, "sound_id", req.SoundID)
		writeError(w, http.StatusInternalServerError, "attach failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *tenantCampaigns) detachSound(w http.ResponseWriter, r *http.Request) {
	campID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	soundID, err := strconv.ParseInt(chi.URLParam(r, "sound_id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid sound_id")
		return
	}
	role := r.URL.Query().Get("role")
	if !campaign.ValidSoundRole(role) {
		writeError(w, http.StatusBadRequest, "role required")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		return a.cRepo.DetachSoundTx(r.Context(), tx, campID, soundID, role)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "detach failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attachScriptReq struct {
	ScriptID int64 `json:"script_id"`
}

func (a *tenantCampaigns) attachScript(w http.ResponseWriter, r *http.Request) {
	campID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req attachScriptReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ScriptID == 0 {
		writeError(w, http.StatusBadRequest, "script_id required")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		if _, err := a.cRepo.GetTx(r.Context(), tx, campID); err != nil {
			return err
		}
		return a.cRepo.AttachScriptTx(r.Context(), tx, campID, req.ScriptID)
	})
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}
	if err != nil {
		slog.Error("attach script failed", "err", err)
		writeError(w, http.StatusInternalServerError, "attach failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *tenantCampaigns) detachScript(w http.ResponseWriter, r *http.Request) {
	campID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	scriptID, err := strconv.ParseInt(chi.URLParam(r, "script_id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid script_id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		return a.cRepo.DetachScriptTx(r.Context(), tx, campID, scriptID)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "detach failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attachListReq struct {
	ListID int64 `json:"list_id"`
}

func (a *tenantCampaigns) attachList(w http.ResponseWriter, r *http.Request) {
	campID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req attachListReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ListID == 0 {
		writeError(w, http.StatusBadRequest, "list_id required")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		if _, err := a.cRepo.GetTx(r.Context(), tx, campID); err != nil {
			return err
		}
		return a.cRepo.AttachListTx(r.Context(), tx, campID, req.ListID)
	})
	if errors.Is(err, campaign.ErrNotFound) {
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}
	if err != nil {
		slog.Error("attach list failed", "err", err)
		writeError(w, http.StatusInternalServerError, "attach failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *tenantCampaigns) detachList(w http.ResponseWriter, r *http.Request) {
	campID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	listID, err := strconv.ParseInt(chi.URLParam(r, "list_id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid list_id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		return a.cRepo.DetachListTx(r.Context(), tx, campID, listID)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "detach failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
