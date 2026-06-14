package admin

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"

	"github.com/acme-corp/mcp-runtime/pkg/audit"
	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/pkg/config"
	"github.com/acme-corp/mcp-runtime/pkg/secrets"
	"github.com/acme-corp/mcp-runtime/pkg/telemetry"
)

// API is the control-plane HTTP server.
type API struct {
	e       *echo.Echo
	cfg     *config.Config
	log     zerolog.Logger
	metrics *telemetry.Metrics
	tracing *telemetry.Tracing
}

// NewAPI wires the admin routes, guarded by admin-role authorization.
func NewAPI(cfg *config.Config, log zerolog.Logger, store Store, sink Sink, sec secrets.Store, auditLog audit.Logger, validator authz.OrgValidator) *API {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())

	// CORS for the admin console SPA (T008). Only the configured origins may call
	// the API from a browser; auth is still enforced per request (bearer + admin
	// role + org audience). No authorization change — headers only.
	if origins := splitCSV(cfg.ConsoleOrigins); len(origins) > 0 {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: origins,
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions},
			AllowHeaders: []string{echo.HeaderAuthorization, echo.HeaderContentType},
		}))
	}

	var tr *telemetry.Tracing
	if t, err := telemetry.NewTracing(context.Background(), "mcp-control-plane", cfg.OTLPEndpoint); err != nil {
		log.Error().Err(err).Msg("tracing init failed; continuing without tracing")
	} else {
		tr = t
		e.Use(tracingMiddleware(tr.Tracer))
	}

	var m *telemetry.Metrics
	if mm, err := telemetry.NewMetrics("mcp-control-plane"); err != nil {
		log.Error().Err(err).Msg("metrics init failed; continuing without metrics")
	} else {
		m = mm
		e.Use(metricsMiddleware(m))
	}

	h := NewHandlers(store, sink, sec, auditLog)
	e.GET("/healthz", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
	if m != nil {
		e.GET("/metrics", echo.WrapHandler(m.Handler()))
	}

	g := e.Group("/v1/orgs/:org/servers")
	g.Use(requireAdmin(validator, cfg.AdminAudience))
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.PATCH("/:id", h.Patch)
	g.DELETE("/:id", h.Delete)
	// Write-only credential endpoints (values never echoed).
	g.PUT("/:id/credentials", h.PutOrgCredentials)
	g.DELETE("/:id/credentials", h.DeleteOrgCredentials)
	g.PUT("/:id/credentials/me", h.PutMyCredentials)
	g.DELETE("/:id/credentials/me", h.DeleteMyCredentials)

	// Audit query (org-scoped, admin-only).
	ag := e.Group("/v1/orgs/:org/audit")
	ag.Use(requireAdmin(validator, cfg.AdminAudience))
	ag.GET("", h.ListAudit)

	// Read-only quotas (admin-only) — surfaces the configured limits for the admin
	// console (T047). Display only; editing is out of scope (FR-017/FR-023).
	qg := e.Group("/v1/orgs/:org/quotas")
	qg.Use(requireAdmin(validator, cfg.AdminAudience))
	qg.GET("", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]int{
			"org_per_min":  cfg.RateOrgPerMin,
			"user_per_min": cfg.RateUserPerMin,
		})
	})

	return &API{e: e, cfg: cfg, log: log, metrics: m, tracing: tr}
}

// Start runs the server until ctx is cancelled, then shuts down gracefully.
func (a *API) Start(ctx context.Context) error {
	go func() {
		if err := a.e.Start(a.cfg.HTTPAddr); err != nil && err != http.ErrServerClosed {
			a.log.Error().Err(err).Msg("control-plane stopped unexpectedly")
		}
	}()
	a.log.Info().Str("addr", a.cfg.HTTPAddr).Msg("control-plane listening")

	<-ctx.Done()
	a.log.Info().Msg("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()
	err := a.e.Shutdown(shutdownCtx)
	if a.tracing != nil {
		_ = a.tracing.Shutdown(shutdownCtx) // flush pending spans
	}
	return err
}

// Mount lets callers register additional routes on the control-plane's Echo
// instance before Start — e.g. the 003 tenant-provisioning platform API. The
// global middleware (recover/CORS/tracing/metrics) registered in NewAPI also
// applies to routes added here.
func (a *API) Mount(register func(e *echo.Echo)) { register(a.e) }

// requireAdmin validates the bearer token for the path org against the admin
// audience and requires the "admin" role (HC-1: org from path, per-org realm).
func requireAdmin(v authz.OrgValidator, audience string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tok := bearer(c.Request().Header.Get("Authorization"))
			if tok == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
			}
			p, err := v.ValidateForOrg(c.Request().Context(), tok, c.Param("org"), audience)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}
			if !p.HasRole("admin") {
				return echo.NewHTTPError(http.StatusForbidden, "admin role required")
			}
			c.Set("principal", p)
			return next(c)
		}
	}
}

func bearer(h string) string {
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// splitCSV splits a comma-separated list, trimming blanks (e.g. CORS origins).
func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
