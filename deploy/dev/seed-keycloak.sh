#!/usr/bin/env bash
#
# Seed the dev Keycloak with the realm/client/user the admin console needs.
#
# DEV ONLY. This intentionally relaxes security for local use:
#   - the realm is set to sslRequired=NONE so the browser's HTTP OIDC requests
#     work against http://localhost (Keycloak otherwise rejects external HTTP);
#   - it creates a weak admin/admin user.
# Never point this at a non-dev realm.
#
# It is idempotent: re-running updates settings in place (safe after a restart).
# kcadm runs *inside* the container because Keycloak's master realm refuses
# token requests over plain HTTP from the host ("HTTPS required").
#
# Usage:
#   bash deploy/dev/seed-keycloak.sh         # or: make seed-keycloak
# Override any value via env, e.g. CONSOLE_ORIGIN=http://localhost:4173 bash ...
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE="$SCRIPT_DIR/compose.yaml"

# Config — defaults match web/admin-console/.env.local.
: "${REALM:=acme}"
: "${CLIENT_ID:=mcp-admin-console}"
: "${CONSOLE_ORIGIN:=http://localhost:5173}"
: "${ADMIN_AUDIENCE:=https://api.mcp.example.com}"
: "${MCP_CLIENT_ID:=mcp-client}"           # client for MCP clients hitting the gateway /mcp
: "${BASE_DOMAIN:=mcp.example.com}"         # gateway MCP_BASE_DOMAIN → resource https://{realm}.{base}/mcp
: "${ADMIN_USER:=admin}"
: "${ADMIN_PW:=admin}"
: "${ACCESS_TTL:=900}"      # access-token lifespan (seconds) — 15m
: "${SSO_IDLE:=28800}"      # SSO/refresh-token idle (seconds) — 8h
: "${SSO_MAX:=86400}"       # SSO session max (seconds) — 24h
: "${KC_ADMIN:=admin}"
: "${KC_ADMIN_PW:=admin}"

echo "Seeding Keycloak (realm=$REALM, client=$CLIENT_ID) — DEV ONLY…"

docker compose -f "$COMPOSE" exec -T \
  -e REALM="$REALM" -e CLIENT_ID="$CLIENT_ID" -e CONSOLE_ORIGIN="$CONSOLE_ORIGIN" \
  -e ADMIN_AUDIENCE="$ADMIN_AUDIENCE" -e ADMIN_USER="$ADMIN_USER" -e ADMIN_PW="$ADMIN_PW" \
  -e MCP_CLIENT_ID="$MCP_CLIENT_ID" -e BASE_DOMAIN="$BASE_DOMAIN" \
  -e ACCESS_TTL="$ACCESS_TTL" -e SSO_IDLE="$SSO_IDLE" -e SSO_MAX="$SSO_MAX" \
  -e KC_ADMIN="$KC_ADMIN" -e KC_ADMIN_PW="$KC_ADMIN_PW" \
  keycloak bash -s <<'KCADM'
set -euo pipefail
K=/opt/keycloak/bin/kcadm.sh

# Wait for Keycloak to accept admin credentials (start-dev can take a moment).
ready=
for _ in $(seq 1 60); do
  if $K config credentials --server http://localhost:8080 --realm master \
       --user "$KC_ADMIN" --password "$KC_ADMIN_PW" >/dev/null 2>&1; then
    ready=1; break
  fi
  sleep 2
done
[ -n "$ready" ] || { echo "Keycloak not ready / auth failed after 120s"; exit 1; }

REALM_SETTINGS="-s enabled=true -s sslRequired=NONE \
  -s accessTokenLifespan=$ACCESS_TTL \
  -s ssoSessionIdleTimeout=$SSO_IDLE -s ssoSessionMaxLifespan=$SSO_MAX \
  -s clientSessionIdleTimeout=$SSO_IDLE -s clientSessionMaxLifespan=$SSO_MAX"

# --- Realm (create or update settings in place) ---
if $K get "realms/$REALM" >/dev/null 2>&1; then
  $K update "realms/$REALM" $REALM_SETTINGS
  echo "  realm $REALM: updated"
else
  $K create realms -s "realm=$REALM" $REALM_SETTINGS
  echo "  realm $REALM: created"
fi

# --- Realm role 'admin' (reaches the token via realm_access.roles) ---
$K get "roles/admin" -r "$REALM" >/dev/null 2>&1 || $K create roles -r "$REALM" -s name=admin
echo "  role admin: ok"

# --- Public PKCE client ---
CLIENT_ATTRS="-s \"redirectUris=[\\\"$CONSOLE_ORIGIN/*\\\"]\" -s \"webOrigins=[\\\"$CONSOLE_ORIGIN\\\"]\""
CID=$($K get clients -r "$REALM" -q "clientId=$CLIENT_ID" --fields id --format csv --noquotes 2>/dev/null | tr -d '\r')
if [ -z "$CID" ]; then
  $K create clients -r "$REALM" \
    -s "clientId=$CLIENT_ID" -s 'name=MCP Admin Console' \
    -s publicClient=true -s standardFlowEnabled=true -s directAccessGrantsEnabled=false \
    -s "redirectUris=[\"$CONSOLE_ORIGIN/*\"]" -s "webOrigins=[\"$CONSOLE_ORIGIN\"]" \
    -s "attributes={\"pkce.code.challenge.method\":\"S256\",\"post.logout.redirect.uris\":\"$CONSOLE_ORIGIN/*\"}"
  CID=$($K get clients -r "$REALM" -q "clientId=$CLIENT_ID" --fields id --format csv --noquotes | tr -d '\r')
  echo "  client $CLIENT_ID: created ($CID)"
else
  $K update "clients/$CID" -r "$REALM" \
    -s "redirectUris=[\"$CONSOLE_ORIGIN/*\"]" -s "webOrigins=[\"$CONSOLE_ORIGIN\"]" \
    -s "attributes={\"pkce.code.challenge.method\":\"S256\",\"post.logout.redirect.uris\":\"$CONSOLE_ORIGIN/*\"}"
  echo "  client $CLIENT_ID: updated ($CID)"
fi

# --- Protocol mappers (idempotent by name) ---
MAPPERS=$($K get "clients/$CID/protocol-mappers/models" -r "$REALM" --fields name --format csv --noquotes 2>/dev/null | tr -d '\r')
case "$MAPPERS" in
  *admin-api-audience*) : ;;
  *) $K create "clients/$CID/protocol-mappers/models" -r "$REALM" \
       -s name=admin-api-audience -s protocol=openid-connect -s protocolMapper=oidc-audience-mapper \
       -s "config.\"included.custom.audience\"=$ADMIN_AUDIENCE" \
       -s 'config."access.token.claim"=true' -s 'config."id.token.claim"=false'
     echo "  mapper admin-api-audience: created" ;;
esac
case "$MAPPERS" in
  *realm-roles-id*) : ;;
  *) $K create "clients/$CID/protocol-mappers/models" -r "$REALM" \
       -s name=realm-roles-id -s protocol=openid-connect -s protocolMapper=oidc-usermodel-realm-role-mapper \
       -s 'config."claim.name"=realm_access.roles' -s 'config."jsonType.label"=String' -s 'config.multivalued=true' \
       -s 'config."id.token.claim"=true' -s 'config."access.token.claim"=true' -s 'config."userinfo.token.claim"=true'
     echo "  mapper realm-roles-id: created" ;;
esac
echo "  mappers: ok"

# --- Public MCP client (for MCP clients — Inspector, mcp-remote, etc. — that
#     connect to the gateway /mcp). The gateway requires the token audience to
#     equal the org's MCP resource URL; an audience mapper supplies it. Direct
#     grants are enabled for dev so a token can be minted from a script/curl. ---
MCP_RESOURCE="https://$REALM.$BASE_DOMAIN/mcp"
MCID=$($K get clients -r "$REALM" -q "clientId=$MCP_CLIENT_ID" --fields id --format csv --noquotes 2>/dev/null | tr -d '\r')
if [ -z "$MCID" ]; then
  $K create clients -r "$REALM" \
    -s "clientId=$MCP_CLIENT_ID" -s 'name=MCP Client' \
    -s publicClient=true -s standardFlowEnabled=true -s directAccessGrantsEnabled=true \
    -s 'redirectUris=["http://localhost:*","http://127.0.0.1:*"]' -s 'webOrigins=["+"]' \
    -s 'attributes={"pkce.code.challenge.method":"S256"}'
  MCID=$($K get clients -r "$REALM" -q "clientId=$MCP_CLIENT_ID" --fields id --format csv --noquotes | tr -d '\r')
  echo "  client $MCP_CLIENT_ID: created ($MCID)"
else
  $K update "clients/$MCID" -r "$REALM" \
    -s publicClient=true -s standardFlowEnabled=true -s directAccessGrantsEnabled=true \
    -s 'redirectUris=["http://localhost:*","http://127.0.0.1:*"]' -s 'webOrigins=["+"]' \
    -s 'attributes={"pkce.code.challenge.method":"S256"}'
  echo "  client $MCP_CLIENT_ID: updated ($MCID)"
fi
MCP_MAPPERS=$($K get "clients/$MCID/protocol-mappers/models" -r "$REALM" --fields name --format csv --noquotes 2>/dev/null | tr -d '\r')
case "$MCP_MAPPERS" in
  *mcp-audience*) : ;;
  *) $K create "clients/$MCID/protocol-mappers/models" -r "$REALM" \
       -s name=mcp-audience -s protocol=openid-connect -s protocolMapper=oidc-audience-mapper \
       -s "config.\"included.custom.audience\"=$MCP_RESOURCE" \
       -s 'config."access.token.claim"=true' -s 'config."id.token.claim"=false'
     echo "  mapper mcp-audience: created" ;;
esac
echo "  client $MCP_CLIENT_ID: ok (audience=$MCP_RESOURCE)"

# --- Admin user (USER_ID — bash reserves UID) ---
USER_ID=$($K get users -r "$REALM" -q "username=$ADMIN_USER" --fields id --format csv --noquotes 2>/dev/null | head -1 | tr -d '\r')
if [ -z "$USER_ID" ]; then
  $K create users -r "$REALM" -s "username=$ADMIN_USER" -s enabled=true \
    -s "email=$ADMIN_USER@$REALM.test" -s emailVerified=true -s firstName=Acme -s lastName=Admin
  echo "  user $ADMIN_USER: created"
else
  echo "  user $ADMIN_USER: exists"
fi
$K set-password -r "$REALM" --username "$ADMIN_USER" --new-password "$ADMIN_PW"
$K add-roles -r "$REALM" --uusername "$ADMIN_USER" --rolename admin 2>/dev/null || true
echo "  user $ADMIN_USER: password set + admin role assigned"

echo "✅ Keycloak seeded: realm=$REALM, login=$ADMIN_USER/$ADMIN_PW, console=$CLIENT_ID, mcp=$MCP_CLIENT_ID, TTL=${ACCESS_TTL}s"
KCADM