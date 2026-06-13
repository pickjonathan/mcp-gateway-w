# Admin Console (Carbon Design System)

A web administration console that gives org admins a visual surface for every
admin-facing capability of the runtime — built on the **Carbon Design System**
(vendored from the project-root handoff). It is a **presentation layer**: a static
SPA that consumes the existing control-plane admin API and per-org Keycloak
identity, adding no new mutating backend.

> Feature spec & plan: `specs/002-admin-console/`. Code: `web/admin-console/`.

## What it does

| Area | Screen |
|---|---|
| Sign in (per-org realm, OAuth 2.1 + PKCE, admin role) | Sign-in / Callback / Forbidden |
| Org overview (server count, health, recent activity, denials, traffic) | Dashboard |
| Manage MCP servers (add remote/stdio, edit, enable/disable, delete) | Servers + Add/Edit form |
| Per-server config, credentials, access, health | Server detail (Tabs) |
| Write-only credentials (set / rotate / clear, never displayed) | Credentials panel |
| Role-based access (assign `allowed_roles`) | Access panel |
| Tamper-evident audit review (filters, chain status, denial highlighting) | Audit |
| Read-only rate limits + connection endpoint | Settings |

## Architecture

- **React + TypeScript + Vite**; the Carbon components ship as React + token CSS in
  the handoff and are vendored directly.
- **OAuth 2.1 + PKCE** against the org's Keycloak realm (pre-registered public
  client); tokens held in memory.
- Data via a typed, **org-scoped** API client + TanStack Query. Dashboard rate
  trends come from the Prometheus query API; everything else from the control-plane.
- The only backend additions are read-only/observability accommodations (CORS, a
  read-only `GET …/quotas`, a `credential_set` status) — no new mutating capability.

## How its guarantees are upheld

- **Org isolation** — every request is scoped to the signed-in org; an adversarial
  test proves an org-A session never requests or renders another org's data.
- **Secret confidentiality** — credentials are write-only; an adversarial test
  proves a value never remains in the DOM after set or rotate.
- **Accessibility** — WCAG 2.1 AA: a hermetic axe gate (structural) + Playwright +
  axe (full, incl. contrast); design-system **adherence** enforced by oxlint.

## Run it

See `web/admin-console/README.md` — `npm install && npm run dev` against the
`/dev-setup` dev stack. Tests: `npm run test` (unit + contract + adversarial + a11y).
