package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"p1/engine/internal/auth"
	"p1/engine/internal/config"
	"p1/engine/internal/db"
	"p1/engine/internal/tenant"
	"p1/engine/migrations"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	logger := newLogger(cfg.LogLevel)

	switch cmd {
	case "serve":
		if err := serve(cfg, logger); err != nil {
			logger.Error("serve failed", "err", err)
			os.Exit(1)
		}
	case "migrate":
		if err := runMigrate(cfg, logger); err != nil {
			logger.Error("migrate failed", "err", err)
			os.Exit(1)
		}
	case "seed":
		if err := runSeed(cfg, logger); err != nil {
			logger.Error("seed failed", "err", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		fmt.Fprintln(os.Stderr, "usage: api [serve|migrate|seed]")
		os.Exit(2)
	}
}

func serve(cfg config.Config, logger *slog.Logger) error {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.AppDatabaseURL)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()

	repo := tenant.NewRepo(pool)
	iss := auth.NewIssuer(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	h := auth.NewHandler(repo, iss)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "# placeholder")
	})
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.Handle("GET /auth/me", auth.RequireAuth(iss)(http.HandlerFunc(h.Me)))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           withLogging(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	logger.Info("api listening", "port", cfg.Port)
	return srv.ListenAndServe()
}

func runMigrate(cfg config.Config, logger *slog.Logger) error {
	sqlDB, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := db.Migrate(sqlDB, migrations.FS, "."); err != nil {
		return err
	}
	logger.Info("migrations applied")
	return nil
}

func runSeed(cfg config.Config, logger *slog.Logger) error {
	if cfg.SuperAdminEmail == "" || cfg.SuperAdminPasswd == "" {
		return errors.New("SUPER_ADMIN_EMAIL and SUPER_ADMIN_PASSWORD must be set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.AppDatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	repo := tenant.NewRepo(pool)
	exists, err := repo.AnySuperAdminExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		logger.Info("super_admin already exists, skipping seed")
		return nil
	}
	hash, err := auth.Hash(cfg.SuperAdminPasswd)
	if err != nil {
		return err
	}
	u, err := repo.CreateUserAsSuperAdmin(ctx, tenant.User{
		TenantID: nil, Email: cfg.SuperAdminEmail, Role: "super_admin", PasswordHash: hash,
	})
	if err != nil {
		return err
	}
	logger.Info("super_admin created", "user_id", u.ID, "email", u.Email)
	return nil
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		logger.Info("req",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"dur_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
