package api

import (
	"crypto/rand"
	"encoding/base64"
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

type adminUsers struct {
	repo *tenant.Repo
}

type createUserRequest struct {
	TenantID *int64 `json:"tenant_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

type updateUserRequest struct {
	Role   string `json:"role"`
	Status string `json:"status"`
}

type userResponse struct {
	ID        int64   `json:"id"`
	TenantID  *int64  `json:"tenant_id,omitempty"`
	Email     string  `json:"email"`
	Role      string  `json:"role"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	TempPassword string `json:"temp_password,omitempty"`
}

func userToResponse(u tenant.User, tempPassword string) userResponse {
	r := userResponse{
		ID:        u.ID,
		TenantID:  u.TenantID,
		Email:     u.Email,
		Role:      u.Role,
		Status:    u.Status,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: u.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		TempPassword: tempPassword,
	}
	return r
}

func (a *adminUsers) create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
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
	if req.Role == "super_admin" && req.TenantID != nil {
		writeError(w, http.StatusBadRequest, "super_admin must not have tenant_id")
		return
	}
	if req.Role != "super_admin" && req.TenantID == nil {
		writeError(w, http.StatusBadRequest, "non-super_admin requires tenant_id")
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

	var created tenant.User
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role}, func(tx pgx.Tx) error {
		var err error
		created, err = a.repo.CreateUserTx(r.Context(), tx, tenant.User{
			TenantID: req.TenantID, Email: req.Email, Role: req.Role, PasswordHash: hash,
		})
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   created.TenantID,
			EntityType: "user",
			EntityID:   strconv.FormatInt(created.ID, 10),
			Action:     "create",
			After:      map[string]any{"email": created.Email, "role": created.Role, "tenant_id": created.TenantID},
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

func (a *adminUsers) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var users []tenant.User
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role}, func(tx pgx.Tx) error {
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

func (a *adminUsers) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var u tenant.User
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role}, func(tx pgx.Tx) error {
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

func (a *adminUsers) update(w http.ResponseWriter, r *http.Request) {
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
	var before, after tenant.User
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role}, func(tx pgx.Tx) error {
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
			TenantID:   after.TenantID,
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

func generatePassword(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:n], nil
}
