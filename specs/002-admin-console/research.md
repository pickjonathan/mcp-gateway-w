# Research — Admin Console UI

Phase 0 decisions. Each: **Decision · Rationale · Alternatives considered**.

## R1 — UI framework: React + TypeScript

- **Decision**: Build the console in **React 18 + TypeScript**.
- **Rationale**: The Carbon handoff ships its components as **React `.jsx`** files
  with `.d.ts` typings (`Button.jsx`, `Tag.jsx`, … and a `_ds_manifest.json`/
  `_ds_bundle.js`). Using React lets us consume the supplied components directly
  with zero re-authoring — the fastest path to 100% design-system adherence
  (FR-021) and the lowest-risk route to the stated "based on the provided design
  system" requirement.
- **Alternatives**: Vue/Svelte/Angular (rejected — would require porting every
  supplied component, breaking adherence and adding effort); plain web components
  (rejected — the handoff is React, not custom elements).

## R2 — Build/dev tool: Vite

- **Decision**: **Vite** for dev server + production static build.
- **Rationale**: Fast, minimal config, first-class TS/React; produces a static
  bundle that the edge can serve — matching "static SPA, presentation-only" and
  Constitution VII (simplicity). Dev server proxies the control-plane API.
- **Alternatives**: Next.js (rejected — SSR/server runtime is unneeded for an API
  client and would add a Node server to operate, violating "no new backend"); CRA
  (deprecated/unmaintained).

## R3 — Design system: vendor the handoff package (not npm `@carbon/react`)

- **Decision**: Vendor the handoff assets into `src/design-system/` — token CSS
  (`tokens/*.css`), the JSX components, and the icon set — and load the global
  token CSS in order per `_ds_manifest.json.globalCssPaths`. Treat the manifest's
  component list as the approved inventory.
- **Rationale**: The request is explicitly "based on the design system **located at
  the project root**." The handoff is the source of truth (it even ships an
  adherence config and a SKILL). Vendoring pins exactly the provided tokens
  (IBM Plex; blue-60 `#0f62fe`; 16-col grid; spacing 01–13; semantic `--text/layer/
  field/border/support/button` tokens) and components.
- **Alternatives**: `@carbon/react` from npm (rejected for v1 — risks drift from
  the provided handoff; noted as a compatible future swap since the vocabulary
  matches); rebuilding components from scratch (rejected — wasteful, adherence risk).

## R4 — Tables: compose from handoff primitives

- **Decision**: Build the **Servers** and **Audit** tables as thin components over
  Carbon tokens + provided primitives (Tile surface, Tag for status/roles, Search,
  Select for filters, Tabs for detail), following Carbon DataTable visual spec.
- **Rationale**: The handoff inventory (Button, Tag, Tile, InlineNotification,
  ProgressBar, Checkbox, Search, Select, TextInput, Toggle, Tabs) has **no packaged
  DataTable**; the data-dense screens still must look/behave Carbon-correct.
- **Alternatives**: pull `@carbon/react` DataTable only (rejected for v1 — mixes
  sources; revisit if richer table features are needed).

## R5 — Authentication: OAuth 2.1 Authorization Code + PKCE (per-org realm)

- **Decision** (per Clarifications 2026-06-13): A **pre-registered public client**
  (a known `client_id`, e.g. `mcp-admin-console`, provisioned per realm) using
  **Authorization Code + PKCE** against the org's Keycloak realm (issuer
  `…/realms/{org}`), via `oidc-client-ts`. The org is taken from the host
  (`{org}.{base}`), exactly like the data plane. Acquire a token for the **admin
  audience** and require the admin role; tokens kept **in memory** with silent
  renew; sign-out clears session.
- **Rationale**: Reuses `001`'s identity model (per-org realm, audience-bound,
  admin role). A first-party admin console is conventionally a pre-registered
  public client — predictable setup, no per-session registration. PKCE is current
  best practice for SPAs; in-memory tokens reduce XSS blast radius.
- **Alternatives**: Dynamic client registration (RFC 7591, as the MCP data-plane
  clients use) — rejected for the console: unnecessary moving parts for a
  first-party UI. Implicit flow (rejected — deprecated, token leakage). A
  Backend-for-Frontend session-cookie proxy (rejected for v1 — adds a server;
  **noted** as a strong token-storage hardening option if XSS risk demands it,
  justified under Constitution VII at that time).

## R6 — Data layer: typed fetch client + TanStack Query

- **Decision**: A small typed client wrapping the control-plane endpoints, with
  **TanStack Query** for caching, request dedup, retries, and **polling** of health/
  usage (SC-008). Mutations (create/edit/enable/delete/credentials/limits)
  invalidate the relevant queries for instant UI refresh.
- **Rationale**: Right-sized for an API-backed admin app; gives loading/empty/error
  states (FR-020) and periodic refresh without bespoke plumbing.
- **Alternatives**: Redux/MobX global store (rejected — heavier than needed, YAGNI);
  hand-rolled fetch + effects (rejected — reinvents caching/retry/polling).

## R7 — Cross-origin + token handling (deployment config, not a new capability)

- **Decision**: Serve the console over HTTPS at the per-org admin host; the
  control-plane must **allow the console origin via CORS** (preflight + credentials
  off, bearer in `Authorization`). Document this as the only required
  control-plane-side change — a **config addition**, not a new capability or an
  authz bypass (FR-023 preserved).
- **Rationale**: A browser SPA on a different origin than the API requires CORS.
  No endpoint/behavior changes; the API still enforces admin role + org audience.
- **Alternatives**: same-origin reverse proxy at the edge (viable alternative that
  avoids CORS entirely; noted — chosen approach keeps the console deployable as
  pure static assets).

## R8 — Testing & quality gates (Constitution V)

- **Decision**: **Vitest + React Testing Library** for components/hooks; **Playwright**
  for e2e of the primary flows with **`axe`** for WCAG 2.1 AA (FR-022); the handoff
  **`_adherence.oxlintrc.json`** as a CI lint gate (FR-021). **Adversarial tests**
  (written first): (a) an org-A session never renders org-B data; (b) a stored
  secret value never appears in the DOM, a copy action, or an error — even after
  set/rotate.
- **Rationale**: Directly enforces the spec's hard guarantees and the
  test-first/adversarial principle; adherence + a11y become mechanical gates.
- **Alternatives**: manual QA only (rejected — non-negotiable per Constitution V).

## R9 — Hosting/serving

- **Decision**: Production = **static assets behind the edge** (the existing
  `*.{base}` edge), no new service. Dev = Vite dev server proxying the dev
  control-plane (`:8090`) and the dev Keycloak (`:8081`).
- **Rationale**: Keeps the footprint minimal (VII); aligns with the per-org host
  model.
- **Alternatives**: a Go static-file service (rejected for v1 — unnecessary service).

## R10 — Client observability

- **Decision**: Rely on the **server-side** audit + metrics/traces (already built in
  `001`) that the console *surfaces*; keep client telemetry minimal for v1 (console
  errors only). A browser OpenTelemetry SDK is **deferred**.
- **Rationale**: Avoids added weight/PII risk; the meaningful security/audit signal
  is server-side. Revisit if real-user monitoring is needed (justify under VII).
- **Alternatives**: full RUM/OTel browser SDK now (rejected — YAGNI for v1).

## Resolved unknowns

No `NEEDS CLARIFICATION` remained from the spec; the framework/auth/DS-vendoring
choices above were the open technical questions and are now decided.
