// Per-tenant setup: register the stdio AWS server, set write-only credentials,
// create the tenant's own bucket (contracts/tenant-aws-setup.md). Idempotent.
import { CONFIG, TenantCfg } from "./config.js";
import { http } from "./http.js";
import { adminToken } from "./tokens.js";
import { ensureRealmUser } from "./tenants.js";
import { createBucket } from "./aws.js";

export interface TenantState {
  t: TenantCfg;
  serverId: string;
  adminTok: string;
}

export async function setupTenant(t: TenantCfg): Promise<TenantState> {
  await ensureRealmUser(t);
  const adminTok = await adminToken(t);

  // register (or reuse) the AWS stdio server
  const list = await http("GET", `${CONFIG.controlPlane}/v1/orgs/${t.slug}/servers`, { token: adminTok });
  const existing = (Array.isArray(list.json) ? list.json : []).find((s: any) => s.slug === "aws");
  let serverId: string | undefined = existing?.id;
  if (!serverId) {
    const create = await http("POST", `${CONFIG.controlPlane}/v1/orgs/${t.slug}/servers`, {
      token: adminTok,
      body: {
        slug: "aws",
        type: "stdio",
        command: CONFIG.awsServerCommand,
        args: [],
        env: {
          AWS_ENDPOINT_URL: CONFIG.awsEndpointInternal,
          AWS_REGION: CONFIG.region,
          AWS_API_MCP_WORKING_DIR: "/tmp",
        },
        credential_mode: "org",
        allowed_roles: [],
      },
    });
    if (![200, 201].includes(create.status)) {
      throw new Error(`register server ${t.slug} failed: ${create.status} ${create.text}`);
    }
    serverId = create.json.id;
  }

  // write-only AWS credentials (FR-006) — the tenant's account keys
  const cred = await http("PUT", `${CONFIG.controlPlane}/v1/orgs/${t.slug}/servers/${serverId}/credentials`, {
    token: adminTok,
    body: { AWS_ACCESS_KEY_ID: t.accountId, AWS_SECRET_ACCESS_KEY: t.secretAccessKey },
  });
  if (![200, 204].includes(cred.status)) {
    throw new Error(`set credentials ${t.slug} failed: ${cred.status} ${cred.text}`);
  }

  // bucket under this tenant's own account
  await createBucket(t);

  return { t, serverId: serverId!, adminTok };
}
