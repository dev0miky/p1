package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"p1/engine/internal/auth"
	"p1/engine/internal/campaign"
	"p1/engine/internal/dnc"
	"p1/engine/internal/fsm"
	"p1/engine/internal/lead"
	"p1/engine/internal/leadimport"
	"p1/engine/internal/script"
	"p1/engine/internal/sound"
	"p1/engine/internal/tenant"
)

type Config struct {
	Repo           *tenant.Repo
	Issuer         *auth.Issuer
	Logger         *slog.Logger
	AllowedOrigins []string
	SoundStorage   *sound.Storage
	ImportStorage  *leadimport.Storage
	ImportRunner   *leadimport.Runner
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

	cRepo := campaign.NewRepo()
	tenC := &tenantCampaigns{repo: cfg.Repo, cRepo: cRepo}
	r.Route("/tenant/campaigns", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireTenant)
		r.Use(auth.RequireAction(auth.ActionManageCampaigns))
		r.Post("/", tenC.create)
		r.Get("/", tenC.list)
		r.Get("/{id}", tenC.get)
		r.Get("/{id}/stats", tenC.stats)
		r.Get("/{id}/leads", tenC.leads)
		r.Get("/{id}/calls", tenC.calls)
		r.Get("/{id}/resources", tenC.listResources)
		r.Post("/{id}/resources/sounds", tenC.attachSound)
		r.Delete("/{id}/resources/sounds/{sound_id}", tenC.detachSound)
		r.Post("/{id}/resources/scripts", tenC.attachScript)
		r.Delete("/{id}/resources/scripts/{script_id}", tenC.detachScript)
		r.Post("/{id}/resources/lists", tenC.attachList)
		r.Delete("/{id}/resources/lists/{list_id}", tenC.detachList)
		r.Patch("/{id}", tenC.update)
	})

	lRepo := lead.NewRepo()
	tenL := &tenantLeads{repo: cfg.Repo, lRepo: lRepo}
	tenLBulk := &tenantLeadsBulk{tenantLeads: tenL, dRepo: dnc.NewRepo()}
	r.Route("/tenant/leads", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireTenant)
		r.Use(auth.RequireAction(auth.ActionManageLeads))
		r.Post("/", tenL.create)
		r.Get("/", tenL.list)
		r.Get("/{id}", tenL.get)
		r.Get("/{id}/activity", tenL.activity)
		r.Patch("/{id}", tenL.update)
		r.Post("/{id}/redial", tenL.redial)
		r.Delete("/{id}", tenL.delete)
		r.Post("/bulk/delete", tenLBulk.delete)
		r.Post("/bulk/attach", tenLBulk.attach)
		r.Post("/bulk/dnc", tenLBulk.markDNC)
	})

	fsmRepo := fsm.NewRepo()
	tenCalls := &tenantCalls{repo: cfg.Repo, fsm: fsmRepo}
	r.Route("/tenant/calls", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireTenant)
		r.Use(auth.RequireAction(auth.ActionViewReports))
		r.Get("/recent", tenCalls.recent)
		r.Get("/stats", tenCalls.stats)
	})

	dRepo := dnc.NewRepo()
	tenD := &tenantDNC{repo: cfg.Repo, dRepo: dRepo}
	r.Route("/tenant/dnc", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireTenant)
		r.Use(auth.RequireAction(auth.ActionManageDNC))
		r.Post("/", tenD.add)
		r.Get("/", tenD.list)
		r.Delete("/{phone}", tenD.remove)
		r.Get("/check", tenD.check)
	})

	tenLL := &tenantLeadLists{repo: cfg.Repo, lRepo: lRepo}
	tenIm := &tenantImports{
		repo:    cfg.Repo,
		lRepo:   lRepo,
		iRepo:   leadimport.NewRepo(),
		storage: cfg.ImportStorage,
		runner:  cfg.ImportRunner,
	}
	r.Route("/tenant/lists", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireTenant)
		r.Use(auth.RequireAction(auth.ActionManageLeads))
		r.Post("/", tenLL.create)
		r.Get("/", tenLL.list)
		r.Get("/{id}", tenLL.get)
		r.Patch("/{id}", tenLL.update)
		r.Delete("/{id}", tenLL.delete)
		r.Post("/{id}/import", tenIm.upload)
	})

	r.Route("/tenant/lead-import-jobs", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireTenant)
		r.Use(auth.RequireAction(auth.ActionManageLeads))
		r.Get("/", tenIm.list)
		r.Get("/{id}", tenIm.get)
		r.Post("/{id}/abort", tenIm.abort)
	})

	tenRT := &tenantRealtime{repo: cfg.Repo}
	r.Route("/tenant/events", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireTenant)
		r.Get("/", tenRT.events)
	})

	tenScripts := &tenantScripts{repo: cfg.Repo, sRepo: script.NewRepo()}
	r.Route("/tenant/scripts", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireTenant)
		r.Use(auth.RequireAction(auth.ActionManageCampaigns))
		r.Post("/", tenScripts.create)
		r.Get("/", tenScripts.list)
		r.Get("/{id}", tenScripts.get)
		r.Patch("/{id}", tenScripts.update)
		r.Delete("/{id}", tenScripts.delete)
	})

	tenSounds := &tenantSounds{repo: cfg.Repo, sRepo: sound.NewRepo(), storage: cfg.SoundStorage}
	r.Route("/tenant/sounds", func(r chi.Router) {
		r.Use(auth.RequireAuth(cfg.Issuer))
		r.Use(auth.RequireTenant)
		r.Use(auth.RequireAction(auth.ActionManageCampaigns))
		r.Post("/", tenSounds.create)
		r.Get("/", tenSounds.list)
		r.Get("/{id}", tenSounds.get)
		r.Get("/{id}/download", tenSounds.download)
		r.Patch("/{id}", tenSounds.update)
		r.Delete("/{id}", tenSounds.delete)
	})

	return r
}
