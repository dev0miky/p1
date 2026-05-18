package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"p1/engine/internal/auth"
	"p1/engine/internal/tenant"
)

type Config struct {
	Repo            *tenant.Repo
	Issuer          *auth.Issuer
	Logger          *slog.Logger
	AllowedOrigins  []string
}

func NewRouter(cfg Config) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(cfg.Logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	authH := auth.NewHandler(cfg.Repo, cfg.Issuer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "# placeholder")
	})

	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", authH.Login)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(cfg.Issuer))
			r.Get("/me", authH.Me)
		})
	})

	admin := &adminTenants{repo: cfg.Repo}
	r.Route("/admin/tenants", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireAction(auth.ActionManageTenants))
		r.Post("/", admin.create)
		r.Get("/", admin.list)
		r.Get("/{id}", admin.get)
		r.Patch("/{id}", admin.update)
	})

	return r
}
