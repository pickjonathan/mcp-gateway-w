# Implementation Plan: Admin Console UI (Carbon Design System)

**Branch**: `002-admin-console` | **Date**: 2026-06-13 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-admin-console/spec.md`

## Summary

A browser-based administration console that surfaces every admin-facing capability
of `001-mcp-server-runtime` (server CRUD, write-only credentials, RBAC, audit
review, quotas/health) using the **Carbon Design System handoff** shipped at the
project root. It is a **static single-page app, presentation-only**: it
authenticates against the existing per-org Keycloak realm (OAuth 2.1 + PKCE) and
talks exclusively to the existing control-plane admin API — no new backend, no new
authorization, no weakening of organization isolation or secret confidentiality.

## Technical Context

**Language/Version**: TypeScript 5.x on Node 20 LTS (build/test toolchain only).
**Primary Dependencies**: React 18 (the handoff ships React **.jsx** components +
`.d.ts` typings), Vite (build/dev), React Router (routing), TanStack Query (API
cache + polling), `oidc-client-ts` (OAuth 2.1 Authorization Code + PKCE). The
**Carbon design system is vendored from the handoff package** (tokens CSS + JSX
components + icon set) rather than pulled from npm.
**Storage**: None client-side persistent. Auth tokens held in memory (not
localStorage). All state is the control-plane API + an in-memory query cache.
**Testing**: Vitest + React Testing Library (unit/component), Playwright (e2e of
primary flows) with `axe` (accessibility), the handoff's `_adherence.oxlintrc.json`
as the design-system lint gate, plus adversarial tests (no cross-org data; secrets
never in the DOM) per Constitution V.
**Target Platform**: Evergreen browsers (last 2 major versions), desktop + tablet.
**Project Type**: Web frontend (SPA) over an existing API — a new `web/admin-console/`.
**Performance Goals**: First meaningful paint < 2s on broadband; UI feedback for
any action < 1s (SC-007); dashboard/health reflect changes within 30s (SC-008).
**Constraints**: Presentation-only (FR-023) — no new server capability; secrets
write-only (FR-013); organization isolation preserved (only the signed-in org's
data); WCAG 2.1 AA (FR-022); 100% design-system adherence (FR-021).
**Scale/Scope**: ~12–15 screens; catalogs/audit logs up to thousands of rows
(server-side pagination/filter).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Assessment |
|---|---|
| **I. Tenant Isolation Inviolable** | ✅ The console only ever renders the signed-in org's data; tokens are audience-bound (reused from `001`); it never aggregates across orgs. Org context = the subdomain the admin authenticates to. Adversarial test: an org-A session shows zero org-B data. |
| **II. Secure Execution of Any MCP** | ✅ N/A to the console (no execution). It configures servers via the API; it cannot bypass sandboxing or reach the data plane. |
| **III. Frictionless Self-Service** | ✅ This feature **is** the frictionless surface — add/govern/remove servers with seconds-to-effect, no engineering. |
| **IV. Hard Constraints Outrank Soft Goals** | ✅ The console must not weaken isolation or secret confidentiality for UX. No conflict introduced. |
| **V. Test-First, Adversarially Verified** | ✅ Plan mandates contract tests (API consumed), a11y + adherence gates, and adversarial tests (cross-org leakage, secret-in-DOM) written before the screens land. |
| **VI. Observable & Auditable** | ✅ Every console action flows through the audited control-plane API. The console *surfaces* audit + metrics; secrets never displayed/copied/logged. |
| **VII. Simplicity with Justified Complexity** | ✅ Static SPA over the existing API; **no new backend, datastore, or service**. Design system vendored from the provided package. No complexity to justify. |

**Result: PASS** (no violations; Complexity Tracking empty).

Security & Compliance alignment: served over HTTPS at the edge; no secrets
persisted client-side; tokens in memory with silent renew; CORS limited to the
console origin (deployment config, not a new capability).

## Project Structure

### Documentation (this feature)

```text
specs/002-admin-console/
├── plan.md              # This file
├── research.md          # Phase 0 — decisions (framework, auth, DS vendoring)
├── data-model.md        # Phase 1 — view models ↔ API
├── quickstart.md        # Phase 1 — run/build/test the console
├── contracts/           # Phase 1 — consumed API + auth/CORS + UI routes
│   ├── control-plane-consumed.md
│   └── ui-routes.md
└── tasks.md             # Phase 2 — /speckit-tasks (NOT created here)
```

### Source Code (repository root)

```text
web/admin-console/                 # the SPA (new)
├── index.html
├── package.json
├── vite.config.ts
├── tsconfig.json
├── .oxlintrc.json                 # = handoff _adherence.oxlintrc.json (adherence gate)
├── src/
│   ├── main.tsx                   # bootstrap + router + auth provider
│   ├── app/                       # shell (header + side nav), routes, providers
│   ├── pages/                     # Dashboard, Servers, ServerDetail, Audit, Settings, SignIn, Forbidden
│   ├── features/                  # servers/ credentials/ rbac/ audit/ quotas/ (data hooks + views)
│   ├── api/                       # typed control-plane client + TanStack Query hooks
│   ├── auth/                      # OAuth2 PKCE (per-org Keycloak), session, guards
│   └── design-system/             # VENDORED from the Carbon handoff
│       ├── components/            # Button, Tag, Tile, InlineNotification, ProgressBar,
│       │                          #   Checkbox, Search, Select, TextInput, Toggle, Tabs, DataTable*
│       ├── tokens/                # base/colors/typography/spacing/motion CSS
│       ├── icons/                 # SVG set
│       └── shell/                 # cloud-console UI-kit shell as the layout base
└── tests/
    ├── unit/                      # Vitest + Testing Library (components/hooks)
    ├── e2e/                       # Playwright (primary flows) + axe (a11y)
    └── adversarial/               # cross-org isolation, secret-never-in-DOM
```

> `DataTable*`: the handoff ships core form/nav/feedback components; the servers
> and audit tables are composed from Carbon tokens + Tile/Tag/Search/Select/Tabs
> primitives (a thin table built to spec), since a packaged DataTable is not in the
> handoff. Captured as a research decision.

**Structure Decision**: A single new SPA under `web/admin-console/`. No backend
changes beyond enabling CORS for the console origin on the existing control-plane
(deployment config). The Go services, `pkg/`, and `deploy/dev/` are untouched.

## Complexity Tracking

> No Constitution violations — section intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    | —          | —                                   |
