package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"p1/engine/internal/audit"
	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/tenant"
)

type tenantUsers struct {
	repo *tenant.Repo
}

type tenantCreateUserRequest struct {
	Email    string `json:"email"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

func (a *tenantUsers) create(w http.ResponseWriter, r *http.Request) {
	var req tenantCreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Email == "" || req.Role == "" {
		writeError(w, http.StatusBadRequest, "email and role required")
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeError(w, http.StatusBadRequest, "invalid email")
		return
	}
	if _, err := auth.ParseRole(req.Role); err != nil {
		writeError(w, http.StatusBadRequest, "invalid role")
		return
	}
	if req.Role == "super_admin" {
		writeError(w, http.StatusForbidden, "cannot create super_admin from tenant context")
		return
	}

	password := req.Password
	tempIssued := false
	if password == "" {
		p, err := generatePassword(20)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "password gen failed")
			return
		}
		password = p
		tempIssued = true
	}
	if len(password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 chars")
		return
	}
	hash, err := auth.Hash(password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash failed")
		return
	}

	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID

	var created tenant.User
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		created, err = a.repo.CreateUserTx(r.Context(), tx, tenant.User{
			TenantID: &tid, Email: req.Email, Role: req.Role, PasswordHash: hash,
		})
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "user",
			EntityID:   strconv.FormatInt(created.ID, 10),
			Action:     "create",
			After:      map[string]any{"email": created.Email, "role": created.Role},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "email already exists in tenant")
			return
		}
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}

	resp := userToResponse(created, "")
	if tempIssued {
		resp.TempPassword = password
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (a *tenantUsers) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var users []tenant.User
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		users, err = a.repo.ListUsersTx(r.Context(), tx)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]userResponse, len(users))
	for i, u := range users {
		out[i] = userToResponse(u, "")
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (a *tenantUsers) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var u tenant.User
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		u, err = a.repo.GetUserByIDTx(r.Context(), tx, id)
		return err
	})
	if errors.Is(err, tenant.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}
	writeJSON(w, http.StatusOK, userToResponse(u, ""))
}

func (a *tenantUsers) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Role == "" && req.Status == "" {
		writeError(w, http.StatusBadRequest, "no fields to update")
		return
	}
	if req.Role != "" {
		if req.Role == "super_admin" {
			writeError(w, http.StatusForbidden, "cannot grant super_admin from tenant context")
			return
		}
		if _, err := auth.ParseRole(req.Role); err != nil {
			writeError(w, http.StatusBadRequest, "invalid role")
			return
		}
	}
	if req.Status != "" && req.Status != "active" && req.Status != "suspended" {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}

	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	var before, after tenant.User
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		before, err = a.repo.GetUserByIDTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		after, err = a.repo.UpdateUserTx(r.Context(), tx, id, req.Role, req.Status)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "user",
			EntityID:   strconv.FormatInt(id, 10),
			Action:     "update",
			Before:     map[string]any{"role": before.Role, "status": before.Status},
			After:      map[string]any{"role": after.Role, "status": after.Status},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if errors.Is(err, tenant.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, userToResponse(after, ""))
}
