package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"p1/engine/internal/audit"
	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/fsxml"
	"p1/engine/internal/gateway"
)

var nameRE = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

type adminGateways struct {
	repo *gateway.Repo
	pool *db.Pool
	prov *gateway.Provisioner
	log  *slog.Logger
}

type gatewayResponse struct {
	ID               int64             `json:"id"`
	Name             string            `json:"name"`
	Description      *string           `json:"description"`
	Proxy            string            `json:"proxy"`
	Register         bool              `json:"register"`
	Username         *string           `json:"username"`
	HasPassword      bool              `json:"has_password"`
	Realm            *string           `json:"realm"`
	FromUser         *string           `json:"from_user"`
	FromDomain       *string           `json:"from_domain"`
	Transport        string            `json:"transport"`
	ExpireSeconds    int               `json:"expire_seconds"`
	RetrySeconds     int               `json:"retry_seconds"`
	CallerIDInFrom   bool              `json:"caller_id_in_from"`
	ExtraParams      map[string]string `json:"extra_params"`
	Enabled          bool              `json:"enabled"`
	IsActive         bool              `json:"is_active"`
	RegisterStatus   string            `json:"register_status"`
	RegisterStatusAt *string           `json:"register_status_at"`
	CreatedAt        string            `json:"created_at"`
	UpdatedAt        string            `json:"updated_at"`
}

func toGatewayResponse(g gateway.Gateway) gatewayResponse {
	r := gatewayResponse{
		ID:             g.ID,
		Name:           g.Name,
		Description:    g.Description,
		Proxy:          g.Proxy,
		Register:       g.Register,
		Username:       g.Username,
		HasPassword:    g.HasPassword,
		Realm:          g.Realm,
		FromUser:       g.FromUser,
		FromDomain:     g.FromDomain,
		Transport:      string(g.Transport),
		ExpireSeconds:  g.ExpireSeconds,
		RetrySeconds:   g.RetrySeconds,
		CallerIDInFrom: g.CallerIDInFrom,
		ExtraParams:    g.ExtraParams,
		Enabled:        g.Enabled,
		IsActive:       g.IsActive,
		RegisterStatus: g.RegisterStatus,
		CreatedAt:      g.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      g.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if g.RegisterStatusAt != nil {
		s := g.RegisterStatusAt.Format("2006-01-02T15:04:05Z07:00")
		r.RegisterStatusAt = &s
	}
	if r.ExtraParams == nil {
		r.ExtraParams = map[string]string{}
	}
	return r
}

type createGatewayRequest struct {
	Name           string            `json:"name"`
	Description    *string           `json:"description"`
	Proxy          string            `json:"proxy"`
	Register       bool              `json:"register"`
	Username       *string           `json:"username"`
	Password       *string           `json:"password"`
	Realm          *string           `json:"realm"`
	FromUser       *string           `json:"from_user"`
	FromDomain     *string           `json:"from_domain"`
	Transport      string            `json:"transport"`
	ExpireSeconds  int               `json:"expire_seconds"`
	RetrySeconds   int               `json:"retry_seconds"`
	CallerIDInFrom bool              `json:"caller_id_in_from"`
	ExtraParams    map[string]string `json:"extra_params"`
	Enabled        bool              `json:"enabled"`
}

func (a *adminGateways) create(w http.ResponseWriter, r *http.Request) {
	var req createGatewayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !nameRE.MatchString(req.Name) {
		writeError(w, http.StatusBadRequest, "name must match ^[a-z0-9_-]{1,64}$")
		return
	}
	if req.Proxy == "" {
		writeError(w, http.StatusBadRequest, "proxy required")
		return
	}
	if req.Transport == "" {
		req.Transport = "udp"
	}
	if !gateway.ValidTransport(req.Transport) {
		writeError(w, http.StatusBadRequest, "transport must be udp, tcp, or tls")
		return
	}
	if req.ExpireSeconds <= 0 {
		req.ExpireSeconds = 3600
	}
	if req.RetrySeconds <= 0 {
		req.RetrySeconds = 30
	}

	claims, _ := auth.ClaimsFromContext(r.Context())

	g := gateway.Gateway{
		Name:           req.Name,
		Description:    req.Description,
		Proxy:          req.Proxy,
		Register:       req.Register,
		Username:       req.Username,
		Password:       req.Password,
		Realm:          req.Realm,
		FromUser:       req.FromUser,
		FromDomain:     req.FromDomain,
		Transport:      gateway.Transport(req.Transport),
		ExpireSeconds:  req.ExpireSeconds,
		RetrySeconds:   req.RetrySeconds,
		CallerIDInFrom: req.CallerIDInFrom,
		ExtraParams:    req.ExtraParams,
		Enabled:        req.Enabled,
	}

	var created gateway.Gateway
	err := db.WithCtx(r.Context(), a.pool, db.Ctx{Role: claims.Role, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		created, err = a.repo.CreateTx(r.Context(), tx, g)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			EntityType: "gateway",
			EntityID:   strconv.FormatInt(created.ID, 10),
			Action:     "create",
			After:      created,
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "gateway name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}

	if a.prov != nil {
		if err := a.prov.Reload(r.Context(), created.Name); err != nil {
			a.log.Error("provisioner reload after create", "gateway", created.Name, "err", err)
		}
	}

	writeJSON(w, http.StatusCreated, toGatewayResponse(created))
}

func (a *adminGateways) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var list []gateway.Gateway
	err := db.WithCtx(r.Context(), a.pool, db.Ctx{Role: claims.Role}, func(tx pgx.Tx) error {
		var err error
		list, err = a.repo.ListTx(r.Context(), tx)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]gatewayResponse, len(list))
	for i, g := range list {
		out[i] = toGatewayResponse(g)
	}
	writeJSON(w, http.StatusOK, map[string]any{"gateways": out})
}

func (a *adminGateways) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var g gateway.Gateway
	err = db.WithCtx(r.Context(), a.pool, db.Ctx{Role: claims.Role}, func(tx pgx.Tx) error {
		var err error
		g, err = a.repo.GetTx(r.Context(), tx, id)
		return err
	})
	if errors.Is(err, gateway.ErrNotFound) {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}
	writeJSON(w, http.StatusOK, toGatewayResponse(g))
}

// updateGatewayRequest uses *string for optional fields; password only applied when non-empty string present.
type updateGatewayRequest struct {
	Description    *string           `json:"description"`
	Proxy          string            `json:"proxy"`
	Register       *bool             `json:"register"`
	Username       *string           `json:"username"`
	Password       *string           `json:"password"`
	Realm          *string           `json:"realm"`
	FromUser       *string           `json:"from_user"`
	FromDomain     *string           `json:"from_domain"`
	Transport      string            `json:"transport"`
	ExpireSeconds  *int              `json:"expire_seconds"`
	RetrySeconds   *int              `json:"retry_seconds"`
	CallerIDInFrom *bool             `json:"caller_id_in_from"`
	ExtraParams    map[string]string `json:"extra_params"`
	Enabled        *bool             `json:"enabled"`
}

func (a *adminGateways) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req updateGatewayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Transport != "" && !gateway.ValidTransport(req.Transport) {
		writeError(w, http.StatusBadRequest, "transport must be udp, tcp, or tls")
		return
	}

	patch := gateway.UpdatePatch{
		Description:    req.Description,
		Proxy:          req.Proxy,
		Register:       req.Register,
		Username:       req.Username,
		Realm:          req.Realm,
		FromUser:       req.FromUser,
		FromDomain:     req.FromDomain,
		Transport:      req.Transport,
		ExpireSeconds:  req.ExpireSeconds,
		RetrySeconds:   req.RetrySeconds,
		CallerIDInFrom: req.CallerIDInFrom,
		Enabled:        req.Enabled,
	}
	// only update password when explicitly provided as non-empty string
	if req.Password != nil && *req.Password != "" {
		patch.Password = req.Password
	}
	if req.ExtraParams != nil {
		patch.ExtraParams = req.ExtraParams
		patch.SetExtraParams = true
	}

	claims, _ := auth.ClaimsFromContext(r.Context())

	var before, after gateway.Gateway
	err = db.WithCtx(r.Context(), a.pool, db.Ctx{Role: claims.Role, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		before, err = a.repo.GetTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		after, err = a.repo.UpdateTx(r.Context(), tx, id, patch)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			EntityType: "gateway",
			EntityID:   strconv.FormatInt(id, 10),
			Action:     "update",
			Before:     before,
			After:      after,
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if errors.Is(err, gateway.ErrNotFound) {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}

	if a.prov != nil {
		if err := a.prov.Reload(r.Context(), after.Name); err != nil {
			a.log.Error("provisioner reload after update", "gateway", after.Name, "err", err)
		}
	}

	writeJSON(w, http.StatusOK, toGatewayResponse(after))
}

func (a *adminGateways) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())

	var name string
	err = db.WithCtx(r.Context(), a.pool, db.Ctx{Role: claims.Role, UserID: claims.UserID}, func(tx pgx.Tx) error {
		g, err := a.repo.GetTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		name = g.Name
		if err := a.repo.DeleteTx(r.Context(), tx, id); err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			EntityType: "gateway",
			EntityID:   strconv.FormatInt(id, 10),
			Action:     "delete",
			Before:     g,
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if errors.Is(err, gateway.ErrNotFound) {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}

	if a.prov != nil {
		if err := a.prov.Reload(r.Context(), name); err != nil {
			a.log.Error("provisioner reload after delete", "gateway", name, "err", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *adminGateways) activate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())

	err = db.WithCtx(r.Context(), a.pool, db.Ctx{Role: claims.Role, UserID: claims.UserID}, func(tx pgx.Tx) error {
		if err := a.repo.ActivateTx(r.Context(), tx, id); err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			EntityType: "gateway",
			EntityID:   strconv.FormatInt(id, 10),
			Action:     "activate",
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if errors.Is(err, gateway.ErrNotFound) {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "activate failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"activated": true})
}

func (a *adminGateways) register(w http.ResponseWriter, r *http.Request) {
	if a.prov == nil {
		writeError(w, http.StatusServiceUnavailable, "provisioner not available")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())

	var g gateway.Gateway
	err = db.WithCtx(r.Context(), a.pool, db.Ctx{Role: claims.Role}, func(tx pgx.Tx) error {
		var err error
		g, err = a.repo.GetTx(r.Context(), tx, id)
		return err
	})
	if errors.Is(err, gateway.ErrNotFound) {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}

	status, err := a.prov.Status(r.Context(), g.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "status check failed")
		return
	}

	err = db.WithCtx(r.Context(), a.pool, db.Ctx{Role: claims.Role}, func(tx pgx.Tx) error {
		return a.repo.SetStatusTx(r.Context(), tx, g.Name, status)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "set status failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"register_status": status})
}

// fsXML serves FreeSWITCH mod_xml_curl requests. Not JWT-authed — gated by shared secret via Basic auth password or X-FS-Secret header.
type fsXML struct {
	repo   *gateway.Repo
	pool   *db.Pool
	secret string
}

func (f *fsXML) handle(w http.ResponseWriter, r *http.Request) {
	if f.secret == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var provided string
	if _, pw, ok := r.BasicAuth(); ok {
		provided = pw
	} else if h := r.Header.Get("X-FS-Secret"); h != "" {
		provided = h
	}

	if subtle.ConstantTimeCompare([]byte(provided), []byte(f.secret)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.Write([]byte(fsxml.NotFound()))
		return
	}

	section := r.FormValue("section")
	keyValue := r.FormValue("key_value")

	if section != "configuration" || keyValue != "sofia.conf" {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.Write([]byte(fsxml.NotFound()))
		return
	}

	ctx := r.Context()
	var gws []gateway.Gateway
	err := db.WithCtx(ctx, f.pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		var err error
		gws, err = f.repo.ListEnabledWithSecretTx(ctx, tx)
		return err
	})
	if err != nil {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.Write([]byte(fsxml.NotFound()))
		return
	}

	views := make([]fsxml.GatewayView, 0, len(gws))
	for _, g := range gws {
		v := fsxml.GatewayView{
			Name:           g.Name,
			Proxy:          g.Proxy,
			Register:       g.Register,
			Transport:      string(g.Transport),
			CallerIDInFrom: g.CallerIDInFrom,
			ExpireSeconds:  g.ExpireSeconds,
			RetrySeconds:   g.RetrySeconds,
			Extra:          g.ExtraParams,
		}
		if g.Username != nil {
			v.Username = *g.Username
		}
		if g.Password != nil {
			v.Password = *g.Password
		}
		if g.Realm != nil {
			v.Realm = *g.Realm
		}
		if g.FromUser != nil {
			v.FromUser = *g.FromUser
		}
		if g.FromDomain != nil {
			v.FromDomain = *g.FromDomain
		}
		views = append(views, v)
	}

	out, err := fsxml.RenderSofia(views)
	if err != nil {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.Write([]byte(fsxml.NotFound()))
		return
	}

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write([]byte(out))
}
