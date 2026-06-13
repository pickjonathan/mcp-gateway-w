package auth

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/pkg/config"
)

// RequireAuth returns Echo middleware enforcing a valid bearer token via the
// given validator. A missing/invalid token (or a nil validator) yields an
// RFC 9728 401 challenge and exposes no server surface (US1 scenario 3). onDeny,
// if non-nil, is invoked with a short reason for each rejection (audit, FR-010).
func RequireAuth(cfg *config.Config, v authz.Validator, onDeny func(c echo.Context, reason string)) echo.MiddlewareFunc {
	deny := func(c echo.Context, reason, msg string) error {
		c.Response().Header().Set("WWW-Authenticate", challenge(cfg, c.Request().Host))
		if onDeny != nil {
			onDeny(c, reason)
		}
		return echo.NewHTTPError(http.StatusUnauthorized, msg)
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tok := bearer(c.Request().Header.Get("Authorization"))
			if tok == "" || v == nil {
				return deny(c, "missing_token", "missing or unsupported bearer token")
			}
			p, err := v.Validate(c.Request().Context(), tok, c.Request().Host)
			if err != nil {
				return deny(c, "invalid_token", "invalid token")
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
