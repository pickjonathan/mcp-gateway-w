# Contract — Control-plane API consumed by the console

The console is a **client** of the existing `001` control-plane admin API. This is
the consumption contract: what it calls, how it authenticates, and the deployment
requirement (CORS). No endpoint behavior changes.

## Authentication (OAuth 2.1 + PKCE)

```
Browser SPA (public client)                    Keycloak (realm = {org})         Control-plane API
  │  1. discover realm from host {org}.{base}                                       │
  │  2. Authorization Code + PKCE  ───────────▶  /realms/{org}/protocol/openid-...  │
  │  3. redirect back with code, exchange (PKCE verifier) ──────▶ access token      │
  │     (audience = admin API; requires "admin" role)                                │
  │  4. call API with  Authorization: Bearer <token>  ─────────────────────────────▶│ validate aud+role+org
```

- Org is taken from the host (`{org}.{base}`), matching the data plane.
- Token held **in memory**; **silent renew** before expiry; sign-out clears it.
- A non-admin token → API returns 403 → console shows Forbidden.
- A token for another org → API rejects (audience/realm mismatch); console never
  requests cross-org data.

## Endpoints consumed

| Method | Path | Purpose | Maps to |
|---|---|---|---|
| GET | `/v1/orgs/{org}/servers` | list catalog | US2, Dashboard |
| POST | `/v1/orgs/{org}/servers` | add server | US2 |
| GET | `/v1/orgs/{org}/servers/{id}` | server detail | US2 |
| PATCH | `/v1/orgs/{org}/servers/{id}` | edit / enable-disable / set `allowed_roles` | US2, US4 |
| DELETE | `/v1/orgs/{org}/servers/{id}` | delete (kill-switch) | US2 |
| PUT | `/v1/orgs/{org}/servers/{id}/credentials` | set org-shared secret (write-only) | US3 |
| DELETE | `/v1/orgs/{org}/servers/{id}/credentials` | clear org-shared secret | US3 |
| PUT | `/v1/orgs/{org}/servers/{id}/credentials/me` | set caller's per-user secret | US3 |
| DELETE | `/v1/orgs/{org}/servers/{id}/credentials/me` | clear caller's per-user secret | US3 |
| GET | `/v1/orgs/{org}/audit` | bare array of hash-chained audit records (newest-first); **no** chain-status field — `Verify()` is not routed | US5 |
| GET | `/v1/orgs/{org}/quotas` *(new, read-only)* | configured per-org/per-user limits (display only) | US6 |
| GET | metrics query API (Prometheus) *(read-only)* | request/denial/error rate trends | US6, Dashboard |

Request/response shapes are defined by `001` (`specs/001-mcp-server-runtime/`).
The console treats credential `PUT`/`DELETE` as **204, value never echoed**. The
two read-only rows above are the accommodations agreed in Clarifications
(2026-06-13); neither mutates state nor changes authorization (FR-023).

## Deployment requirement — CORS (config, not a capability)

- The control-plane MUST allow the console origin (`https://{org}.{base}` or the
  admin host) for the methods above, accepting the `Authorization` header.
- This is a **configuration** addition only — no endpoint/authz change (FR-023).
- Alternative that needs no CORS: serve the console **same-origin** behind the edge
  and route `/v1/...` to the control-plane (documented in research R7/R9).

## Dependencies (resolved — Clarifications 2026-06-13)

- **Quotas (FR-017)** → **read-only display**: add a small read-only quotas
  endpoint (`GET …/quotas`) returning the configured limits; no editing in v1.
- **Usage rates (FR-005/FR-019)** → **query Prometheus**: the console queries the
  metrics system's query API for rate charts (reachable via the edge same-origin
  proxy or CORS); health/counts/denials come from servers + audit.
- **Credential "is set" status**: confirm the servers API surfaces set/not-set so
  the console can show status without reading values; add a non-secret status field
  if absent (implementation detail for `/speckit-tasks`).
