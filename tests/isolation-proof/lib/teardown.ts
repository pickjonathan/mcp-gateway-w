// Teardown + clean post-check (SC-008).
import { CONFIG, TenantCfg } from "./config.js";
import { http } from "./http.js";
import { deleteTenant } from "./tenants.js";
import { deleteBucketBestEffort } from "./aws.js";
import { TenantState } from "./setup.js";

const GONE = new Set(["deleted", "deleting", "offboarding", undefined, null]);

export async function teardownTenant(opToken: string, st: TenantState): Promise<void> {
  const { t, adminTok, serverId } = st;
  if (adminTok && serverId) {
    await http("DELETE", `${CONFIG.controlPlane}/v1/orgs/${t.slug}/servers/${serverId}/credentials`, { token: adminTok });
    await http("DELETE", `${CONFIG.controlPlane}/v1/orgs/${t.slug}/servers/${serverId}`, { token: adminTok });
  }
  await deleteBucketBestEffort(t);
  await deleteTenant(opToken, t.slug);
}

/** SC-008: verify none of the feature-created tenants linger as active. */
export async function verifyClean(opToken: string, tenants: TenantCfg[]): Promise<{ clean: boolean; detail: string }> {
  const residual: string[] = [];
  for (const t of tenants) {
    const g = await http("GET", `${CONFIG.controlPlane}/v1/platform/tenants/${t.slug}`, { token: opToken });
    if (g.status === 404) continue;
    const status = g.json?.tenant?.status ?? g.json?.status;
    if (!GONE.has(status)) residual.push(`${t.slug}(${status})`);
  }
  return {
    clean: residual.length === 0,
    detail: residual.length ? `residual: ${residual.join(", ")}` : "no residual tenants/servers/creds/buckets",
  };
}
