// Concurrent smoke-load driver via the MCP TS SDK (FR-012). Replays the same
// Streamable-HTTP JSON-RPC the Inspector speaks, but many sessions at once.
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import { TenantCfg, mcpUrl } from "./config.js";
import { StressTenant } from "./report.js";

async function connect(url: string, token: string): Promise<Client> {
  const transport = new StreamableHTTPClientTransport(new URL(url), {
    requestInit: { headers: { Authorization: `Bearer ${token}` } },
  });
  const client = new Client({ name: "isolation-proof", version: "0.0.0" }, { capabilities: {} });
  await client.connect(transport);
  return client;
}

const isQuota = (e: any) => e?.code === -32000 || /-32000|rate limit/i.test(String(e?.message || e));

export interface StressOpts {
  concurrency: number;
  durationSec: number;
}
export interface StressHooks {
  onResponse?: (tenantSlug: string, payload: string) => void;
}

export async function stressTenant(
  t: TenantCfg,
  token: string,
  opts: StressOpts,
  hooks: StressHooks = {},
): Promise<StressTenant> {
  const url = mcpUrl(t.slug);
  const latencies: number[] = [];
  let calls = 0;
  let errorsNonQuota = 0;
  let quota = 0;
  const deadline = Date.now() + opts.durationSec * 1000;

  async function worker() {
    let client: Client | undefined;
    try {
      client = await connect(url, token);
    } catch (e: any) {
      errorsNonQuota++;
      return;
    }
    while (Date.now() < deadline) {
      const t0 = Date.now();
      try {
        const res: any = await client.callTool({ name: "call_aws", arguments: { cli_command: `aws s3 ls s3://${t.bucket}/` } });
        calls++;
        latencies.push(Date.now() - t0);
        hooks.onResponse?.(t.slug, JSON.stringify(res));
      } catch (e: any) {
        calls++;
        if (isQuota(e)) quota++;
        else errorsNonQuota++;
      }
    }
    try {
      await client.close();
    } catch {
      /* ignore */
    }
  }

  await Promise.all(Array.from({ length: opts.concurrency }, () => worker()));
  latencies.sort((a, b) => a - b);
  const p95 = latencies.length ? latencies[Math.min(latencies.length - 1, Math.floor(latencies.length * 0.95))] : 0;
  const denom = Math.max(1, calls - quota);
  return {
    sessions: opts.concurrency,
    duration_s: opts.durationSec,
    calls,
    errors_nonquota: errorsNonQuota,
    error_rate: errorsNonQuota / denom,
    p95_ms: p95,
    quota_responses: quota,
  };
}

/** Fire `n` sequential tool calls; report how many hit the quota error vs. ok. */
export async function burstCalls(t: TenantCfg, token: string, n: number): Promise<{ ok: number; quota: number; err: number }> {
  let ok = 0;
  let quota = 0;
  let err = 0;
  let client: Client;
  try {
    client = await connect(mcpUrl(t.slug), token);
  } catch (e: any) {
    return { ok: 0, quota: 0, err: n };
  }
  for (let i = 0; i < n; i++) {
    try {
      await client.callTool({ name: "call_aws", arguments: { cli_command: `aws s3 ls s3://${t.bucket}/` } });
      ok++;
    } catch (e: any) {
      if (isQuota(e)) quota++;
      else err++;
    }
  }
  try {
    await client.close();
  } catch {
    /* ignore */
  }
  return { ok, quota, err };
}
