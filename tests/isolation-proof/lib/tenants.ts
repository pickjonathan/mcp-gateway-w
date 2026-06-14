// Tenant lifecycle via the platform API (003) + Keycloak Admin API helpers to
// give the harness a known-password, admin-roled user it can authenticate as.
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

export async function tenantExists(opToken: string, slug: string): Promise<boolean> {
  const g = await http("GET", `${CONFIG.controlPlane}/v1/platform/tenants/${slug}`, { token: opToken });
  return g.status === 200;
}

const kcAdmin = (path: string) => `${CONFIG.keycloak}/admin/realms${path}`;

/** Set a known password + admin role on the tenant's user, and enable direct
 *  grants on the console client so the harness can password-grant admin tokens. */
export async function ensureRealmUser(t: TenantCfg): Promise<void> {
  const tok = await kcAdminToken();
  const q = await http("GET", kcAdmin(`/${t.slug}/users?max=500`), { token: tok });
  const users: any[] = Array.isArray(q.json) ? q.json : [];
  const u = users.find((x) => x.username === t.user || x.email === t.user || x.email === t.adminEmail);
  if (!u) throw new Error(`no user matching '${t.user}' in realm '${t.slug}' (status ${q.status})`);

  await http("PUT", kcAdmin(`/${t.slug}/users/${u.id}/reset-password`), {
    token: tok,
    body: { type: "password", value: t.password, temporary: false },
  });
  await http("PUT", kcAdmin(`/${t.slug}/users/${u.id}`), { token: tok, body: { ...u, enabled: true } });

  const role = await http("GET", kcAdmin(`/${t.slug}/roles/admin`), { token: tok });
  if (role.json?.id) {
    await http("POST", kcAdmin(`/${t.slug}/users/${u.id}/role-mappings/realm`), {
      token: tok,
      body: [{ id: role.json.id, name: "admin" }],
    });
  }
  await enableDirectGrants(tok, t.slug, CONFIG.adminClient);
  await enableDirectGrants(tok, t.slug, CONFIG.dataPlaneClient);
}

async function enableDirectGrants(tok: string, slug: string, clientId: string): Promise<void> {
  const c = await http("GET", kcAdmin(`/${slug}/clients?clientId=${encodeURIComponent(clientId)}`), { token: tok });
  const client = Array.isArray(c.json) ? c.json[0] : undefined;
  if (client?.id && !client.directAccessGrantsEnabled) {
    await http("PUT", kcAdmin(`/${slug}/clients/${client.id}`), {
      token: tok,
      body: { ...client, directAccessGrantsEnabled: true },
    });
  }
}
