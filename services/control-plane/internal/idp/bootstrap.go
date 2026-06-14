package idp

// BootstrapParams are the per-tenant inputs to BuildBootstrapSpec.
type BootstrapParams struct {
	Slug          string
	DisplayName   string
	AdminEmail    string
	ConsoleOrigin string // e.g. http://localhost:5173
	AdminAudience string // console token audience (control-plane API)
	MCPResource   string // mcp-client token audience (= gateway MCP resource URL)
	AccessTTL     int    // access-token lifespan (s)
	SSOIdle       int    // SSO idle timeout (s)
	SSOMax        int    // SSO max lifespan (s)
	SSLRequired   string // "none" (dev) | "external" (prod)
}

// BootstrapSpec is the full set of identity assets to create for a new tenant
// realm — the programmatic equivalent of deploy/dev/seed-keycloak.sh.
type BootstrapSpec struct {
	Realm          Realm
	ConsoleClient  Client
	ConsoleMappers []ProtocolMapper
	MCPClient      Client
	MCPMappers     []ProtocolMapper
	AdminRole      string
	AdminUser      User
}

// BuildBootstrapSpec assembles the identity assets for a tenant. The audience and
// realm-role mappers, PKCE settings, and the MCP-resource audience mirror the dev
// seed script so the OAuth alignment chain (issuer/audience) holds for the gateway
// and console (contracts/keycloak-provisioning.md).
func BuildBootstrapSpec(p BootstrapParams) BootstrapSpec {
	return BootstrapSpec{
		Realm: Realm{
			Name:                p.Slug,
			Enabled:             true,
			AccessTokenLifespan: p.AccessTTL,
			SSOIdle:             p.SSOIdle,
			SSOMax:              p.SSOMax,
			SSLRequired:         p.SSLRequired,
		},
		ConsoleClient: Client{
			ClientID:     "mcp-admin-console",
			Name:         "MCP Admin Console",
			PublicClient: true,
			StandardFlow: true,
			DirectGrants: false,
			RedirectURIs: []string{p.ConsoleOrigin + "/*"},
			WebOrigins:   []string{p.ConsoleOrigin},
			Attributes: map[string]string{
				"pkce.code.challenge.method": "S256",
				"post.logout.redirect.uris":  p.ConsoleOrigin + "/*",
			},
		},
		ConsoleMappers: []ProtocolMapper{
			{Name: "admin-api-audience", Mapper: "oidc-audience-mapper", Config: map[string]string{
				"included.custom.audience": p.AdminAudience,
				"access.token.claim":       "true",
				"id.token.claim":           "false",
			}},
			{Name: "realm-roles-id", Mapper: "oidc-usermodel-realm-role-mapper", Config: map[string]string{
				"claim.name":           "realm_access.roles",
				"jsonType.label":       "String",
				"multivalued":          "true",
				"id.token.claim":       "true",
				"access.token.claim":   "true",
				"userinfo.token.claim": "true",
			}},
		},
		MCPClient: Client{
			ClientID:     "mcp-client",
			Name:         "MCP Client",
			PublicClient: true,
			StandardFlow: true,
			DirectGrants: true,
			RedirectURIs: []string{"http://localhost:*", "http://127.0.0.1:*"},
			WebOrigins:   []string{"+"},
			Attributes:   map[string]string{"pkce.code.challenge.method": "S256"},
		},
		MCPMappers: []ProtocolMapper{
			{Name: "mcp-resource-audience", Mapper: "oidc-audience-mapper", Config: map[string]string{
				"included.custom.audience": p.MCPResource,
				"access.token.claim":       "true",
				"id.token.claim":           "false",
			}},
		},
		AdminRole: "admin",
		AdminUser: User{
			Username:        p.AdminEmail,
			Email:           p.AdminEmail,
			Enabled:         true,
			RequiredActions: []string{"UPDATE_PASSWORD"},
		},
	}
}
