package brokering

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/pkg/audit"
	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/pkg/secrets"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/idp"
)

// OrgValidator validates an org-scoped admin token (satisfied by *authz.JWTValidator).
type OrgValidator interface {
	ValidateForOrg(ctx context.Context, raw, org, audience string) (*authz.Principal, error)
}

// Handlers serve org-scoped brokering configuration (US4). The IdP client secret
// is written to the secret store; only a ref is persisted and it is never echoed.
type Handlers struct {
	store   Store
	broker  idp.Broker
	secrets secrets.Store
	audit   audit.Logger
	now     func() time.Time
}

// NewHandlers builds the brokering handlers. broker may be nil (provisioning off).
func NewHandlers(store Store, broker idp.Broker, sec secrets.Store, auditLog audit.Logger) *Handlers {
	return &Handlers{store: store, broker: broker, secrets: sec, audit: auditLog, now: time.Now}
}

// RegisterRoutes mounts org-scoped brokering routes (admin-guarded).
func RegisterRoutes(e *echo.Echo, h *Handlers, v OrgValidator, adminAudience string) {
	g := e.Group("/v1/orgs/:org/identity-providers")
	g.Use(requireOrgAdmin(v, adminAudience))
	g.PUT("/:alias", h.Put)
	g.GET("", h.List)
	g.DELETE("/:alias", h.Delete)
}

type putReq struct {
	Type         string            `json:"type"`
	Config       map[string]string `json:"config"`
	Secret       string            `json:"secret"`
	RoleMappings map[string]string `json:"role_mappings"`
}

// Put configures (creates or updates) a brokered IdP for the org (FR-016).
func (h *Handlers) Put(c echo.Context) error {
	org, alias := c.Param("org"), c.Param("alias")
	if h.broker == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "provisioning not configured")
	}
	var req putReq
	if err := c.Bind(&req); err != nil || (req.Type != "oidc" && req.Type != "saml") {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "type must be oidc or saml")
	}
	ctx := c.Request().Context()

	secretRef := "idp/" + org + "/" + alias
	if req.Secret != "" && h.secrets != nil {
		if err := h.secrets.Put(ctx, secretRef, map[string]string{"clientSecret": req.Secret}); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "store secret failed")
		}
	}
	// Apply to Keycloak with the secret merged into the provider config.
	cfg := map[string]string{}
	for k, v := range req.Config {
		cfg[k] = v
	}
	if req.Secret != "" {
		cfg["clientSecret"] = req.Secret
	}
	if err := h.broker.UpsertIdentityProvider(ctx, org, idp.IdentityProvider{Alias: alias, ProviderID: req.Type, Enabled: true, Config: cfg}); err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "apply identity provider failed")
	}

	now := h.now()
	l := Link{ID: newID(), OrgID: org, Alias: alias, Type: req.Type, Config: req.Config, SecretRef: secretRef, RoleMappings: req.RoleMappings, Enabled: true, CreatedAt: now, UpdatedAt: now}
	if existing, err := h.store.Get(ctx, org, alias); err == nil {
		l.ID, l.CreatedAt = existing.ID, existing.CreatedAt
	}
	if err := h.store.Upsert(ctx, l); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "store link failed")
	}
	h.record(ctx, org, actor(c), "brokering.configure", alias)
	return c.JSON(http.StatusOK, l) // SecretRef is json:"-"; the secret is never returned
}

// List returns the org's brokered IdPs (non-secret config only).
func (h *Handlers) List(c echo.Context) error {
	out, err := h.store.List(c.Request().Context(), c.Param("org"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "list failed")
	}
	return c.JSON(http.StatusOK, map[string]any{"identity_providers": out})
}

// Delete removes a brokered IdP (config + Keycloak instance + secret).
func (h *Handlers) Delete(c echo.Context) error {
	org, alias := c.Param("org"), c.Param("alias")
	ctx := c.Request().Context()
	if _, err := h.store.Get(ctx, org, alias); errors.Is(err, ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "identity provider not found")
	}
	if h.broker != nil {
		_ = h.broker.DeleteIdentityProvider(ctx, org, alias)
	}
	if h.secrets != nil {
		_ = h.secrets.Delete(ctx, "idp/"+org+"/"+alias)
	}
	_ = h.store.Delete(ctx, org, alias)
	h.record(ctx, org, actor(c), "brokering.delete", alias)
	return c.NoContent(http.StatusNoContent)
}

func (h *Handlers) record(ctx context.Context, org, actorID, action, target string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.Record(ctx, audit.Event{Time: h.now(), OrgID: org, Actor: actorID, Action: action, Target: target})
}

func requireOrgAdmin(v OrgValidator, audience string) echo.MiddlewareFunc {
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

func actor(c echo.Context) string {
	if p, ok := c.Get("principal").(*authz.Principal); ok && p != nil {
		return p.UserID
	}
	return ""
}

func bearer(h string) string {
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "idl_" + hex.EncodeToString(b)
}
