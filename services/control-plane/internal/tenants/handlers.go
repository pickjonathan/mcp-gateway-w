package tenants

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
)

// Handlers serve the platform (operator) tenant API. All routes are mounted under
// requirePlatformAdmin (platform realm + platform-admin role).
type Handlers struct {
	svc   *Service
	store Store
}

// NewHandlers builds the platform API handlers.
func NewHandlers(svc *Service, store Store) *Handlers { return &Handlers{svc: svc, store: store} }

func actor(c echo.Context) string {
	if p, ok := c.Get("principal").(*authz.Principal); ok && p != nil {
		return p.UserID
	}
	return ""
}

type provisionReq struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	AdminEmail  string `json:"admin_email"`
}

// Create provisions a tenant (US1). 202 with {tenant, job}; 409 slug taken; 422 invalid.
func (h *Handlers) Create(c echo.Context) error {
	var req provisionReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	t, job, err := h.svc.Provision(c.Request().Context(), ProvisionRequest{
		Slug: req.Slug, DisplayName: req.DisplayName, AdminEmail: req.AdminEmail, Actor: actor(c),
	})
	switch {
	case errors.Is(err, ErrSlugTaken):
		return echo.NewHTTPError(http.StatusConflict, "slug already exists or reserved")
	case errors.Is(err, ErrInvalidSlug):
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrNotConfigured):
		return echo.NewHTTPError(http.StatusServiceUnavailable, "provisioning not configured")
	case err != nil:
		// Provisioning ran but failed mid-saga (compensated): report 502 with the job.
		return c.JSON(http.StatusBadGateway, map[string]any{"tenant": t, "job": job, "error": "provisioning_failed"})
	}
	return c.JSON(http.StatusAccepted, map[string]any{"tenant": t, "job": job})
}

// List returns all tenants (platform-scoped).
func (h *Handlers) List(c echo.Context) error {
	ts, err := h.store.ListTenants(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "list tenants failed")
	}
	return c.JSON(http.StatusOK, map[string]any{"tenants": ts})
}

// Get returns one tenant.
func (h *Handlers) Get(c echo.Context) error {
	t, err := h.store.GetTenant(c.Request().Context(), c.Param("slug"))
	if errors.Is(err, ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "get tenant failed")
	}
	return c.JSON(http.StatusOK, t)
}

// GetJob returns a provisioning job's status.
func (h *Handlers) GetJob(c echo.Context) error {
	j, err := h.store.GetJob(c.Request().Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "get job failed")
	}
	return c.JSON(http.StatusOK, j)
}

// Suspend disables a tenant (US3).
func (h *Handlers) Suspend(c echo.Context) error { return h.lifecycle(c, h.svc.Suspend) }

// Resume re-enables a suspended tenant (US3).
func (h *Handlers) Resume(c echo.Context) error { return h.lifecycle(c, h.svc.Resume) }

// Delete removes a tenant (US3); audit retained >= 1y.
func (h *Handlers) Delete(c echo.Context) error { return h.lifecycle(c, h.svc.Delete) }

func (h *Handlers) lifecycle(c echo.Context, fn func(ctx context.Context, slug, actor string) (Tenant, error)) error {
	t, err := fn(c.Request().Context(), c.Param("slug"), actor(c))
	switch {
	case errors.Is(err, ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
	case errors.Is(err, ErrConflict):
		return echo.NewHTTPError(http.StatusConflict, "tenant in a conflicting state")
	case errors.Is(err, ErrNotConfigured):
		return echo.NewHTTPError(http.StatusServiceUnavailable, "provisioning not configured")
	case err != nil:
		return echo.NewHTTPError(http.StatusInternalServerError, "lifecycle action failed")
	}
	return c.JSON(http.StatusAccepted, map[string]any{"tenant": t})
}
