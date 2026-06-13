package auth

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/pkg/config"
)

// metadataDoc is the RFC 9728 (OAuth 2.0 Protected Resource Metadata) document
// for a per-org MCP endpoint.
type metadataDoc struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
	BearerMethods        []string `json:"bearer_methods_supported"`
}

// ProtectedResourceMetadataHandler returns the RFC 9728 document for the org
// identified by the request host (FR-024).
func ProtectedResourceMetadataHandler(cfg *config.Config) echo.HandlerFunc {
	return func(c echo.Context) error {
		org := authz.OrgFromHost(c.Request().Host, cfg.BaseDomain)
		if org == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "unknown organization host")
		}
		return c.JSON(http.StatusOK, metadataDoc{
			Resource:             fmt.Sprintf("https://%s.%s/mcp", org, cfg.BaseDomain),
			AuthorizationServers: []string{fmt.Sprintf(cfg.KeycloakIssuerTemplate, org)},
			ScopesSupported:      []string{"mcp:tools", "mcp:resources", "mcp:prompts"},
			BearerMethods:        []string{"header"},
		})
	}
}

// challenge builds the RFC 9728 WWW-Authenticate header value pointing clients
// at the resource-metadata document.
func challenge(cfg *config.Config, host string) string {
	org := authz.OrgFromHost(host, cfg.BaseDomain)
	return fmt.Sprintf(
		`Bearer resource_metadata="https://%s.%s/.well-known/oauth-protected-resource"`,
		org, cfg.BaseDomain,
	)
}
