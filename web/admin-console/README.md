# MCP Admin Console

A web administration console for the multi-tenant MCP runtime, built on the
**Carbon Design System** (vendored from the project-root handoff). It is a
presentation layer over the existing control-plane admin API and per-org Keycloak
identity — see [`specs/002-admin-console/`](../../specs/002-admin-console/).

## Stack

React 18 + TypeScript + Vite · TanStack Query · React Router · oidc-client-ts
(OAuth 2.1 + PKCE) · Vitest + Testing Library + MSW + axe-core · Playwright (e2e).

## Develop

```sh
cp .env.example .env.local        # point at your dev control-plane + Keycloak realm
npm install
npm run dev                       # http://localhost:5173 (proxies /v1 + /metrics-api)
```

Bring up the backend first with the `/dev-setup` skill (control-plane :8090,
Keycloak :8081, Prometheus :9090).

## Scripts

| Command | What |
|---|---|
| `npm run dev` | Vite dev server (proxies `/v1`→control-plane, `/metrics-api`→Prometheus) |
| `npm run build` | `tsc -b` typecheck + production build to `dist/` |
| `npm run test` | Vitest — unit, contract, **adversarial** (cross-org, secret-never-in-DOM), a11y |
| `npm run lint` | oxlint with the Carbon **adherence** config |
| `npm run test:e2e` | Playwright + axe (needs `npx playwright install` + a mock-backed dev server) |

## Build output / performance budget

Production bundle (gzip): **JS ≈ 80 KB**, CSS ≈ 3 KB. Comfortably under a 250 KB
first-load budget — first meaningful paint < 2s on broadband (SC-001/SC-007).

## Layout

```
src/
  app/         shell (header + side nav), routes, providers, feedback, srOnly
  auth/        OAuth2 PKCE (oidc), AuthProvider/session, org-from-host, RequireAdmin guard
  api/         typed control-plane client + TanStack Query hooks (servers, credentials, audit, quotas, metrics)
  pages/       SignIn, Callback, Forbidden, Dashboard, Servers, ServerForm, ServerDetail, Audit, Settings
  features/    dashboard, servers (ConfirmDialog), credentials, rbac
  design-system/  VENDORED Carbon: tokens, components, icons, shell, styles.css
tests/         unit, contract, adversarial, a11y (vitest); e2e (Playwright); mocks (MSW)
```

## Hosting

Static assets (`dist/`) served at the edge (same-origin with the control-plane, so
`/v1` and `/metrics-api` route without CORS). For a cross-origin deployment, set
`MCP_CONSOLE_ORIGINS` on the control-plane to allow the console origin.

## Security notes

- Tokens are held **in memory** (oidc `InMemoryWebStorage`); PKCE flow-state in
  `sessionStorage`. No secrets persisted client-side.
- Credentials are **write-only**: the console can set/rotate/clear but never reads
  or renders stored values (proven by `tests/adversarial/secret-never-in-dom`).
- Org isolation is structural: every request is scoped to the session's org
  (proven by `tests/adversarial/cross-org-isolation`).
