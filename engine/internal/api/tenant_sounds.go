package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"p1/engine/internal/audit"
	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/sound"
	"p1/engine/internal/tenant"
)

type tenantSounds struct {
	repo    *tenant.Repo
	sRepo   *sound.Repo
	storage *sound.Storage
}

type soundResponse struct {
	ID          int64   `json:"id"`
	TenantID    int64   `json:"tenant_id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	MimeType    string  `json:"mime_type"`
	SizeBytes   int64   `json:"size_bytes"`
	DurationMS  *int    `json:"duration_ms,omitempty"`
	SHA256      *string `json:"sha256,omitempty"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func soundToResponse(s sound.Sound) soundResponse {
	return soundResponse{
		ID:          s.ID,
		TenantID:    s.TenantID,
		Name:        s.Name,
		Description: s.Description,
		MimeType:    s.MimeType,
		SizeBytes:   s.SizeBytes,
		DurationMS:  s.DurationMS,
		SHA256:      s.SHA256,
		Status:      string(s.Status),
		CreatedAt:   s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   s.UpdatedAt.Format(time.RFC3339),
	}
}

const maxSoundBytes = 25 << 20 // 25 MB

var allowedMimes = map[string]string{
	"audio/wav":                ".wav",
	"audio/wave":               ".wav",
	"audio/x-wav":              ".wav",
	"audio/mpeg":               ".mp3",
	"audio/mp3":                ".mp3",
	"audio/ogg":                ".ogg",
	"application/octet-stream": "",
}

func extensionForUpload(mime, originalFilename string) (string, bool) {
	if ext := allowedMimes[mime]; ext != "" {
		return ext, true
	}
	switch strings.ToLower(filepath.Ext(originalFilename)) {
	case ".wav":
		return ".wav", true
	case ".mp3":
		return ".mp3", true
	case ".ogg":
		return ".ogg", true
	}
	return "", false
}

func mimeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".wav":
		return "audio/wav"
	case ".mp3":
		return "audio/mpeg"
	case ".ogg":
		return "audio/ogg"
	}
	return "application/octet-stream"
}

func (a *tenantSounds) create(w http.ResponseWriter, r *http.Request) {
	if a.storage == nil {
		writeError(w, http.StatusInternalServerError, "sound storage not configured")
		return
	}
	if err := r.ParseMultipartForm(maxSoundBytes + (1 << 20)); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	desc := strings.TrimSpace(r.FormValue("description"))
	var descPtr *string
	if desc != "" {
		descPtr = &desc
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file required")
		return
	}
	defer func() { _ = file.Close() }()

	if header.Size > maxSoundBytes {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("file too large (max %d bytes)", maxSoundBytes))
		return
	}

	declaredMime := header.Header.Get("Content-Type")
	ext, ok := extensionForUpload(declaredMime, header.Filename)
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported audio type — wav, mp3, ogg only")
		return
	}
	mime := mimeForExt(ext)

	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}

	fileKey := uuid.NewString() + ext
	res, err := a.storage.Write(tid, fileKey, io.LimitReader(file, maxSoundBytes))
	if err != nil {
		slog.Error("sound storage write failed", "err", err, "tenant", tid, "key", fileKey)
		writeError(w, http.StatusInternalServerError, "storage write failed")
		return
	}

	s := sound.Sound{
		TenantID:    tid,
		Name:        name,
		Description: descPtr,
		FileKey:     fileKey,
		MimeType:    mime,
		SizeBytes:   res.Size,
		SHA256:      &res.SHA256,
		Status:      sound.StatusReady,
	}

	var created sound.Sound
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		created, err = a.sRepo.CreateTx(r.Context(), tx, s)
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "sound",
			EntityID:   strconv.FormatInt(created.ID, 10),
			Action:     "create",
			After:      map[string]any{"name": created.Name, "size_bytes": created.SizeBytes},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if err != nil {
		_ = a.storage.Delete(tid, fileKey)
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "sound name already exists")
			return
		}
		slog.Error("sound create failed", "err", err, "tenant", tid)
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, soundToResponse(created))
}

func (a *tenantSounds) list(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var list []sound.Sound
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		list, err = a.sRepo.ListTx(r.Context(), tx)
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]soundResponse, len(list))
	for i, s := range list {
		out[i] = soundToResponse(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sounds": out})
}

func (a *tenantSounds) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var s sound.Sound
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		s, err = a.sRepo.GetTx(r.Context(), tx, id)
		return err
	})
	if errors.Is(err, sound.ErrNotFound) {
		writeError(w, http.StatusNotFound, "sound not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}
	writeJSON(w, http.StatusOK, soundToResponse(s))
}

func (a *tenantSounds) download(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var s sound.Sound
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		s, err = a.sRepo.GetTx(r.Context(), tx, id)
		return err
	})
	if errors.Is(err, sound.ErrNotFound) {
		writeError(w, http.StatusNotFound, "sound not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}
	f, err := a.storage.Open(s.TenantID, s.FileKey)
	if err != nil {
		slog.Error("sound storage open failed", "err", err, "id", id, "key", s.FileKey)
		writeError(w, http.StatusInternalServerError, "open failed")
		return
	}
	defer func() { _ = f.Close() }()
	w.Header().Set("Content-Type", s.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(s.SizeBytes, 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, sanitizeFilename(s.Name)+filepath.Ext(s.FileKey)))
	if _, err := io.Copy(w, f); err != nil {
		slog.Warn("sound stream interrupted", "err", err, "id", id)
	}
}

func sanitizeFilename(s string) string {
	// strip slashes + control chars; keep alnum/-/_/space/dot
	b := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b = append(b, r)
		case r == '-' || r == '_' || r == ' ' || r == '.':
			b = append(b, r)
		}
	}
	if len(b) == 0 {
		return "sound"
	}
	return string(b)
}

func (a *tenantSounds) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}
	var out sound.Sound
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		out, err = a.sRepo.UpdateTx(r.Context(), tx, id, sound.UpdatePatch{Name: req.Name, Description: req.Description})
		return err
	})
	if errors.Is(err, sound.ErrNotFound) {
		writeError(w, http.StatusNotFound, "sound not found")
		return
	}
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "sound name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, soundToResponse(out))
}

func (a *tenantSounds) delete(w http.ResponseWriter, r *http.Request) {
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
	var s sound.Sound
	err = db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: tid, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var err error
		s, err = a.sRepo.GetTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		if err := a.sRepo.DeleteTx(r.Context(), tx, id); err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{
			RequestID:  middleware.GetReqID(r.Context()),
			ActorType:  "user",
			ActorID:    strconv.FormatInt(claims.UserID, 10),
			TenantID:   &tid,
			EntityType: "sound",
			EntityID:   strconv.FormatInt(id, 10),
			Action:     "delete",
			Before:     map[string]any{"name": s.Name, "size_bytes": s.SizeBytes},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
	if errors.Is(err, sound.ErrNotFound) {
		writeError(w, http.StatusNotFound, "sound not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	if a.storage != nil {
		_ = a.storage.Delete(tid, s.FileKey)
	}
	w.WriteHeader(http.StatusNoContent)
}
