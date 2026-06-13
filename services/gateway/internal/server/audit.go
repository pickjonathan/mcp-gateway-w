package server

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/acme-corp/mcp-runtime/pkg/audit"
	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/pkg/config"
)

// buildAuditLogger selects the gateway's audit backend: a durable, Object-Lock'd
// S3 archive when configured, else in-memory. The gateway uses its own stream
// (a "-gateway" bucket suffix) so its single-writer hash chain never collides
// with the control-plane's.
func buildAuditLogger(cfg *config.Config, log zerolog.Logger) audit.Logger {
	if cfg.AuditS3Endpoint == "" {
		return audit.NewMemLogger()
	}
	arch, err := audit.NewS3Archive(context.Background(), cfg.AuditS3Endpoint, cfg.AuditS3AccessKey,
		cfg.AuditS3SecretKey, cfg.AuditS3Bucket+"-gateway", cfg.AuditS3UseSSL, cfg.AuditRetention)
	if err != nil {
		log.Error().Err(err).Msg("gateway audit archive init failed; using in-memory")
		return audit.NewMemLogger()
	}
	return arch
}

// recordAuthDenied logs an authentication failure (no/invalid token) as a
// tamper-evident audit event (FR-010). The actor is unknown — the token was not
// trusted — so only the host-derived org is recorded; no token is logged.
func (s *Server) recordAuthDenied(c echo.Context, reason string) {
	if !s.auditDenyAllowed("auth.denied") {
		return
	}
	org := authz.OrgFromHost(c.Request().Host, s.cfg.BaseDomain)
	_ = s.audit.Record(c.Request().Context(), audit.Event{
		OrgID:    org,
		Actor:    "unknown",
		Action:   "auth.denied",
		Target:   c.Request().URL.Path,
		Metadata: map[string]string{"reason": reason, "host": c.Request().Host},
	})
}

// recordAuthzDenied logs an authorization (RBAC) denial — a user attempting a
// server they may not use, the cross-tenant/over-privilege signal.
func (s *Server) recordAuthzDenied(ctx context.Context, org, user, target, reason string) {
	if !s.auditDenyAllowed("authz.denied") {
		return
	}
	_ = s.audit.Record(ctx, audit.Event{
		OrgID:    org,
		Actor:    user,
		Action:   "authz.denied",
		Target:   target,
		Metadata: map[string]string{"reason": reason},
	})
}

// auditDenyAllowed rate-limits denial-audit writes so an unauthenticated flood
// can't amplify audit storage. The limiter is global (one bucket) to stay bounded
// regardless of how many distinct hosts an attacker forges. Drops are counted in
// mcp_audit_dropped_total so suppression is observable, never silent — the
// mcp_requests_total{code="401"} metric still captures the full denial rate.
func (s *Server) auditDenyAllowed(action string) bool {
	if s.auditDeny == nil || s.auditDeny.Allow("deny") {
		return true
	}
	if s.metrics != nil {
		s.metrics.AuditDropped.Add(context.Background(), 1, metric.WithAttributes(attribute.String("action", action)))
	}
	return false
}
