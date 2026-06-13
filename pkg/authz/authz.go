// Package authz validates OAuth 2.0 access tokens for the MCP gateway acting as
// a protected resource server, enforcing per-org issuer + audience binding so a
// token minted for one organization cannot be used against another (HC-1).
package authz

import (
	"context"
	"errors"
	"strings"
)

// Principal is the resolved identity for an authenticated request.
type Principal struct {
	OrgID  string
	UserID string
	Roles  []string
}

// HasRole reports whether the principal carries the given role.
func (p *Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Validator validates a bearer token for the given request host and returns the
// resolved principal.
type Validator interface {
	Validate(ctx context.Context, token, host string) (*Principal, error)
}

// OrgValidator validates a token for an explicit org against an expected
// audience (used by path-based APIs such as the control plane).
type OrgValidator interface {
	ValidateForOrg(ctx context.Context, token, org, audience string) (*Principal, error)
}

var (
	// ErrUnknownOrg is returned when the request host is not a recognized
	// "{org}.{baseDomain}" address.
	ErrUnknownOrg = errors.New("authz: unknown organization host")
	// ErrInvalid is returned when a token fails validation.
	ErrInvalid = errors.New("authz: invalid token")
)

// OrgFromHost extracts the org slug from "{org}.baseDomain[:port]". It returns
// "" when host is not a subdomain of baseDomain.
func OrgFromHost(host, baseDomain string) string {
	host = strings.ToLower(host)
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	suffix := "." + strings.ToLower(baseDomain)
	if strings.HasSuffix(host, suffix) {
		return strings.TrimSuffix(host, suffix)
	}
	return ""
}
