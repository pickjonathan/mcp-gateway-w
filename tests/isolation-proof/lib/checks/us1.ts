// US1 — each realm's DEDICATED ROLED USER, via the gateway, uses its own AWS MCP:
// lists tools (Inspector, FR-007), then writes a unique marker + reads its own
// bucket through the sandboxed AWS server (real S3 on the shared ministack, its
// own account). The marker is later validated server-side per account (routing.ts).
import { CONFIG, mcpUrl } from "../config.js";
import { http } from "../http.js";
import { userToken } from "../tokens.js";
import { toolsList as inspectorList } from "../inspector.js";
import { connect, findTool } from "../mcp.js";
import { Report } from "../report.js";
import { TenantState } from "../setup.js";

export async function runUS1(report: Report, states: TenantState[]): Promise<void> {
  for (const st of states) {
    const t = st.t;
    const url = mcpUrl(t.slug);
    const tok = await userToken(t); // the dedicated roled user (has the permission)

    // FR-007: the MCP Inspector lists this tenant's tools for its user
    const insp = await inspectorList(url, tok);
    const inspOk = insp.ok && /call_aws/.test(insp.stdout);
    report.add({
      id: `US1.inspector.${t.slug}`,
      ref: ["FR-007"],
      story: "US1",
      name: `${t.slug}: ${t.user} lists AWS tools via the Inspector`,
      passed: inspOk,
      detail: inspOk ? "" : `${(insp.stderr || insp.stdout || String(insp.exitCode)).slice(0, 160)}`,
    });

    let client: any;
    try {
      client = await connect(url, tok);
    } catch (e: any) {
      report.add({ id: `US1.discover.${t.slug}`, ref: ["FR-007"], story: "US1", name: `${t.slug}: connect MCP session`, passed: false, detail: String(e?.message || e).slice(0, 160) });
      continue;
    }
    try {
      const lt: any = await client.listTools();
      const callName = findTool(lt.tools || [], "call_aws");
      report.add({
        id: `US1.permission.${t.slug}`,
        ref: ["FR-007"],
        story: "US1",
        name: `${t.slug}: ${t.user} (role ${t.role}) may use the AWS MCP`,
        passed: !!callName,
        detail: callName || "no call_aws tool — role/permission missing?",
      });
      if (callName) {
        // write a UNIQUE marker object via the MCP (proof of which account this MCP hits)
        const w = await client
          .callTool({ name: callName, arguments: { cli_command: `aws s3api put-object --bucket ${t.bucket} --key ${t.marker}` } })
          .catch((e: any) => ({ isError: true, _err: String(e?.message || e) }));
        report.add({
          id: `US1.write.${t.slug}`,
          ref: ["FR-007"],
          story: "US1",
          name: `${t.slug}: MCP writes marker to its own bucket`,
          passed: !w.isError && !/AccessDenied|NoSuchBucket|exception|Could not/i.test(JSON.stringify(w)),
          detail: w.isError ? (w._err || JSON.stringify(w)).slice(0, 180) : `${t.bucket}/${t.marker}`,
        });

        const r = await client
          .callTool({ name: callName, arguments: { cli_command: `aws s3 ls s3://${t.bucket}/proof/` } })
          .catch((e: any) => ({ isError: true, _err: String(e?.message || e) }));
        const rtext = JSON.stringify(r);
        report.add({
          id: `US1.read.${t.slug}`,
          ref: ["FR-007"],
          story: "US1",
          name: `${t.slug}: MCP reads its own bucket`,
          passed: !r.isError && rtext.includes(`${t.slug}-via-gateway`),
          detail: r.isError ? (r._err || rtext).slice(0, 180) : "",
        });
      }
    } finally {
      try {
        await client.close();
      } catch {
        /* ignore */
      }
    }

    // write-only credentials (FR-006)
    const srv = await http("GET", `${CONFIG.controlPlane}/v1/orgs/${t.slug}/servers/${st.serverId}`, { token: st.adminTok });
    const noSecret = !srv.text.includes(t.secretAccessKey);
    const flagged = srv.json?.credential_set === true || srv.json?.credentialSet === true;
    report.add({
      id: `US1.write_only.${t.slug}`,
      ref: ["FR-006", "SC-007"],
      story: "US1",
      name: `${t.slug}: credentials are write-only`,
      passed: noSecret && flagged,
      detail: noSecret ? (flagged ? "" : "credential_set not true") : "SECRET LEAKED in server GET",
    });
  }
}
