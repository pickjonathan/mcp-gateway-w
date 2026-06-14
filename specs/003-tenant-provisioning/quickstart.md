# Quickstart: Tenant Provisioning (local) — and the isolation acceptance gate

Provisions a second tenant `globex` next to the seeded `acme`, populates it three ways,
and **proves cross-tenant isolation** — the walkthrough that gates a release (Constitution:
"the `quickstart.md` isolation walkthrough is the acceptance gate"). Run against the
`deploy/dev` stack.

## 0. One-time platform setup
```sh
make dev-up                      # postgres/redis/vault/keycloak/minio/...
# Seed the PLATFORM realm + the control plane's privileged Keycloak service account:
bash deploy/dev/seed-keycloak.sh                 # existing: seeds acme
PLATFORM=1 bash deploy/dev/seed-keycloak.sh      # NEW: seeds _platform realm + platform-admin + the
                                                 # control-plane service-account client (secret → Vault)
make run-control-plane           # :8090  (now serves /v1/platform/* and the SCIM bridge)
make run-gateway                 # :8080
```
Add a hosts entry for the new tenant (wildcard DNS stand-in): `127.0.0.1 globex.mcp.example.com`.

## 1. Provision `globex` (US1)
```sh
OP=$(curl -s $KC/_platform/protocol/openid-connect/token -d grant_type=password \
  -d client_id=mcp-platform -d username=operator -d password=operator -d scope=openid \
  | jq -r .access_token)
curl -s -X POST http://localhost:8090/v1/platform/tenants \
  -H "Authorization: Bearer $OP" -H 'Content-Type: application/json' \
  -d '{"slug":"globex","display_name":"Globex","admin_email":"ops@globex.example"}'
# → 202 {tenant:{status:provisioning}, job:{...}}; poll GET /v1/platform/tenants/globex/jobs/{id}
# until status=succeeded (< 5 min, SC-001). Tenant → active.
```
**Verify usable**: the `globex` admin can sign in at `globex.mcp.example.com` console; a
`globex` token (realm `globex`, aud `globex.mcp.example.com:8080/mcp`) is accepted at the gateway.

## 2. Prove isolation (the gate — HC-1, adversarial)
```sh
ACME=$(mint acme alice);  GLOBEX=$(mint globex its-admin)
# acme token on globex host → REJECTED (issuer/audience mismatch):
curl -s -o /dev/null -w '%{http_code}\n' http://globex.mcp.example.com:8080/mcp \
  -H "Authorization: Bearer $ACME" -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'   # ⇒ 401
# globex operator cannot be impersonated by an org token:
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8090/v1/platform/tenants \
  -H "Authorization: Bearer $ACME"                                                        # ⇒ 403
```
Expected: **401** (cross-tenant data plane) and **403** (org token on platform API). `acme`
sees none of `globex` and vice-versa (SC-002).

## 3. Invite a user (US2)
```sh
GA=$(mint globex its-admin)
curl -s -X POST http://localhost:8090/v1/orgs/globex/invitations -H "Authorization: Bearer $GA" \
  -d '{"email":"dana@globex.example","roles":["member"]}'      # 201; accept token emailed
# Accept (public): POST /v1/invitations:accept {token,password} → Dana exists in globex realm only.
```

## 4. Enterprise SSO + SCIM (US4)
```sh
# Brokering: point globex at a test OIDC IdP; first login JIT-provisions with mapped roles.
curl -s -X PUT http://localhost:8090/v1/orgs/globex/identity-providers/corp -H "Authorization: Bearer $GA" \
  -d '{"type":"oidc","config":{...},"secret":"...","role_mappings":{"groups.eng":"aws-users"}}'
# SCIM: enable directory sync, get the per-tenant bearer (shown ONCE):
SCIM=$(curl -s -X PUT http://localhost:8090/v1/orgs/globex/directory-sync -H "Authorization: Bearer $GA" \
  -d '{"group_role_mappings":{"Engineering":"aws-users"}}')
BEARER=$(echo "$SCIM" | jq -r .bearer);  URL=$(echo "$SCIM" | jq -r .scim_base_url)
# Push a user, then deactivate — access must drop by next token (SC-005):
curl -s -X POST $URL/Users -H "Authorization: Bearer $BEARER" -d '{"userName":"erin@globex.example","active":true,...}'
curl -s -X PATCH $URL/Users/{id} -H "Authorization: Bearer $BEARER" -d '{"Operations":[{"op":"replace","path":"active","value":false}]}'
# Within ≤15 min, erin's gateway token is rejected.
```

## 5. Lifecycle (US3)
```sh
curl -s -X POST http://localhost:8090/v1/platform/tenants/globex:suspend -H "Authorization: Bearer $OP"
#  → new/refreshed globex tokens rejected within <1 min (SC-006); acme unaffected. Then :resume restores.
# Delete a throwaway tenant → realm/clients/credentials gone, slug reusable; WORM audit retained ≥1y (SC-007).
```

## Acceptance checklist (release gate)
- [ ] `globex` provisioned end-to-end < 5 min; admin signs in; gateway accepts its tokens (SC-001).
- [ ] Cross-tenant: 401 (data plane) + 403 (platform API); zero cross-tenant visibility (SC-002).
- [ ] Mid-saga failure leaves no ghost realm; re-run is idempotent (SC-003, SC-008).
- [ ] Invite → active < 3 min (SC-004); brokered + SCIM users provision; deactivation drops access ≤15 min (SC-005).
- [ ] Suspend rejects new tokens < 1 min, reversible (SC-006); delete leaves no residue, audit retained (SC-007).
- [ ] Privileged Keycloak credential + SCIM bearers never appear in any API response, log, or trace.
