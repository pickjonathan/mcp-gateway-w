// Package invites implements org-scoped user invitations for 003-tenant-provisioning
// (US2): a tenant admin invites a user by email with assigned roles; the invitee
// accepts to get an account in that tenant's realm only. Invitation records are
// org-scoped (org_id + RLS); the raw accept token is emailed once and never stored.
//
// Implementation lands in Phase 4 (US2). This scaffold reserves the package.
package invites
