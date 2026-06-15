// Per-tenant setup: register the stdio AWS server, set write-only credentials,
// create the tenant's own bucket (contracts/tenant-aws-setup.md). Idempotent.
import { CONFIG, TenantCfg, mcpUrl } from "./config.js";
import { http } from "./http.js";
import { adminToken } from "./tokens.js";
import { ensureAdminUser, ensureRoleUser, ensureMcpClientAudience } from "./tenants.js";
import { createBucket } from "./aws.js";

export interface TenantState {
  t: TenantCfg;
  serverId: string;
  adminTok: string;
}

export async function setupTenant(t: TenantCfg): Promise<TenantState> {
  await ensureAdminUser(t); // admin identity for control-plane ops
  await ensureRoleUser(t); // dedicated user + role that permits the AWS MCP (step 5/6)
  await ensureMcpClientAudience(t, mcpUrl(t.slug)); // align data-plane audience with this gateway
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
          // The sandbox rootfs is read-only (writable /tmp only); point HOME there so
          // the server can write its cache/config instead of crashing on a read-only $HOME.
          HOME: "/tmp",
          // AWS CLI v2 defaults to CRC64NVME upload checksums, which this ministack
          // build doesn't support — only add checksums when the API requires them.
          AWS_REQUEST_CHECKSUM_CALCULATION: "when_required",
        },
        credential_mode: "org_shared",
        allowed_roles: [t.role], // only users with this role may use the AWS MCP
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
