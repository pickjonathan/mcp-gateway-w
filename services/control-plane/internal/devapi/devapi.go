// Package devapi exposes DEV-ONLY, unauthenticated helper endpoints for local
// development. It MUST NOT be mounted in production: listing realms enumerates
// tenants, which would be a cross-tenant disclosure (HC-1). main.go registers it
// only when MCP_ENV != prod.
package devapi

import (
	"net/http"
	"sort"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/idp"
)

// systemRealms are never offered as selectable tenant orgs.
var systemRealms = map[string]bool{
	"master": true, "_platform": true,
}

// Handlers serve the dev helper endpoints.
type Handlers struct {
	lister idp.RealmLister // nil when provisioning is not configured
}

// NewHandlers builds the dev handlers over a realm lister (may be nil).
func NewHandlers(lister idp.RealmLister) *Handlers { return &Handlers{lister: lister} }

// RegisterRoutes mounts the dev endpoints. DEV ONLY — the caller MUST gate this by
// environment (never register in prod).
func RegisterRoutes(e *echo.Echo, h *Handlers) {
	e.GET("/v1/dev/orgs", h.ListOrgs)
}

// ListOrgs returns the tenant realm names (minus system realms), so the dev org
// picker reflects the realms that actually exist in Keycloak.
func (h *Handlers) ListOrgs(c echo.Context) error {
	if h.lister == nil {
		return c.JSON(http.StatusOK, map[string]any{"orgs": []string{}})
	}
	realms, err := h.lister.ListRealms(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "list realms failed")
	}
	orgs := make([]string, 0, len(realms))
	for _, r := range realms {
		if !systemRealms[r] {
			orgs = append(orgs, r)
		}
	}
	sort.Strings(orgs)
	return c.JSON(http.StatusOK, map[string]any{"orgs": orgs})
}
