// Package tenants implements the platform-scoped tenant registry and lifecycle
// (provision / suspend / resume / delete) for 003-tenant-provisioning. A tenant
// is a Keycloak realm; this package owns the registry record and orchestrates the
// realm bootstrap (via the idp package) as an idempotent, compensating saga.
//
// Its API surface is the operator (platform) API, authorized against the platform
// realm + platform-admin role — distinct from any tenant org token (HC-1).
package tenants
