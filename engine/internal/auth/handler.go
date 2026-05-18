package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"p1/engine/internal/tenant"
)

type Handler struct {
	repo *tenant.Repo
	iss  *Issuer
}

func NewHandler(repo *tenant.Repo, iss *Issuer) *Handler {
	return &Handler{repo: repo, iss: iss}
}

type loginRequest struct {
	TenantSlug string `json:"tenant_slug"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	TOTP       string `json:"totp"`
}

type loginResponse struct {
	Token    string `json:"token"`
	UserID   int64  `json:"user_id"`
	TenantID *int64 `json:"tenant_id,omitempty"`
	Role     string `json:"role"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password required")
		return
	}

	user, err := h.repo.FindUserForLogin(r.Context(), req.TenantSlug, req.Email)
	if errors.Is(err, tenant.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup failed")
		return
	}

	if !Verify(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if user.Status != "active" {
		writeError(w, http.StatusForbidden, "account "+user.Status)
		return
	}

	var tenantID int64
	if user.TenantID != nil {
		tenantID = *user.TenantID
	}

	token, err := h.iss.Issue(Claims{
		UserID:   user.ID,
		TenantID: tenantID,
		Role:     user.Role,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token issue failed")
		return
	}

	_ = h.repo.MarkUserLoggedIn(r.Context(), user.ID)

	writeJSON(w, http.StatusOK, loginResponse{
		Token:    token,
		UserID:   user.ID,
		TenantID: user.TenantID,
		Role:     user.Role,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
