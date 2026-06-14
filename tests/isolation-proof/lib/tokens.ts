// Headless OAuth tokens per realm (research.md D6). No interactive login.
import { CONFIG, TenantCfg } from "./config.js";

async function oidcToken(realm: string, params: Record<string, string>): Promise<string> {
  const url = `${CONFIG.keycloak}/realms/${realm}/protocol/openid-connect/token`;
  const res = await fetch(url, {
    method: "POST",
    headers: { "content-type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams(params).toString(),
  });
  const text = await res.text();
  if (!res.ok) throw new Error(`token grant failed (realm=${realm}): ${res.status} ${text}`);
  const j = JSON.parse(text);
  if (!j.access_token) throw new Error(`no access_token (realm=${realm})`);
  return j.access_token as string;
}

export function operatorToken(): Promise<string> {
  return oidcToken(CONFIG.platformRealm, {
    grant_type: "password",
    client_id: CONFIG.operator.client,
    username: CONFIG.operator.user,
    password: CONFIG.operator.password,
    scope: "openid",
  });
}

/** Data-plane user token (audience = the tenant's MCP resource) via mcp-client. */
export function userToken(t: TenantCfg): Promise<string> {
  return oidcToken(t.slug, {
    grant_type: "password",
    client_id: CONFIG.dataPlaneClient,
    username: t.user,
    password: t.password,
    scope: "openid",
  });
}

/** Admin-API token (audience = MCP_ADMIN_AUDIENCE) via the console client. */
export function adminToken(t: TenantCfg): Promise<string> {
  return oidcToken(t.slug, {
    grant_type: "password",
    client_id: CONFIG.adminClient,
    username: t.user,
    password: t.password,
    scope: "openid",
  });
}

/** Keycloak Admin API token (provisioner service account, or admin-cli fallback). */
export function kcAdminToken(): Promise<string> {
  if (CONFIG.kcAdmin.clientSecret) {
    return oidcToken(CONFIG.kcAdmin.realm, {
      grant_type: "client_credentials",
      client_id: CONFIG.kcAdmin.clientId,
      client_secret: CONFIG.kcAdmin.clientSecret,
    });
  }
  return oidcToken(CONFIG.kcAdmin.realm, {
    grant_type: "password",
    client_id: "admin-cli",
    username: CONFIG.kcAdmin.fallbackUser,
    password: CONFIG.kcAdmin.fallbackPassword,
  });
}
