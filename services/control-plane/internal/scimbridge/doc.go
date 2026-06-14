// Package scimbridge implements a per-tenant SCIM 2.0 subset (RFC 7643/7644) that
// translates a customer IdP's directory-sync operations into Keycloak Admin API
// calls for that tenant's realm (003-tenant-provisioning US4). Each tenant
// authenticates with a write-only per-tenant bearer; deactivation (active=false)
// removes gateway access by the user's next token.
//
// Implementation lands in Phase 6 (US4); a scoping spike (T043) precedes it. This
// scaffold reserves the package.
package scimbridge
