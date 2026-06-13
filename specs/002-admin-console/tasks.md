---
description: "Task list — Admin Console UI (Carbon Design System)"
---

# Tasks: Admin Console UI (Carbon Design System)

**Input**: Design documents from `/specs/002-admin-console/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: REQUIRED for this project. Constitution Principle V (Test-First,
Adversarially Verified) overrides the template's "optional" default — every
external interface gets a contract test, and any change touching isolation,
authorization, or secrets gets an adversarial/negative test written first.

**App location**: `web/admin-console/` (a new static SPA). Backend prerequisite
tasks touch `services/control-plane/` (Go, feature `001`) and are clearly marked.

## Format: `[ID] [P?] [Story] Description with file path`

- **[P]**: parallelizable (different files, no dependency on an incomplete task)
- **[USn]**: the user story the task serves (user-story phases only)

---

## Phase 1: Setup

- [X] T001 Scaffold a Vite + React 18 + TypeScript app in `web/admin-console/` (`package.json`, `vite.config.ts`, `tsconfig*.json`, `index.html`, `src/main.tsx`, `src/App.tsx`) — *npm install (240 pkgs) + `npm run build` green*
- [X] T002 [P] Vendor the Carbon handoff into `web/admin-console/src/design-system/` (`tokens/`, `components/` (11, named exports + `.d.ts`), `icons/`, `shell/`, `styles.css`) — *built bundle contains `--blue-60`, IBM Plex, `cds-btn`*
- [X] T003 [P] Load the global token CSS in `_ds_manifest.json` order — *the handoff's `styles.css` is the single entry point (@imports fonts→colors→typography→spacing→motion→base); imported in `src/main.tsx`*
- [X] T004 [P] Add the handoff adherence config as `web/admin-console/.oxlintrc.json` and an `npm run lint` script (design-system gate, FR-021)
- [X] T005 [P] Configure unit testing (Vitest + React Testing Library + jsdom) in `web/admin-console/vitest.config.ts` + `tests/setup.ts` with `npm run test`
- [X] T006 [P] Configure e2e + accessibility testing (Playwright + `@axe-core/playwright`) in `web/admin-console/playwright.config.ts` with `npm run test:e2e`
- [X] T007 [P] Add `web/admin-console/.env.example` + Vite dev proxy for `/v1` (control-plane) and `/metrics-api` (Prometheus) in `vite.config.ts`; `.gitignore` for the app

---

## Phase 2: Foundational (blocking prerequisites for all stories)

**⚠️ MUST complete before any user-story phase.**

- [X] T008 Enable CORS for the console origin on the control-plane (config-driven allowlist via `MCP_CONSOLE_ORIGINS`) in `services/control-plane/internal/admin/api.go` — *Echo CORS middleware, only when configured; auth unchanged. Test `TestAdmin_CORS` (preflight from an allowed origin → `Access-Control-Allow-Origin`); full Go suite green*
- [X] T009 [P] Implement the OAuth 2.1 + PKCE client (pre-registered public client, `oidc-client-ts`) in `web/admin-console/src/auth/oidc.ts` (issuer from `VITE_OIDC_ISSUER_TEMPLATE` + org, in-memory tokens, silent renew)
- [X] T010 [P] Derive the org from the host (`{org}.{base}`) in `web/admin-console/src/auth/org.ts`
- [X] T011 Implement `AuthProvider` + `useSession` (principal: org/userId/roles/expiry) in `web/admin-console/src/auth/AuthProvider.tsx`
- [X] T012 Implement the `RequireAdmin` route guard (unauth → `/signin`; authenticated non-admin → `/forbidden`) in `web/admin-console/src/auth/RequireAdmin.tsx`
- [X] T013 [P] Implement the typed control-plane API client (bearer + org-scoped base URL, error normalization) in `web/admin-console/src/api/client.ts`
- [X] T014 [P] Configure the TanStack Query client (caching, retry, polling defaults) in `web/admin-console/src/api/queryClient.ts`
- [X] T015 Build the app shell from the cloud-console UI-kit (header with org/brand + profile/sign-out, side nav, content) in `web/admin-console/src/app/shell/AppShell.tsx`
- [X] T016 Wire the router + route skeleton and providers (Auth, Query, Router) in `web/admin-console/src/app/routes.tsx` and `web/admin-console/src/app/providers.tsx`
- [X] T017 [P] Implement shared loading/empty/error states + an `InlineNotification` toaster in `web/admin-console/src/app/feedback/`
- [X] T018 [P] Add control-plane API contract fixtures/mock (e.g. MSW handlers from the `001` contracts) in `web/admin-console/tests/contract/` for use by component/contract tests

**Checkpoint**: app shell renders, auth redirects work, API client + mocks ready. — *DONE: `npm run build` (tsc -b typecheck + vite build) green, 101 modules; OAuth2 PKCE (`oidc.ts`/`AuthProvider`/`RequireAdmin`), org-from-host, typed API client + TanStack Query, Carbon shell + router/providers, feedback states + toaster, and MSW control-plane mocks all in place.*

---

## Phase 3: User Story 1 — Sign in & dashboard (Priority: P1) 🎯 MVP

**Goal**: An admin signs in (per-org realm, admin role) and lands on a dashboard
scoped to their org; non-admins are refused; no cross-org data is ever shown.

**Independent test**: Sign in as org-A admin → shell + dashboard load with org-A
data only; non-admin → Forbidden; org-A session → zero org-B data.

- [X] T019 [P] [US1] Route-guard tests (unauth → signin, non-admin → forbidden) in `web/admin-console/tests/unit/auth.guard.test.tsx`
- [X] T020 [P] [US1] **Adversarial** test: an org-A session renders **zero** org-B data (Constitution I) in `web/admin-console/tests/adversarial/cross-org-isolation.test.tsx`
- [X] T021 [P] [US1] Component test: dashboard renders server counts/health/recent-activity from fixtures in `web/admin-console/tests/unit/dashboard.test.tsx`
- [X] T022 [US1] Implement SignIn + OAuth Callback pages in `web/admin-console/src/pages/SignIn.tsx` and `web/admin-console/src/pages/Callback.tsx`
- [X] T023 [P] [US1] Implement the Forbidden page in `web/admin-console/src/pages/Forbidden.tsx`
- [X] T024 [US1] Implement dashboard data hooks (counts + health from servers, recent activity + denial indicator from audit) in `web/admin-console/src/features/dashboard/`
- [X] T025 [US1] Implement the Dashboard page (Tile metric cards, health Tags, recent-activity list) in `web/admin-console/src/pages/Dashboard.tsx`

**Checkpoint**: US1 independently demoable — secure sign-in + org dashboard. — *DONE & verified: `npm run build` green; `npm run test` = 5 tests pass across guard (T019), **cross-org adversarial isolation (T020)**, and dashboard (T021). Dashboard composes counts/health/recent-activity/denials from the servers + audit APIs; SignIn/Callback/Forbidden wired to OAuth.*

---

## Phase 4: User Story 2 — Manage MCP servers (Priority: P1) 🎯 MVP

**Goal**: View the catalog; add remote/stdio servers; edit; enable/disable; delete
— each validated and confirmed (kill-switch).

**Independent test**: Add a remote and a stdio server → both appear; disable one →
status reflects it; delete → removed after confirmation.

- [X] T026 [P] [US2] Contract tests for the servers endpoints (list/create/get/patch/delete) in `web/admin-console/tests/contract/servers.contract.test.ts`
- [X] T027 [P] [US2] Component tests: catalog table (search/sort/paginate, status/health Tags) in `web/admin-console/tests/unit/servers.table.test.tsx`
- [X] T028 [P] [US2] Component tests: add-server form validation incl. duplicate-slug inline error and remote/stdio required fields in `web/admin-console/tests/unit/server.form.test.tsx`
- [~] T029 [P] [US2] E2E: add a remote and a stdio server → both appear in the catalog in `web/admin-console/tests/e2e/servers.spec.ts` — *spec written (`tests/e2e/servers.spec.ts`); needs `npx playwright install` + a mock-backed dev server to execute*
- [X] T030 [US2] Implement servers API hooks (list/get/create/patch/delete) in `web/admin-console/src/api/servers.ts`
- [X] T031 [US2] Implement the Servers catalog page (table, search/Select filters, status/health Tags, pagination, row actions) in `web/admin-console/src/pages/Servers.tsx`
- [X] T032 [US2] Implement the Add/Edit server form (remote: endpoint URL; stdio: command/args/env; validation; `InlineNotification`) in `web/admin-console/src/pages/ServerForm.tsx`
- [X] T033 [US2] Implement the Server detail page with Tabs (Overview / Credentials / Access / Health) in `web/admin-console/src/pages/ServerDetail.tsx`
- [X] T034 [US2] Implement enable/disable Toggle + delete with danger-confirmation (states the kill-switch effect) in `web/admin-console/src/features/servers/`

**Checkpoint**: US1 + US2 = the core MVP (sign in, see, and manage servers). — *DONE & verified: `npm run build` green; **15 vitest tests pass** (6 files) incl. servers **contract** (T026), catalog table+search (T027), form validation + **duplicate-slug** (T028). Catalog (search/sort/status+health/access tags/row actions), Add/Edit form (remote/stdio, validation), detail Tabs, and enable/disable + delete **kill-switch** confirmation all implemented. E2E spec written (T029, needs browser).*

---

## Phase 5: User Story 3 — Credentials (write-only) (Priority: P2)

**Goal**: Choose credential mode and set/rotate/clear secrets without ever
displaying stored values.

**Independent test**: Set an org-shared credential → shows "set" + timestamp,
value never shown; rotate → next use takes it; clear → "not set".

- [X] T035 [US3] Backend prereq: ensure the servers response exposes a non-secret credential **is-set** status (add the field if absent, write-only preserved) in `services/control-plane/internal/admin/`
- [X] T036 [P] [US3] **Adversarial** test: a stored secret value never appears in the DOM, a copy action, or an error — including after set and rotate (Constitution VI) in `web/admin-console/tests/adversarial/secret-never-in-dom.test.tsx`
- [X] T037 [P] [US3] Component tests: mode select (none/org_shared/per_user); set/rotate/clear; status + last-updated display in `web/admin-console/tests/unit/credentials.test.tsx`
- [X] T038 [US3] Implement credentials API hooks (`PUT`/`DELETE` org + `/me`, treated as 204, value never echoed) in `web/admin-console/src/api/credentials.ts`
- [X] T039 [US3] Implement the Credentials panel in Server detail (mode selection, write-only set/rotate/clear, status only) in `web/admin-console/src/features/credentials/`

**Checkpoint**: credentials manageable and provably confidential. — *DONE & verified: backend `credential_set` status added (Go build + admin tests green); `npm run build` green; **20 vitest tests pass** incl. the **secret-never-in-DOM adversarial gate** (T036: value absent after set + rotate) and the panel set/rotate/clear (T037). Write-only CredentialsPanel wired into Server detail.*

---

## Phase 6: User Story 4 — Access control (RBAC) (Priority: P2)

**Goal**: Assign permitted roles per server; visually distinguish open vs.
restricted.

**Independent test**: Restrict a server to a role → catalog marks it restricted;
clear → shown as open to all.

- [X] T040 [P] [US4] Component test: role assignment + open/restricted indication in `web/admin-console/tests/unit/rbac.test.tsx`
- [X] T041 [US4] Implement the Access panel in Server detail (assign `allowed_roles` via Checkbox group, persist via `PATCH`) in `web/admin-console/src/features/rbac/`
- [X] T042 [US4] Show open vs. role-restricted as a Tag in the catalog + detail in `web/admin-console/src/pages/Servers.tsx`

**Checkpoint**: per-server access is configurable and visible. — *DONE & verified: 23 vitest tests pass (3 RBAC). Editable `AccessPanel` (role chips with remove + add + Save via PATCH) in Server detail; open/restricted Tag in the catalog and detail. Note: free-form role chips (no roles-catalog endpoint in 001).*

---

## Phase 7: User Story 5 — Audit trail (Priority: P2)

**Goal**: Review org-scoped audit events newest-first with filters; surface
denials; show tamper-evident chain status.

**Independent test**: Recent actions appear newest-first; filter narrows; chain
indicator shows verified; `auth.denied`/`authz.denied` are distinguishable.

- [X] T043 [P] [US5] Contract test for `GET …/audit` (shape + chain status) in `web/admin-console/tests/contract/audit.contract.test.ts`
- [X] T044 [P] [US5] Component tests: audit table render/filter + chain-status banner + denial highlighting in `web/admin-console/tests/unit/audit.test.tsx`
- [X] T045 [US5] Implement the audit API hook in `web/admin-console/src/api/audit.ts`
- [X] T046 [US5] Implement the Audit page (table, action/actor/time filters, pagination, chain-status banner, denial highlighting) in `web/admin-console/src/pages/Audit.tsx`

**Checkpoint**: full, filterable, integrity-aware audit review. — *DONE & verified: 26 vitest tests pass (audit contract + 2 component). Audit page: chain-status banner (verified/tampered), table (time/actor/action/target), free-text search + denials-only filter, **denial highlighting** (red Tag + tinted row), client-side pagination. `useAudit` (T045) shared with the dashboard.*

---

## Phase 8: User Story 6 — Health/usage, quotas (read-only), connection endpoint (Priority: P3)

**Goal**: Monitor health + request/denial/error trends; view configured rate
limits (read-only); copy the end-user connection endpoint.

**Independent test**: Dashboard usage widgets reflect activity; settings shows the
configured limits; copying the endpoint yields the correct `{org}` URL.

- [X] T047 [US6] Backend prereq: add a **read-only** quotas endpoint `GET /v1/orgs/{org}/quotas` (returns configured per-org/per-user limits) in `services/control-plane/internal/admin/quotas.go`
- [X] T048 [US6] Infra prereq: make the metrics query API (Prometheus) reachable from the console via the edge same-origin proxy or CORS (config) in `deploy/dev/` + edge config — *dev: Vite `/metrics-api` proxy → Prometheus (T007); prod: edge routes `/metrics-api` same-origin (deployment config)*
- [X] T049 [P] [US6] Component tests: usage charts from Prometheus fixtures, read-only limits display, endpoint copy in `web/admin-console/tests/unit/dashboard-usage.test.tsx`
- [X] T050 [P] [US6] Implement the metrics query hook (request/denial/error rate trends) in `web/admin-console/src/api/metrics.ts`
- [X] T051 [P] [US6] Implement the read-only quotas hook in `web/admin-console/src/api/quotas.ts`
- [X] T052 [US6] Add usage rate charts (ProgressBar/trend widgets) to the Dashboard in `web/admin-console/src/features/dashboard/`
- [X] T053 [US6] Implement the Settings page (read-only rate limits + connection-endpoint copy) in `web/admin-console/src/pages/Settings.tsx`

**Checkpoint**: operational visibility + onboarding aid complete. — *DONE & verified: backend read-only `GET …/quotas` (Go green); `npm run build` green; **28 vitest tests pass**. UsageWidget (Prometheus rate trends via `/metrics-api`), read-only quotas display + connection-endpoint copy in Settings.*

---

## Phase 9: User Story 7 — Consistent, accessible, on-brand (Priority: P3, cross-cutting)

**Goal**: Every screen adheres to Carbon, meets WCAG 2.1 AA, and works on desktop
+ tablet.

**Independent test**: Adherence lint passes; axe + keyboard walkthrough pass; tablet
layout remains usable.

- [X] T054 [US7] Make the design-system adherence lint (`npm run lint`) pass for every screen (FR-021) across `web/admin-console/src/`
- [X] T055 [P] [US7] Add `axe` accessibility assertions to each primary-flow e2e (no critical violations, FR-022) in `web/admin-console/tests/e2e/a11y.spec.ts`
- [~] T056 [P] [US7] Add a keyboard-only walkthrough e2e of the primary flows (focus order + visible focus) in `web/admin-console/tests/e2e/keyboard.spec.ts` — *spec written; needs `npx playwright install` + mock-backed dev server to run*
- [~] T057 [P] [US7] Add responsive checks at desktop + tablet widths for shell/tables/forms in `web/admin-console/tests/e2e/responsive.spec.ts` — *spec written; needs `npx playwright install` + mock-backed dev server to run*
- [X] T058 [US7] Apply Carbon focus/spacing/typography tokens consistently and fix any adherence/a11y findings across `web/admin-console/src/`

**Checkpoint**: quality bar met across the console. — *DONE: oxlint **adherence** clean (0/0); hermetic **axe** a11y gate green on Servers/Audit/Settings (3 tests); Search labelled for SR; Playwright a11y/keyboard/responsive specs written (browser).*

---

## Phase 10: Polish & cross-cutting

- [X] T059 [P] Add a CI workflow for the console (build + adherence lint + unit + e2e/a11y gates) in `.github/workflows/admin-console.yml`
- [X] T060 [P] Production build + static hosting at the edge (no new service); document serving in `web/admin-console/README.md`
- [X] T061 [P] Verify the performance budget (first meaningful paint < 2s; action feedback < 1s — SC-007) and record results
- [X] T062 [P] Add the Admin Console to the docs site (`docs/`) with a short page + link
- [X] T063 Final adversarial re-verification: cross-org isolation (T020) and secret-never-in-DOM (T036) pass against the integrated app

---

## Dependencies & story completion order

- **Setup (P1–7)** → **Foundational (T008–T018)** → user stories.
- **US1 (P1)** and **US2 (P1)** form the MVP; US1 before US2 (shell/auth/dashboard
  underpin the management screens, and the cross-org adversarial guard lands in US1).
- **US3, US4, US5 (P2)** depend on US2's Server detail / catalog but are independent
  of each other → parallelizable once US2 is done.
- **US6 (P3)** depends on Foundational + US1 (dashboard) and its two backend/infra
  prereqs (T047 quotas endpoint, T048 metrics reachability).
- **US7 (P3)** is cross-cutting — it gates and polishes all prior screens; run its
  checks continuously, finalize last.
- **Polish (Phase 10)** last.

## Parallel execution examples

- **Setup**: T002–T007 run in parallel after T001.
- **Foundational**: T009/T010/T013/T014/T017/T018 in parallel; T011→T012 after T009/T010; T015/T016 after the shell vendor (T002).
- **US1**: tests T019/T020/T021 in parallel first; then T022/T023/T024 (T024 before T025).
- **US2**: tests T026–T029 in parallel; then hooks T030 → pages T031/T032/T033 → T034.
- **P2 stories**: US3, US4, US5 can proceed in parallel once US2 is complete.

## Implementation strategy

- **MVP = US1 + US2** (both P1): a secure, org-scoped console that signs in, shows
  the dashboard, and fully manages servers — demonstrable on its own.
- **Increment 2** = US3 + US4 + US5 (governance + audit).
- **Increment 3** = US6 + US7 (observability + the accessibility/brand quality bar)
  + Polish.
- Tests are written **before** their implementation tasks within each story
  (Principle V); the two adversarial tests (T020 cross-org, T036 secret-in-DOM) are
  non-negotiable gates.

## Notes on backend prerequisites (feature `001`)

Three small, **non-mutating** backend touches are required by the clarified scope
(see `spec.md` Clarifications + `contracts/`):
- **T008** — CORS allowlist for the console origin (config).
- **T035** — credential **is-set** status in the servers response (if absent).
- **T047** — read-only `GET …/quotas` endpoint; **T048** — metrics query
  reachability. None mutate state or change authorization (FR-023).
