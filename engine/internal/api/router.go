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

	adminT := &adminTenants{repo: cfg.Repo}
	r.Route("/admin/tenants", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireAction(auth.ActionManageTenants))
		r.Post("/", adminT.create)
		r.Get("/", adminT.list)
		r.Get("/{id}", adminT.get)
		r.Patch("/{id}", adminT.update)
	})

	adminU := &adminUsers{repo: cfg.Repo}
	r.Route("/admin/users", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireAction(auth.ActionManageTenants))
		r.Post("/", adminU.create)
		r.Get("/", adminU.list)
		r.Get("/{id}", adminU.get)
		r.Patch("/{id}", adminU.update)
	})

	tenU := &tenantUsers{repo: cfg.Repo}
	r.Route("/tenant/users", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireTenant)
		r.Use(auth.RequireAction(auth.ActionManageUsers))
		r.Post("/", tenU.create)
		r.Get("/", tenU.list)
		r.Get("/{id}", tenU.get)
		r.Patch("/{id}", tenU.update)
	})

	return r
}
