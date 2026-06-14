package idp

import "context"

// Keycloak is the subset of the Keycloak Admin API the control plane uses to
// provision and manage per-tenant realms. Every method is idempotent-friendly
// (callers check existence first); implementations never log secrets.
type Keycloak interface {
	RealmExists(ctx context.Context, realm string) (bool, error)
	CreateRealm(ctx context.Context, r Realm) error
	UpdateRealm(ctx context.Context, r Realm) error
	SetRealmEnabled(ctx context.Context, realm string, enabled bool) error
	DeleteRealm(ctx context.Context, realm string) error

	CreateClient(ctx context.Context, realm string, c Client) (id string, err error)
	AddProtocolMapper(ctx context.Context, realm, clientID string, m ProtocolMapper) error

	CreateRealmRole(ctx context.Context, realm, role string) error
	CreateUser(ctx context.Context, realm string, u User) (id string, err error)
	AssignRealmRole(ctx context.Context, realm, userID, role string) error
}

// Realm holds the per-tenant realm settings the bootstrap applies (mirrors the
// dev seed script: 15m access tokens, SSO idle/max, dev sslRequired=none).
type Realm struct {
	Name                string
	Enabled             bool
	AccessTokenLifespan int    // seconds
	SSOIdle             int    // seconds
	SSOMax              int    // seconds
	SSLRequired         string // "none" (dev) | "external" (prod)
}

// Client is a public PKCE OAuth client (console or MCP client).
type Client struct {
	ClientID     string
	Name         string
	PublicClient bool
	StandardFlow bool
	DirectGrants bool
	RedirectURIs []string
	WebOrigins   []string
	Attributes   map[string]string // e.g. {"pkce.code.challenge.method":"S256"}
}

// ProtocolMapper adds a claim/audience mapper to a client (e.g. the MCP-resource
// audience mapper, the realm-roles mapper).
type ProtocolMapper struct {
	Name   string
	Mapper string            // protocolMapper id, e.g. "oidc-audience-mapper"
	Config map[string]string // mapper-specific config
}

// User is an initial realm user (e.g. the org admin), created disabled-until-setup
// via a required action (set password through the invite email).
type User struct {
	Username        string
	Email           string
	Enabled         bool
	RequiredActions []string // e.g. ["UPDATE_PASSWORD"]
}
