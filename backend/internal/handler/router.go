package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/ats-proxy/proxy-manager/backend/internal/config"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
	"github.com/ats-proxy/proxy-manager/backend/internal/repository"
	"github.com/ats-proxy/proxy-manager/backend/internal/service"
)

func NewRouter(pool *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(RequestIDMiddleware)
	r.Use(CORSMiddleware)

	// Repos
	userRepo := repository.NewUserRepo(pool)
	sessionRepo := repository.NewSessionRepo(pool)
	configRepo := repository.NewConfigRepo(pool)
	domainRuleRepo := repository.NewDomainRuleRepo(pool)
	ipRangeRuleRepo := repository.NewIPRangeRuleRepo(pool)
	parentProxyRepo := repository.NewParentProxyRepo(pool)
	clientACLRepo := repository.NewClientACLRepo(pool)
	proxyRepo := repository.NewProxyRepo(pool)
	configProxyRepo := repository.NewConfigProxyRepo(pool)
	proxyStatsRepo := repository.NewProxyStatsRepo(pool)
	proxyLogsRepo := repository.NewProxyLogsRepo(pool)
	auditRepo := repository.NewAuditRepo(pool)

	// Services
	authSvc := service.NewAuthService(userRepo, sessionRepo, cfg.JWTSecret)
	userSvc := service.NewUserService(userRepo, auditRepo)
	configSvc := service.NewConfigService(pool, configRepo, domainRuleRepo, ipRangeRuleRepo, parentProxyRepo, clientACLRepo, configProxyRepo, auditRepo)
	syncSvc := service.NewSyncService(proxyRepo, configRepo, configProxyRepo, proxyStatsRepo, proxyLogsRepo, configSvc, rdb)
	proxySvc := service.NewProxyService(proxyRepo, proxyStatsRepo, proxyLogsRepo, configRepo, configProxyRepo, auditRepo)
	auditSvc := service.NewAuditService(auditRepo, userRepo)

	// Handlers
	authH := NewAuthHandler(authSvc)
	userH := NewUserHandler(userSvc)
	configH := NewConfigHandler(configSvc)
	syncH := NewSyncHandler(syncSvc)
	proxyH := NewProxyHandler(proxySvc)
	auditH := NewAuditHandler(auditSvc)

	r.Route("/api/v1", func(r chi.Router) {
		// Health
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":      "ok",
				"server_time": time.Now().UTC(),
			})
		})

		// Auth (public)
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/refresh", authH.Refresh)

		// Sync (no auth - called by Helper)
		r.Route("/sync", func(r chi.Router) {
			r.Post("/register", syncH.Register)
			r.Get("/", syncH.GetConfig)
			r.Post("/ack", syncH.Ack)
			r.Post("/stats", syncH.Stats)
			r.Post("/logs", syncH.Logs)
		})

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(cfg.JWTSecret))

			// Auth (authenticated)
			r.Post("/auth/beacon", authH.Beacon)
			r.Post("/auth/logout", authH.Logout)

			// Users
			r.Route("/users", func(r chi.Router) {
				r.Use(RequireRole(domain.RoleRoot, domain.RoleAdmin))
				r.Get("/", userH.List)
				r.Post("/", userH.Create)
				r.Put("/{id}", userH.Update)
				r.Delete("/{id}", userH.Delete)
			})

			// Configs
			r.Route("/configs", func(r chi.Router) {
				r.Get("/", configH.List)
				r.Post("/", configH.Create)
				r.Get("/{id}", configH.GetByID)
				r.Put("/{id}", configH.Update)
				r.With(RequireRole(domain.RoleRoot, domain.RoleAdmin)).Delete("/{id}", configH.Delete)
				r.Post("/{id}/submit", configH.Submit)
				r.Post("/{id}/approve", configH.Approve)
				r.Post("/{id}/reject", configH.Reject)
				r.Post("/{id}/clone", configH.Clone)
			})

			// Proxies
			r.Route("/proxies", func(r chi.Router) {
				r.Get("/", proxyH.List)
				r.Get("/{id}", proxyH.GetByID)
				r.Post("/{id}/logs", proxyH.StartLogCapture)
				r.Get("/{id}/logs", proxyH.GetLogs)
				r.With(RequireRole(domain.RoleRoot, domain.RoleAdmin)).Put("/{id}/config", proxyH.AssignConfig)
				r.With(RequireRole(domain.RoleRoot, domain.RoleAdmin)).Delete("/{id}", proxyH.Delete)
			})

			// Audit
			r.Route("/audit", func(r chi.Router) {
				r.Use(RequireRole(domain.RoleRoot, domain.RoleAdmin))
				r.Get("/", auditH.List)
			})
		})
	})

	return r
}
