package tenants

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
)

// RealmValidator validates a token issued by an explicit realm (the platform
// realm). Satisfied by *authz.JWTValidator.
type RealmValidator interface {
	ValidateForRealm(ctx context.Context, raw, realm, audience string) (*authz.Principal, error)
}

// requirePlatformAdmin authenticates operator requests against the platform realm
// and requires the platform-admin role. A tenant/org token can never satisfy this
// (different realm + role), keeping cross-tenant operations off tenant credentials
// (HC-1 / Constitution I).
func requirePlatformAdmin(v RealmValidator, realm, audience string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tok := bearer(c.Request().Header.Get("Authorization"))
			if tok == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
			}
			p, err := v.ValidateForRealm(c.Request().Context(), tok, realm, audience)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}
			if !p.HasRole("platform-admin") {
				return echo.NewHTTPError(http.StatusForbidden, "platform-admin role required")
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
