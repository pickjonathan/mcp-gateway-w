// Package server wires the gateway HTTP surface (health, OAuth discovery, and
// the aggregated MCP endpoint) onto an Echo server, and consumes control-plane
// server-change events into a per-org downstream catalog.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/acme-corp/mcp-runtime/pkg/audit"
	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/pkg/config"
	"github.com/acme-corp/mcp-runtime/pkg/secrets"
	"github.com/acme-corp/mcp-runtime/pkg/serverevents"
	"github.com/acme-corp/mcp-runtime/pkg/telemetry"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/auth"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/downstream"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/mcp"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/quota"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/remotehttp"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/sandbox"
)

// Server is the gateway HTTP server.
type Server struct {
	e           *echo.Echo
	cfg         *config.Config
	log         zerolog.Logger
	catalog     *downstream.Catalog
	handler     *mcp.Handler
	runtime     sandbox.Runtime
	secrets     secrets.Store
	audit       audit.Logger
	metrics     *telemetry.Metrics
	tracing     *telemetry.Tracing
	redis       *redis.Client        // nil unless a Redis-backed quota limiter is in use
	blockEgress bool                 // refuse remote calls to non-public IPs (SSRF protection)
	auditDeny   *quota.WindowLimiter // bounds denial-audit writes (anti-amplification)
}

// New builds the gateway server with middleware and routes wired.
func New(cfg *config.Config, log zerolog.Logger) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	cat := downstream.NewCatalog()
	var sec secrets.Store = secrets.NewMemStore()
	if cfg.VaultAddr != "" {
		sec = secrets.NewVaultStore(cfg.VaultAddr, cfg.VaultToken)
	}
	enforcer, rdb := buildEnforcer(cfg)
	s := &Server{
		e: e, cfg: cfg, log: log,
		catalog:     cat,
		runtime:     sandbox.Select(cfg.SandboxRuntime, cfg.SandboxImage, cfg.SandboxEgressNetwork, log),
		secrets:     sec,
		audit:       buildAuditLogger(cfg, log),
		redis:       rdb,
		blockEgress: cfg.BlockPrivateEgress,
		auditDeny:   quota.NewWindowLimiter(cfg.AuditDenyPerMin, time.Minute),
	}
	// Handler is built after s so it can carry s's audit deny-recorder.
	s.handler = mcp.NewHandler(cat, mcp.WithQuota(enforcer), mcp.WithDenyRecorder(s.recordAuthzDenied))
	if m, err := telemetry.NewMetrics("mcp-gateway"); err != nil {
		log.Error().Err(err).Msg("metrics init failed; continuing without metrics")
	} else {
		s.metrics = m
	}
	if t, err := telemetry.NewTracing(context.Background(), "mcp-gateway", cfg.OTLPEndpoint); err != nil {
		log.Error().Err(err).Msg("tracing init failed; continuing without tracing")
	} else {
		s.tracing = t
	}

	// Middleware must be registered before routes. Order (outer→inner): recover,
	// tracing (wraps the whole request), logging, metrics (sees final status).
	e.Use(middleware.Recover())
	if s.tracing != nil {
		e.Use(tracingMiddleware(s.tracing.Tracer))
	}
	e.Use(requestLogger(log))
	if s.metrics != nil {
		e.Use(s.metricsMiddleware())
	}
	s.routes()
	return s
}

// buildEnforcer selects the quota backend: a Redis-backed limiter (shared across
// gateway replicas) when a Redis address is configured, else an in-process
// limiter. Returns the redis client (if any) so it can be closed on shutdown.
func buildEnforcer(cfg *config.Config) (*quota.Enforcer, *redis.Client) {
	if cfg.RedisAddr == "" {
		return quota.NewEnforcer(cfg.RateOrgPerMin, cfg.RateUserPerMin, time.Minute), nil
	}
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	return quota.NewRedisEnforcer(rdb, cfg.RateOrgPerMin, cfg.RateUserPerMin, time.Minute), rdb
}

func (s *Server) routes() {
	s.e.GET("/healthz", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
	s.e.GET("/readyz", func(c echo.Context) error { return c.String(http.StatusOK, "ready") })

	// Prometheus scrape endpoint (FR-008). Unauthenticated by design — protect it
	// at the network layer (scrape from inside the mesh), never expose publicly.
	if s.metrics != nil {
		s.e.GET("/metrics", echo.WrapHandler(s.metrics.Handler()))
	}

	// RFC 9728 discovery for the per-org MCP resource (FR-024).
	s.e.GET("/.well-known/oauth-protected-resource", auth.ProtectedResourceMetadataHandler(s.cfg))

	validator := authz.NewJWTValidator(s.cfg.BaseDomain, s.cfg.KeycloakIssuerTemplate, s.cfg.ResourceTemplate, authz.NewJWKSKeySource())
	g := s.e.Group("/mcp")
	g.Use(auth.RequireAuth(s.cfg, validator, s.recordAuthDenied))
	g.POST("", s.handleMCP)
	g.POST("/", s.handleMCP)
}

func (s *Server) handleMCP(c echo.Context) error {
	var req mcp.Request
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &mcp.Response{
			JSONRPC: "2.0",
			Error:   &mcp.Error{Code: mcp.CodeParse, Message: "parse error"},
		})
	}
	p, _ := c.Get("principal").(*authz.Principal)
	resp := s.handler.Dispatch(c.Request().Context(), p, &req)
	s.observeToolCall(c.Request().Context(), p, &req, resp)
	if resp == nil { // notification — no reply expected
		return c.NoContent(http.StatusAccepted)
	}
	return c.JSON(http.StatusOK, resp)
}

// SubscribeServers consumes control-plane server-change events and updates the
// per-org catalog (building downstream clients). Runs until ctx is cancelled.
func (s *Server) SubscribeServers(ctx context.Context, bus serverevents.Bus) {
	go func() {
		if err := bus.Subscribe(ctx, s.applyServerEvent); err != nil && ctx.Err() == nil {
			s.log.Error().Err(err).Msg("server-event subscription ended")
		}
	}()
}

func (s *Server) applyServerEvent(e serverevents.Event) {
	switch e.Action {
	case serverevents.ActionUpsert:
		// per_user mode needs a distinct instance per calling user (each carries
		// that user's credentials), so register a provider rather than one shared
		// client (US6).
		if e.CredentialMode == "per_user" {
			switch e.Type {
			case "remote_http", "stdio":
				s.catalog.AddProvider(e.OrgID, e.Slug, s.perUserProvider(e), e.AllowedRoles)
				s.log.Info().Str("org", e.OrgID).Str("slug", e.Slug).Str("type", e.Type).Msg("registered per-user server")
			default:
				s.log.Warn().Str("type", e.Type).Str("slug", e.Slug).Msg("server type not yet supported by data plane")
			}
			return
		}
		switch e.Type {
		case "remote_http":
			opts := []remotehttp.Option{remotehttp.WithBlockPrivate(s.blockEgress)}
			if creds := s.resolveCredentials(e); len(creds) > 0 {
				opts = append(opts, remotehttp.WithHeader(credHeaders(creds)))
			}
			s.catalog.AddScoped(e.OrgID, e.Slug, remotehttp.New(e.EndpointURL, opts...), e.AllowedRoles)
			s.log.Info().Str("org", e.OrgID).Str("slug", e.Slug).Msg("registered remote server")
		case "stdio":
			env := make([]string, 0, len(e.Env))
			for k, v := range e.Env {
				env = append(env, k+"="+v)
			}
			env = append(env, kvEnv(s.resolveCredentials(e))...)
			s.catalog.AddScoped(e.OrgID, e.Slug, sandbox.NewServer(s.runtime, sandbox.Spec{Command: e.Command, Args: e.Args, Env: env}), e.AllowedRoles)
			s.log.Info().Str("org", e.OrgID).Str("slug", e.Slug).Msg("registered stdio server")
		default:
			s.log.Warn().Str("type", e.Type).Str("slug", e.Slug).Msg("server type not yet supported by data plane")
		}
	case serverevents.ActionCredentialChanged:
		// A per-user secret was rotated: drop that user's cached instance so the
		// next request rebuilds it with the new secret (T079). Org-level rotation
		// arrives as an upsert instead, which rebuilds the shared instance.
		s.catalog.Invalidate(e.OrgID, e.Slug, e.UserID)
		s.log.Info().Str("org", e.OrgID).Str("slug", e.Slug).Str("user", e.UserID).Msg("invalidated cached credential instance")
	case serverevents.ActionRemove:
		s.catalog.Remove(e.OrgID, e.Slug)
		s.log.Info().Str("org", e.OrgID).Str("slug", e.Slug).Msg("removed server")
	}
}

// Start runs the server until ctx is cancelled, then shuts down gracefully.
func (s *Server) Start(ctx context.Context) error {
	go func() {
		if err := s.e.Start(s.cfg.HTTPAddr); err != nil && err != http.ErrServerClosed {
			s.log.Error().Err(err).Msg("server stopped unexpectedly")
		}
	}()
	s.log.Info().Str("addr", s.cfg.HTTPAddr).Msg("gateway listening")

	<-ctx.Done()
	s.log.Info().Msg("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()
	err := s.e.Shutdown(shutdownCtx)
	if s.tracing != nil {
		_ = s.tracing.Shutdown(shutdownCtx) // flush pending spans
	}
	if s.redis != nil {
		_ = s.redis.Close()
	}
	return err
}

func requestLogger(log zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			req := c.Request()
			status := c.Response().Status
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
			}
			ev := log.Info()
			if err != nil {
				ev = log.Error().Err(err)
			}
			ev.Str("method", req.Method).
				Str("path", req.URL.Path).
				Int("status", status).
				Str("host", req.Host).
				Msg("request")
			return err
		}
	}
}
