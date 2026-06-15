// Tenant lifecycle via the platform API (003) + Keycloak Admin API helpers.
// Two identities per realm: the provisioned ADMIN (control-plane ops) and a
// DEDICATED data-plane USER with a role that permits the realm's AWS MCP (step 5/6).
import { CONFIG, TenantCfg } from "./config.js";
import { http, sleep } from "./http.js";
import { kcAdminToken } from "./tokens.js";

export async function provisionTenant(opToken: string, t: TenantCfg): Promise<void> {
  const r = await http("POST", `${CONFIG.controlPlane}/v1/platform/tenants`, {
    token: opToken,
    body: { slug: t.slug, display_name: t.displayName, admin_email: t.adminEmail },
  });
  if (![201, 202, 409].includes(r.status)) {
    throw new Error(`provision ${t.slug} failed: ${r.status} ${r.text}`);
  }
  for (let i = 0; i < 60; i++) {
    const g = await http("GET", `${CONFIG.controlPlane}/v1/platform/tenants/${t.slug}`, { token: opToken });
    const status = g.json?.tenant?.status ?? g.json?.status;
    if (status === "active") return;
    if (status === "failed") throw new Error(`tenant ${t.slug} provisioning failed: ${g.text}`);
    await sleep(2000);
  }
  throw new Error(`tenant ${t.slug} did not reach active in time`);
}

export async function deleteTenant(opToken: string, slug: string): Promise<void> {
  await http("DELETE", `${CONFIG.controlPlane}/v1/platform/tenants/${slug}`, { token: opToken });
}

const kcAdmin = (path: string) => `${CONFIG.keycloak}/admin/realms${path}`;

// Make a user loginable via password grant: enabled, verified, no required actions,
// profile complete (else VERIFY_PROFILE → "Account is not fully set up"), known password.
async function prepUser(tok: string, slug: string, u: any, password: string, displayName: string): Promise<void> {
  const upd = await http("PUT", kcAdmin(`/${slug}/users/${u.id}`), {
    token: tok,
    body: { enabled: true, emailVerified: true, requiredActions: [], firstName: u.firstName || "Proof", lastName: u.lastName || displayName },
  });
  if (upd.status >= 400) throw new Error(`prepare ${slug}/${u.username} failed: ${upd.status} ${upd.text}`);
  const pw = await http("PUT", kcAdmin(`/${slug}/users/${u.id}/reset-password`), {
    token: tok,
    body: { type: "password", value: password, temporary: false },
  });
  if (pw.status >= 400) throw new Error(`reset-password ${slug}/${u.username} failed: ${pw.status} ${pw.text}`);
}

/** Prepare the provisioned admin user for control-plane password grants. */
export async function ensureAdminUser(t: TenantCfg): Promise<void> {
  const tok = await kcAdminToken();
  const q = await http("GET", kcAdmin(`/${t.slug}/users?max=500`), { token: tok });
  const users: any[] = Array.isArray(q.json) ? q.json : [];
  const u = users.find((x) => x.email === t.adminEmail || x.username === t.adminEmail);
  if (!u) throw new Error(`admin user '${t.adminEmail}' not found in realm '${t.slug}'`);
  await prepUser(tok, t.slug, u, t.adminPassword, t.displayName);
  await enableDirectGrants(tok, t.slug, CONFIG.adminClient);
  await enableDirectGrants(tok, t.slug, CONFIG.dataPlaneClient);
}

/** Create the realm role + a dedicated user holding it (permission to use the AWS MCP). */
export async function ensureRoleUser(t: TenantCfg): Promise<void> {
  const tok = await kcAdminToken();
  // realm role
  let role = await http("GET", kcAdmin(`/${t.slug}/roles/${encodeURIComponent(t.role)}`), { token: tok });
  if (role.status === 404) {
    await http("POST", kcAdmin(`/${t.slug}/roles`), { token: tok, body: { name: t.role, description: "Permission to use the AWS MCP (004 proof)" } });
    role = await http("GET", kcAdmin(`/${t.slug}/roles/${encodeURIComponent(t.role)}`), { token: tok });
  }
  // user
  const find = async () => {
    const q = await http("GET", kcAdmin(`/${t.slug}/users?username=${encodeURIComponent(t.user)}`), { token: tok });
    return (Array.isArray(q.json) ? q.json : []).find((x: any) => x.username === t.user);
  };
  let u = await find();
  if (!u) {
    const cr = await http("POST", kcAdmin(`/${t.slug}/users`), {
      token: tok,
      body: { username: t.user, email: `${t.user}@${t.slug}.example`, firstName: "Proof", lastName: t.displayName, enabled: true, emailVerified: true },
    });
    if (cr.status >= 400 && cr.status !== 409) throw new Error(`create user ${t.user} failed: ${cr.status} ${cr.text}`);
    u = await find();
  }
  if (!u) throw new Error(`role user '${t.user}' not found in realm '${t.slug}' after create`);
  await prepUser(tok, t.slug, u, t.password, t.displayName);
  if (role.json?.id) {
    await http("POST", kcAdmin(`/${t.slug}/users/${u.id}/role-mappings/realm`), { token: tok, body: [{ id: role.json.id, name: t.role }] });
  }
}

/** Point the mcp-client's audience mapper at the audience THIS gateway expects. */
export async function ensureMcpClientAudience(t: TenantCfg, audience: string): Promise<void> {
  const tok = await kcAdminToken();
  const c = await http("GET", kcAdmin(`/${t.slug}/clients?clientId=${encodeURIComponent(CONFIG.dataPlaneClient)}`), { token: tok });
  const client = Array.isArray(c.json) ? c.json[0] : undefined;
  if (!client?.id) throw new Error(`mcp-client not found in realm ${t.slug}`);
  const mm = await http("GET", kcAdmin(`/${t.slug}/clients/${client.id}/protocol-mappers/models`), { token: tok });
  const list: any[] = Array.isArray(mm.json) ? mm.json : [];
  const aud = list.find((m) => m.protocolMapper === "oidc-audience-mapper");
  if (aud) {
    aud.config = { ...aud.config, "included.custom.audience": audience };
    const r = await http("PUT", kcAdmin(`/${t.slug}/clients/${client.id}/protocol-mappers/models/${aud.id}`), { token: tok, body: aud });
    if (r.status >= 400) throw new Error(`update mcp-client audience (${t.slug}) failed: ${r.status} ${r.text}`);
  } else {
    const r = await http("POST", kcAdmin(`/${t.slug}/clients/${client.id}/protocol-mappers/models`), {
      token: tok,
      body: {
        name: "mcp-audience",
        protocol: "openid-connect",
        protocolMapper: "oidc-audience-mapper",
        config: { "included.custom.audience": audience, "access.token.claim": "true", "id.token.claim": "false" },
      },
    });
    if (r.status >= 400) throw new Error(`create mcp-client audience (${t.slug}) failed: ${r.status} ${r.text}`);
  }
}

async function enableDirectGrants(tok: string, slug: string, clientId: string): Promise<void> {
  const c = await http("GET", kcAdmin(`/${slug}/clients?clientId=${encodeURIComponent(clientId)}`), { token: tok });
  const client = Array.isArray(c.json) ? c.json[0] : undefined;
  if (client?.id && !client.directAccessGrantsEnabled) {
    await http("PUT", kcAdmin(`/${slug}/clients/${client.id}`), { token: tok, body: { ...client, directAccessGrantsEnabled: true } });
  }
}
