package authz

import (
	"context"
	"crypto"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// KeySource resolves the public signing key for a given issuer and key id.
type KeySource interface {
	KeyFor(ctx context.Context, issuer, kid string) (crypto.PublicKey, error)
}

// JWTValidator validates RS256 access tokens, enforcing issuer and audience
// binding derived from the request host (one Keycloak realm per org).
type JWTValidator struct {
	baseDomain       string
	issuerTemplate   string // fmt template taking the org slug
	resourceTemplate string // optional fmt template (org) for the MCP resource/audience
	keys             KeySource
}

// NewJWTValidator builds a validator. issuerTemplate takes the org slug, e.g.
// "https://auth.mcp.example.com/realms/%s". resourceTemplate (optional) overrides
// the MCP resource/audience URL — see MCPResource.
func NewJWTValidator(baseDomain, issuerTemplate, resourceTemplate string, keys KeySource) *JWTValidator {
	return &JWTValidator{baseDomain: baseDomain, issuerTemplate: issuerTemplate, resourceTemplate: resourceTemplate, keys: keys}
}

// MCPResource returns the per-org MCP resource URL — the value advertised in
// RFC 9728 metadata and required as the token audience. When template is
// non-empty it is fmt-applied to the org; otherwise the canonical
// https://{org}.{baseDomain}/mcp is used.
func MCPResource(org, baseDomain, template string) string {
	if template != "" {
		return fmt.Sprintf(template, org)
	}
	return fmt.Sprintf("https://%s.%s/mcp", org, baseDomain)
}

type claims struct {
	jwt.RegisteredClaims
	PreferredUsername string `json:"preferred_username"`
	RealmAccess       struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// Validate verifies the token's signature, issuer, audience, and expiry and
// returns the resolved principal. The audience MUST equal the org's MCP
// resource URL derived from host (FR-023).
func (v *JWTValidator) Validate(ctx context.Context, raw, host string) (*Principal, error) {
	org := OrgFromHost(host, v.baseDomain)
	if org == "" {
		return nil, ErrUnknownOrg
	}
	return v.verify(ctx, raw, org, MCPResource(org, v.baseDomain, v.resourceTemplate))
}

// ValidateForOrg verifies a token for an explicit org (path-based APIs) against
// the given expected audience.
func (v *JWTValidator) ValidateForOrg(ctx context.Context, raw, org, audience string) (*Principal, error) {
	if org == "" {
		return nil, ErrUnknownOrg
	}
	return v.verify(ctx, raw, org, audience)
}

// ValidateForRealm verifies a token issued by an explicit realm against the given
// audience. Unlike ValidateForOrg, realm is not a tenant org slug — it names the
// issuing realm directly (e.g. the platform realm for operator/cross-tenant APIs,
// 003-tenant-provisioning). The returned Principal.OrgID carries that realm name;
// callers gate on roles (e.g. platform-admin), not on org.
func (v *JWTValidator) ValidateForRealm(ctx context.Context, raw, realm, audience string) (*Principal, error) {
	if realm == "" {
		return nil, ErrUnknownOrg
	}
	return v.verify(ctx, raw, realm, audience)
}

func (v *JWTValidator) verify(ctx context.Context, raw, org, audience string) (*Principal, error) {
	wantIssuer := fmt.Sprintf(v.issuerTemplate, org)
	keyfunc := func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method %q", t.Method.Alg())
		}
		kid, _ := t.Header["kid"].(string)
		return v.keys.KeyFor(ctx, wantIssuer, kid)
	}
	var c claims
	tok, err := jwt.ParseWithClaims(raw, &c, keyfunc,
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(wantIssuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !tok.Valid {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if c.Subject == "" {
		return nil, fmt.Errorf("%w: missing subject", ErrInvalid)
	}
	return &Principal{OrgID: org, UserID: c.Subject, Username: c.PreferredUsername, Roles: c.RealmAccess.Roles}, nil
}
