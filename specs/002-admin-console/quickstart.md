# Quickstart — Admin Console UI

How to set up, run, and test the console locally against the dev stack.

## Prerequisites

- **Node 20 LTS** (+ npm).
- The dev backend running (control-plane `:8090`, Keycloak `:8081`) — bring it up
  with the **`/dev-setup`** skill or `docker compose -f deploy/dev/compose.yaml up -d`.
- A Keycloak realm for your dev org (e.g. `acme`) with an **admin-role** user.
- The **Carbon handoff** (`Carbon Design System-handoff.zip` at the repo root).

## One-time: scaffold + vendor the design system

```sh
# from repo root
mkdir -p web/admin-console
# scaffold a Vite React+TS app in web/admin-console (npm create vite@latest …)
# then vendor the design system from the handoff:
unzip -o "Carbon Design System-handoff.zip" -d /tmp/cds
cp -R /tmp/cds/carbon-design-system/project/components  web/admin-console/src/design-system/components
cp -R /tmp/cds/carbon-design-system/project/tokens      web/admin-console/src/design-system/tokens
cp -R /tmp/cds/carbon-design-system/project/assets/icons web/admin-console/src/design-system/icons
cp -R /tmp/cds/carbon-design-system/project/ui_kits/cloud-console web/admin-console/src/design-system/shell
cp /tmp/cds/carbon-design-system/project/_adherence.oxlintrc.json web/admin-console/.oxlintrc.json
```

Load the global token CSS in the order given by the handoff
`_ds_manifest.json.globalCssPaths` (fonts → colors → typography → spacing → motion
→ base → styles).

## Configure

`web/admin-console/.env.local`:

```sh
VITE_BASE_DOMAIN=mcp.example.com
VITE_API_BASE=http://localhost:8090          # dev control-plane
VITE_OIDC_ISSUER_TEMPLATE=http://localhost:8081/realms/%s   # %s = org
VITE_OIDC_CLIENT_ID=mcp-admin-console
VITE_DEV_ORG=acme                              # dev convenience (prod: from host)
```

In dev, the Vite dev server **proxies** `/v1/*` to `VITE_API_BASE` so the browser
stays same-origin (no CORS needed locally). In prod, either serve same-origin
behind the edge or enable CORS for the console origin on the control-plane
(see `contracts/control-plane-consumed.md`).

## Run

```sh
cd web/admin-console
npm install
npm run dev            # http://localhost:5173
```

Sign in with your dev org's admin user → you should land on the Dashboard scoped
to that org.

## Test & quality gates

```sh
npm run test           # Vitest + React Testing Library (unit/component)
npm run test:e2e       # Playwright primary flows + axe (WCAG 2.1 AA)
npm run lint           # oxlint with the handoff adherence config (FR-021)
```

Adversarial tests (must exist and pass, Constitution V):
- an org-A session renders **zero** org-B data;
- a stored secret value never appears in the DOM / a copy action / an error,
  including after set and rotate.

## Build

```sh
npm run build          # static assets in dist/ — served at the edge (no new service)
```

## Smoke checklist (maps to spec)

- Sign in as admin → Dashboard shows this org's counts/health (US1).
- Add a remote and a stdio server → both appear in the catalog (US2).
- Set an org-shared credential → shows "set"; the value is never displayed (US3).
- Restrict a server to a role → catalog marks it restricted (US4).
- Open Audit → recent actions appear newest-first; chain status shows verified (US5).
- Copy the connection endpoint from Settings → correct `{org}` URL (US6).
- Run `npm run lint` + `npm run test:e2e` → adherence + a11y pass (US7).
