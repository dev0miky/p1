package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"p1/engine/internal/auth"
	"p1/engine/internal/tenant"
)

type tenantRealtime struct {
	repo *tenant.Repo
}

const (
	notifyChannel  = "tenant_events"
	sseHeartbeat   = 30 * time.Second
	sseWaitTimeout = 25 * time.Second // < heartbeat so the keepalive comment can fire reliably
)

func (a *tenantRealtime) events(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	tid := claims.TenantID
	if tid <= 0 && claims.Role != "super_admin" {
		writeError(w, http.StatusBadRequest, "no tenant context")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // tell nginx/caddy not to buffer
	w.WriteHeader(http.StatusOK)

	// First frame: tell the client we're connected. Useful for the UI to
	// show a "live" dot immediately rather than waiting for the first event.
	if _, err := fmt.Fprintf(w, "event: hello\ndata: {\"ok\":true}\n\n"); err != nil {
		return
	}
	flusher.Flush()

	conn, err := a.repo.Pool().Acquire(r.Context())
	if err != nil {
		slog.Error("sse acquire conn", "err", err, "tenant", tid)
		_, _ = fmt.Fprintf(w, "event: error\ndata: {\"error\":\"acquire failed\"}\n\n")
		flusher.Flush()
		return
	}
	defer conn.Release()

	pgconn := conn.Conn()
	if _, err := pgconn.Exec(r.Context(), "LISTEN "+notifyChannel); err != nil {
		slog.Error("sse listen", "err", err, "tenant", tid)
		return
	}
	defer func() {
		// Best-effort unlisten with a fresh context — request ctx may be cancelled.
		uctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _ = pgconn.Exec(uctx, "UNLISTEN "+notifyChannel)
		cancel()
	}()

	ctx := r.Context()
	heartbeat := time.NewTicker(sseHeartbeat)
	defer heartbeat.Stop()

	for {
		// Wait up to sseWaitTimeout for a notification. If none arrives we
		// loop and emit a comment line as keepalive.
		wctx, cancel := context.WithTimeout(ctx, sseWaitTimeout)
		notif, err := pgconn.WaitForNotification(wctx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return // client disconnected
			}
			if isDeadlineExceeded(err) {
				if _, werr := fmt.Fprintf(w, ": keepalive %d\n\n", time.Now().Unix()); werr != nil {
					return
				}
				flusher.Flush()
				continue
			}
			slog.Warn("sse wait err", "err", err, "tenant", tid)
			continue
		}
		if !shouldDeliver(notif.Payload, tid, claims.Role) {
			continue
		}
		eventType := extractType(notif.Payload)
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, notif.Payload); err != nil {
			return
		}
		flusher.Flush()
	}
}

func shouldDeliver(payload string, tenantID int64, role string) bool {
	if role == "super_admin" {
		return true
	}
	var p struct {
		TenantID int64 `json:"tenant_id"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return false
	}
	return p.TenantID == tenantID
}

func extractType(payload string) string {
	var p struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "message"
	}
	if p.Type == "" {
		return "message"
	}
	return p.Type
}

func isDeadlineExceeded(err error) bool {
	if err == nil {
		return false
	}
	return err == context.DeadlineExceeded || err.Error() == "context deadline exceeded"
}

// keep pgxpool import even if not directly named — needed transitively for type assertions.
var _ = (*pgxpool.Pool)(nil)
var _ = (*pgx.Conn)(nil)
