package invites

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/acme-corp/mcp-runtime/pkg/audit"
	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/idp"
)

// DefaultTTL is how long an invitation stays valid.
const DefaultTTL = 7 * 24 * time.Hour

// OrgValidator validates an org-scoped admin token (satisfied by *authz.JWTValidator).
type OrgValidator interface {
	ValidateForOrg(ctx context.Context, raw, org, audience string) (*authz.Principal, error)
}

// Notifier delivers the invitation link. The dev implementation logs it.
type Notifier func(email, rawToken string)

// Handlers serve org-scoped invitation management plus the public accept endpoint.
type Handlers struct {
	store  Store
	kc     idp.Keycloak
	audit  audit.Logger
	notify Notifier
	now    func() time.Time
}

// NewHandlers builds the invitation handlers. notify may be nil (a logging stub is used).
func NewHandlers(store Store, kc idp.Keycloak, auditLog audit.Logger, log zerolog.Logger, notify Notifier) *Handlers {
	if notify == nil {
		notify = func(email, raw string) {
			log.Info().Str("email", email).Msg("invitation created (dev: deliver this token out-of-band)")
		}
	}
	return &Handlers{store: store, kc: kc, audit: auditLog, notify: notify, now: time.Now}
}

// RegisterRoutes mounts org-scoped invitation routes (admin-guarded) and the
// public accept route on e.
func RegisterRoutes(e *echo.Echo, h *Handlers, v OrgValidator, adminAudience string) {
	g := e.Group("/v1/orgs/:org/invitations")
	g.Use(requireOrgAdmin(v, adminAudience))
	g.POST("", h.Create)
	g.GET("", h.List)
	g.DELETE("/:id", h.Revoke)

	// Public: the invite token authenticates the request (no org token).
	e.POST("/v1/invitations:accept", h.Accept)
}

type createReq struct {
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// Create issues an invitation (US2/FR-013). The raw token is delivered to the
// invitee out-of-band and is NEVER returned in the response.
func (h *Handlers) Create(c echo.Context) error {
	org := c.Param("org")
	var req createReq
	if err := c.Bind(&req); err != nil || !strings.Contains(req.Email, "@") {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "valid email required")
	}
	raw, hash := newToken(org)
	now := h.now()
	inv := Invitation{
		ID: newID(), OrgID: org, Email: req.Email, Roles: req.Roles, TokenHash: hash,
		Status: StatusPending, ExpiresAt: now.Add(DefaultTTL), CreatedBy: actor(c), CreatedAt: now,
	}
	if err := h.store.Create(c.Request().Context(), inv); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "create invitation failed")
	}
	h.notify(req.Email, raw)
	h.record(c.Request().Context(), org, actor(c), "invitation.create", req.Email)
	return c.JSON(http.StatusCreated, inv) // TokenHash is json:"-"; raw token omitted
}

// List returns the org's invitations.
func (h *Handlers) List(c echo.Context) error {
	out, err := h.store.List(c.Request().Context(), c.Param("org"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "list failed")
	}
	return c.JSON(http.StatusOK, map[string]any{"invitations": out})
}

// Revoke cancels a pending invitation.
func (h *Handlers) Revoke(c echo.Context) error {
	org := c.Param("org")
	inv, err := h.store.Get(c.Request().Context(), org, c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "invitation not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "get failed")
	}
	inv.Status = StatusRevoked
	_ = h.store.Update(c.Request().Context(), inv)
	h.record(c.Request().Context(), org, actor(c), "invitation.revoke", inv.Email)
	return c.NoContent(http.StatusNoContent)
}

type acceptReq struct {
	Token string `json:"token"`
}

// Accept consumes a valid invitation, creating the user in that org's realm only
// with the assigned roles (US2/FR-014). Public: the token is the credential.
func (h *Handlers) Accept(c echo.Context) error {
	var req acceptReq
	if err := c.Bind(&req); err != nil || req.Token == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "token required")
	}
	org, secret, ok := parseToken(req.Token)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "malformed token")
	}
	ctx := c.Request().Context()
	inv, err := h.store.GetByTokenHash(ctx, org, hashSecret(secret))
	if errors.Is(err, ErrNotFound) {
		return echo.NewHTTPError(http.StatusGone, "invitation invalid")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "lookup failed")
	}
	if inv.Status != StatusPending || h.now().After(inv.ExpiresAt) {
		return echo.NewHTTPError(http.StatusGone, "invitation expired or already used")
	}
	if h.kc == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "provisioning not configured")
	}
	uid, err := h.kc.CreateUser(ctx, org, idp.User{Username: inv.Email, Email: inv.Email, Enabled: true, RequiredActions: []string{"UPDATE_PASSWORD"}})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "create user failed")
	}
	for _, role := range inv.Roles {
		if err := h.kc.AssignRealmRole(ctx, org, uid, role); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "assign role failed")
		}
	}
	now := h.now()
	inv.Status = StatusAccepted
	inv.AcceptedAt = &now
	_ = h.store.Update(ctx, inv)
	h.record(ctx, org, inv.Email, "invitation.accept", inv.Email)
	return c.JSON(http.StatusOK, map[string]any{"org": org, "email": inv.Email, "roles": inv.Roles})
}

func (h *Handlers) record(ctx context.Context, org, actorID, action, target string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.Record(ctx, audit.Event{Time: h.now(), OrgID: org, Actor: actorID, Action: action, Target: target})
}

// requireOrgAdmin authenticates the org-admin token for the path org (per-org
// realm + admin role), mirroring the control-plane admin API.
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
	return "inv_" + hex.EncodeToString(b)
}
