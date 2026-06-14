package scimbridge

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/pkg/audit"
	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/idp"
)

// OrgValidator validates an org-scoped admin token (for the config endpoints).
type OrgValidator interface {
	ValidateForOrg(ctx context.Context, raw, org, audience string) (*authz.Principal, error)
}

// Handlers serve both the tenant-admin directory-sync config endpoints
// (org-admin auth) and the SCIM 2.0 bridge endpoints (per-tenant bearer auth).
// The bridge translates the IdP-emitted subset (Users create/replace, active
// toggle, group→role) to the Keycloak Admin API for the bearer's org only.
type Handlers struct {
	store      Store
	dir        idp.Directory
	audit      audit.Logger
	baseDomain string
	now        func() time.Time
}

// NewHandlers builds the SCIM handlers. dir may be nil (provisioning disabled).
func NewHandlers(store Store, dir idp.Directory, auditLog audit.Logger, baseDomain string) *Handlers {
	return &Handlers{store: store, dir: dir, audit: auditLog, baseDomain: baseDomain, now: time.Now}
}

// RegisterRoutes mounts the org-admin config routes and the bearer-auth SCIM routes.
func RegisterRoutes(e *echo.Echo, h *Handlers, v OrgValidator, adminAudience string) {
	cfg := e.Group("/v1/orgs/:org/directory-sync")
	cfg.Use(requireOrgAdmin(v, adminAudience))
	cfg.PUT("", h.Configure)
	cfg.GET("", h.GetConfig)
	cfg.POST("/rotate", h.Rotate)
	cfg.DELETE("", h.Disable)

	scim := e.Group("/scim/v2")
	scim.Use(h.scimAuth)
	scim.POST("/Users", h.CreateUser)
	scim.GET("/Users", h.QueryUsers)
	scim.PUT("/Users/:id", h.ReplaceUser)
	scim.PATCH("/Users/:id", h.PatchUser)
	scim.GET("/ServiceProviderConfig", h.serviceProviderConfig)
}

// ---- Tenant-admin config (org-admin auth) ----

type configReq struct {
	GroupRoleMappings map[string]string `json:"group_role_mappings"`
}

func (h *Handlers) scimBaseURL(org string) string {
	return fmt.Sprintf("https://%s.%s/scim/v2", org, h.baseDomain)
}

// Configure enables directory sync and issues a per-tenant bearer (shown once).
func (h *Handlers) Configure(c echo.Context) error {
	org := c.Param("org")
	var req configReq
	_ = c.Bind(&req)
	raw, hash := NewBearer(org)
	if err := h.store.Upsert(c.Request().Context(), Connection{
		OrgID: org, BearerHash: hash, GroupRoleMappings: req.GroupRoleMappings, Status: "active", CreatedAt: h.now(),
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "store connection failed")
	}
	h.record(c.Request().Context(), org, actor(c), "directory_sync.configure", org)
	return c.JSON(http.StatusOK, map[string]any{
		"status": "active", "scim_base_url": h.scimBaseURL(org), "bearer": raw, // shown ONCE
	})
}

// GetConfig returns status (never the bearer).
func (h *Handlers) GetConfig(c echo.Context) error {
	org := c.Param("org")
	conn, err := h.store.Get(c.Request().Context(), org)
	if err == ErrNotFound {
		return echo.NewHTTPError(http.StatusNotFound, "not configured")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "get failed")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"status": conn.Status, "scim_base_url": h.scimBaseURL(org), "last_sync_at": conn.LastSyncAt,
	})
}

// Rotate issues a new bearer (invalidating the old one).
func (h *Handlers) Rotate(c echo.Context) error {
	org := c.Param("org")
	conn, err := h.store.Get(c.Request().Context(), org)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "not configured")
	}
	raw, hash := NewBearer(org)
	conn.BearerHash = hash
	conn.Status = "active"
	if err := h.store.Upsert(c.Request().Context(), conn); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "rotate failed")
	}
	h.record(c.Request().Context(), org, actor(c), "directory_sync.rotate", org)
	return c.JSON(http.StatusOK, map[string]any{"status": "active", "scim_base_url": h.scimBaseURL(org), "bearer": raw})
}

// Disable removes the directory-sync connection.
func (h *Handlers) Disable(c echo.Context) error {
	org := c.Param("org")
	_ = h.store.Delete(c.Request().Context(), org)
	h.record(c.Request().Context(), org, actor(c), "directory_sync.disable", org)
	return c.NoContent(http.StatusNoContent)
}

// ---- SCIM 2.0 (per-tenant bearer auth) ----

type scimUser struct {
	Schemas  []string `json:"schemas,omitempty"`
	ID       string   `json:"id,omitempty"`
	UserName string   `json:"userName"`
	Active   *bool    `json:"active,omitempty"`
	Emails   []struct {
		Value string `json:"value"`
	} `json:"emails,omitempty"`
	Groups []struct {
		Value string `json:"value"`
	} `json:"groups,omitempty"`
}

func (u scimUser) email() string {
	if len(u.Emails) > 0 {
		return u.Emails[0].Value
	}
	return u.UserName
}

func (u scimUser) activeOrTrue() bool { return u.Active == nil || *u.Active }

// scimAuth resolves the org from the bearer ("{org}.{secret}") and verifies its
// hash. The org is taken from the bearer itself, so a tenant's bearer can only
// ever act on that tenant's realm (cross-tenant isolation is structural — HC-1).
func (h *Handlers) scimAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		org, secret, ok := ParseBearer(bearer(c.Request().Header.Get("Authorization")))
		if !ok {
			return scimError(c, http.StatusUnauthorized, "missing or malformed bearer")
		}
		conn, err := h.store.Get(c.Request().Context(), org)
		if err != nil || conn.Status != "active" || HashSecret(secret) != conn.BearerHash {
			return scimError(c, http.StatusUnauthorized, "invalid bearer")
		}
		if h.dir == nil {
			return scimError(c, http.StatusServiceUnavailable, "provisioning not configured")
		}
		c.Set("scimOrg", org)
		return next(c)
	}
}

func scimOrg(c echo.Context) string { s, _ := c.Get("scimOrg").(string); return s }

// CreateUser provisions (or re-activates) a user in the bearer's realm and maps
// group memberships to realm roles.
func (h *Handlers) CreateUser(c echo.Context) error {
	org := scimOrg(c)
	var u scimUser
	if err := c.Bind(&u); err != nil || u.UserName == "" {
		return scimError(c, http.StatusBadRequest, "userName required")
	}
	ctx := c.Request().Context()
	id, found, err := h.dir.FindUserByUsername(ctx, org, u.UserName)
	if err != nil {
		return scimError(c, http.StatusBadGateway, "directory error")
	}
	if !found {
		id, err = h.dir.CreateUser(ctx, org, idp.User{Username: u.UserName, Email: u.email(), Enabled: u.activeOrTrue()})
		if err != nil {
			return scimError(c, http.StatusBadGateway, "create user failed")
		}
	} else {
		_ = h.dir.SetUserEnabled(ctx, org, id, u.activeOrTrue())
	}
	h.applyGroups(ctx, org, id, u.Groups)
	_ = h.store.Touch(ctx, org, h.now())
	h.record(ctx, org, "scim", "directory_sync.user.upsert", u.UserName)
	return c.JSON(http.StatusCreated, h.userRep(id, u))
}

// ReplaceUser (PUT) sets the user's enabled state from active and re-maps groups.
func (h *Handlers) ReplaceUser(c echo.Context) error {
	org := scimOrg(c)
	var u scimUser
	if err := c.Bind(&u); err != nil {
		return scimError(c, http.StatusBadRequest, "invalid body")
	}
	ctx := c.Request().Context()
	_ = h.dir.SetUserEnabled(ctx, org, c.Param("id"), u.activeOrTrue())
	h.applyGroups(ctx, org, c.Param("id"), u.Groups)
	_ = h.store.Touch(ctx, org, h.now())
	return c.JSON(http.StatusOK, h.userRep(c.Param("id"), u))
}

type patchReq struct {
	Operations []struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value"`
	} `json:"Operations"`
}

// PatchUser handles the deactivation path (active=false ⇒ disable ⇒ gateway access
// removed by next token, SC-005) and reactivation.
func (h *Handlers) PatchUser(c echo.Context) error {
	org, id := scimOrg(c), c.Param("id")
	var p patchReq
	if err := c.Bind(&p); err != nil {
		return scimError(c, http.StatusBadRequest, "invalid patch")
	}
	ctx := c.Request().Context()
	for _, op := range p.Operations {
		if !strings.EqualFold(op.Op, "replace") {
			continue
		}
		// "active" may be the path, or a member of a value object.
		if strings.EqualFold(op.Path, "active") {
			if err := h.dir.SetUserEnabled(ctx, org, id, truthy(op.Value)); err != nil {
				return scimError(c, http.StatusBadGateway, "set enabled failed")
			}
		} else if m, ok := op.Value.(map[string]any); ok {
			if v, present := m["active"]; present {
				if err := h.dir.SetUserEnabled(ctx, org, id, truthy(v)); err != nil {
					return scimError(c, http.StatusBadGateway, "set enabled failed")
				}
			}
		}
	}
	_ = h.store.Touch(ctx, org, h.now())
	h.record(ctx, org, "scim", "directory_sync.user.patch", id)
	return c.JSON(http.StatusOK, map[string]any{"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:User"}, "id": id})
}

// QueryUsers supports the `userName eq "x"` filter IdPs use to reconcile.
func (h *Handlers) QueryUsers(c echo.Context) error {
	org := scimOrg(c)
	name := filterUserName(c.QueryParam("filter"))
	resources := []any{}
	if name != "" {
		if id, found, err := h.dir.FindUserByUsername(c.Request().Context(), org, name); err == nil && found {
			resources = append(resources, map[string]any{"id": id, "userName": name})
		}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"schemas":      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		"totalResults": len(resources), "Resources": resources,
	})
}

func (h *Handlers) serviceProviderConfig(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"schemas":       []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"patch":         map[string]bool{"supported": true},
		"filter":        map[string]any{"supported": true, "maxResults": 200},
		"bulk":          map[string]bool{"supported": false},
		"changePassword": map[string]bool{"supported": false},
		"sort":          map[string]bool{"supported": false},
	})
}

func (h *Handlers) applyGroups(ctx context.Context, org, userID string, groups []struct {
	Value string `json:"value"`
}) {
	conn := Connection{}
	if c, err := h.store.Get(ctx, org); err == nil {
		conn = c
	}
	for _, g := range groups {
		if role, ok := conn.GroupRoleMappings[g.Value]; ok {
			_ = h.dir.AssignRealmRole(ctx, org, userID, role)
		}
	}
}

func (h *Handlers) userRep(id string, u scimUser) map[string]any {
	return map[string]any{
		"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		"id":      id, "userName": u.UserName, "active": u.activeOrTrue(),
	}
}

func (h *Handlers) record(ctx context.Context, org, actorID, action, target string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.Record(ctx, audit.Event{Time: h.now(), OrgID: org, Actor: actorID, Action: action, Target: target})
}

// ---- helpers ----

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

func scimError(c echo.Context, status int, detail string) error {
	return c.JSON(status, map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:Error"},
		"status":  fmt.Sprintf("%d", status), "detail": detail,
	})
}

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(x, "true")
	default:
		return false
	}
}

// filterUserName extracts `x` from `userName eq "x"` (the only filter we support).
func filterUserName(filter string) string {
	parts := strings.SplitN(filter, "eq", 2)
	if len(parts) != 2 || !strings.Contains(strings.ToLower(parts[0]), "username") {
		return ""
	}
	return strings.Trim(strings.TrimSpace(parts[1]), `"`)
}
