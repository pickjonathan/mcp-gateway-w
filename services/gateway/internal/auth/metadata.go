package auth

import (
	"fmt"
	"net/http"
	"strings"

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
			Resource:             authz.MCPResource(org, cfg.BaseDomain, cfg.ResourceTemplate),
			AuthorizationServers: []string{fmt.Sprintf(cfg.KeycloakIssuerTemplate, org)},
			ScopesSupported:      []string{"mcp:tools", "mcp:resources", "mcp:prompts"},
			BearerMethods:        []string{"header"},
		})
	}
}

// challenge builds the RFC 9728 WWW-Authenticate header value pointing clients
// at the resource-metadata document (same scheme/host/port as the resource, so
// the local http://…:8080 form works as well as the canonical https URL).
func challenge(cfg *config.Config, host string) string {
	org := authz.OrgFromHost(host, cfg.BaseDomain)
	resource := authz.MCPResource(org, cfg.BaseDomain, cfg.ResourceTemplate)
	base := strings.TrimSuffix(resource, "/mcp")
	return fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource"`, base)
}
