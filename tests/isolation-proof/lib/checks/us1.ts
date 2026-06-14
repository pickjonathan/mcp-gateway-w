// US1 — each tenant uses its own AWS MCP server end-to-end via the Inspector.
import { CONFIG, mcpUrl } from "../config.js";
import { http } from "../http.js";
import { userToken } from "../tokens.js";
import { toolsList, callAws } from "../inspector.js";
import { Report } from "../report.js";
import { TenantState } from "../setup.js";

export async function runUS1(report: Report, states: TenantState[]): Promise<void> {
  for (const st of states) {
    const t = st.t;
    const url = mcpUrl(t.slug);
    const tok = await userToken(t);

    // tools/list includes call_aws (FR-007)
    const list = await toolsList(url, tok);
    report.add({
      id: `US1.tools_list.${t.slug}`,
      ref: ["FR-007"],
      story: "US1",
      name: `${t.slug}: tools/list includes call_aws`,
      passed: list.ok && /call_aws/.test(list.stdout),
      detail: list.ok ? "" : `inspector exit ${list.exitCode}: ${(list.stderr || "").slice(0, 200)}`,
    });

    // server writes to its OWN bucket (uses injected write-only creds, in-sandbox)
    const key = `us1-${t.slug}.txt`;
    const write = await callAws(url, tok, `aws s3api put-object --bucket ${t.bucket} --key ${key} --body /dev/null`);
    report.add({
      id: `US1.write.${t.slug}`,
      ref: ["FR-007"],
      story: "US1",
      name: `${t.slug}: server writes its own bucket`,
      passed: write.ok,
      detail: write.ok ? "" : `${(write.stderr || write.stdout).slice(0, 200)}`,
    });

    // server reads its OWN bucket back and sees the object
    const ls = await callAws(url, tok, `aws s3 ls s3://${t.bucket}/`);
    report.add({
      id: `US1.read.${t.slug}`,
      ref: ["FR-007"],
      story: "US1",
      name: `${t.slug}: server reads its own bucket`,
      passed: ls.ok && ls.stdout.includes(key),
      detail: ls.ok ? "" : `${(ls.stderr || ls.stdout).slice(0, 200)}`,
    });

    // write-only credentials (FR-006): server GET never echoes the secret
    const srv = await http("GET", `${CONFIG.controlPlane}/v1/orgs/${t.slug}/servers/${st.serverId}`, { token: st.adminTok });
    const noSecret = !srv.text.includes(t.secretAccessKey);
    const flagged = srv.json?.credential_set === true || srv.json?.credentialSet === true;
    report.add({
      id: `US1.write_only.${t.slug}`,
      ref: ["FR-006", "SC-007"],
      story: "US1",
      name: `${t.slug}: credentials are write-only`,
      passed: noSecret && flagged,
      detail: noSecret ? (flagged ? "" : "credential_set flag not true") : "SECRET LEAKED in server GET response",
    });
  }
}
