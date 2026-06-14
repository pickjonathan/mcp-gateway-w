// Package idp integrates with the Keycloak Admin API to provision per-tenant
// identity assets — realm, OAuth clients, protocol mappers, roles, users — and to
// suspend/delete realms. It is the Half-A "tenant bootstrap" mechanism of
// 003-tenant-provisioning.
//
// The Keycloak dependency sits behind the Keycloak interface so call sites are
// hermetically testable (httptest). Implementations MUST NOT log token or secret
// values (Constitution VI / secret confidentiality).
package idp
