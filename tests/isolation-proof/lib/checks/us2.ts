// US2 — cross-tenant access is impossible, and proven (FR-009/010/011).
// Real MCP SDK for the data-plane vectors; control-plane API for cross-org admin.
import { CONFIG, mcpUrl } from "../config.js";
import { http } from "../http.js";
import { userToken } from "../tokens.js";
import { connect, tryConnect } from "../mcp.js";
import { auditChainIntact } from "../audit.js";
import { Report } from "../report.js";
import { TenantState } from "../setup.js";

export async function runUS2(report: Report, states: TenantState[]): Promise<void> {
  const pairs: [number, number][] = [[0, 1], [1, 0]];
  const collected: string[] = [];
  const secrets = states.map((s) => s.t.secretAccessKey);

  for (const [si, di] of pairs) {
    const src = states[si];
    const dst = states[di];
    const srcTok = await userToken(src.t);
    const dstUrl = mcpUrl(dst.t.slug);

    // V1 — src's token presented to dst's endpoint: the MCP handshake must be rejected.
    const v1 = await tryConnect(dstUrl, srcTok);
    collected.push(v1.error || "");
    report.add({
      id: `US2.V1.${src.t.slug}->${dst.t.slug}`,
      ref: ["FR-009"],
      story: "US2",
      name: `V1 ${src.t.slug} token rejected at ${dst.t.slug}`,
      passed: !v1.connected,
      detail: !v1.connected ? `rejected (${(v1.error || "").slice(0, 70)})` : "NOT rejected — cross-tenant token accepted!",
    });

    // V2 + V4 share one src MCP session.
    let client: any;
    let tools: any[] = [];
    try {
      client = await connect(mcpUrl(src.t.slug), srcTok);
      const lt: any = await client.listTools();
      tools = lt.tools || [];
    } catch (e: any) {
      collected.push(String(e?.message || e));
    }
    collected.push(JSON.stringify(tools));

    // V2 — src's catalog must not reveal dst's resources.
    report.add({
      id: `US2.V2.${src.t.slug}`,
      ref: ["FR-009", "FR-011"],
      story: "US2",
      name: `V2 ${src.t.slug} catalog does not reveal ${dst.t.slug}`,
      passed: !!client && !JSON.stringify(tools).includes(dst.t.bucket),
      detail: client ? `${tools.length} own tools, none referencing ${dst.t.bucket}` : "could not list src catalog",
    });

    // V3 — src's admin token against dst's control-plane org → denied.
    const v3 = await http("GET", `${CONFIG.controlPlane}/v1/orgs/${dst.t.slug}/servers`, { token: src.adminTok });
    const v3Denied = v3.status === 401 || v3.status === 403 || (Array.isArray(v3.json) && v3.json.length === 0);
    report.add({
      id: `US2.V3.${src.t.slug}->${dst.t.slug}`,
      ref: ["FR-009"],
      story: "US2",
      name: `V3 ${src.t.slug} admin token denied on ${dst.t.slug} control-plane`,
      passed: v3Denied,
      detail: `status=${v3.status}`,
    });

    // (Cross-tenant downstream access is covered by the Routing checks: each user is
    // routed only to its own MCP/account, validated server-side.)

    try {
      await client?.close();
    } catch {
      /* ignore */
    }
  }

  // FR-010 — each tenant's audit trail is non-empty and hash-chained (tamper-evident).
  for (const st of states) {
    const chain = await auditChainIntact(st.t.slug, st.adminTok);
    report.add({
      id: `US2.audit.${st.t.slug}`,
      ref: ["FR-010"],
      story: "US2",
      name: `${st.t.slug}: audit trail is tamper-evident (hash-chained)`,
      passed: chain.ok,
      detail: `${chain.count} hash-chained records`,
    });
  }

  // V6 — no secret value leaked across any collected output.
  const blob = collected.join("\n");
  const hits = secrets.filter((s) => s && blob.includes(s)).length;
  report.recordLeakage(collected.length, hits);
  report.add({
    id: "US2.V6",
    ref: ["FR-011", "SC-007"],
    story: "US2",
    name: "V6 no secret values leaked in any response",
    passed: hits === 0,
    detail: `scanned ${collected.length} outputs, ${hits} secret hits`,
  });
}
