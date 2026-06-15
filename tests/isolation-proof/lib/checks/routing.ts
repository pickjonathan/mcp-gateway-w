// Routing / per-account isolation — the core of the proof:
//   (a) each realm's USER, via the gateway, is routed to ITS OWN AWS MCP (sees its
//       own bucket, never the other realm's);
//   (b) SERVER-SIDE, ministack's per-account state confirms each MCP wrote to the
//       CORRECT account (its bucket + unique marker live in its account, and the
//       other realm's bucket does NOT), i.e. the correct MCP called the correct account.
// Plus ministack request logs as supplementary evidence (ministack exposes no
// per-account metrics/log-attribution — research: its README).
import { execSync } from "node:child_process";
import { CONFIG, mcpUrl } from "../config.js";
import { userToken } from "../tokens.js";
import { connect, findTool } from "../mcp.js";
import { listBuckets, objectExists } from "../aws.js";
import { Report } from "../report.js";
import { TenantState } from "../setup.js";

export async function runRouting(report: Report, states: TenantState[]): Promise<void> {
  for (let i = 0; i < states.length; i++) {
    const self = states[i].t;
    const other = states[(i + 1) % states.length].t;

    // (a) USER-VIEW: as self's user, list buckets through the gateway → MCP. The
    // user must see its OWN bucket and NOT the other realm's (routed to own MCP/account).
    const tok = await userToken(self);
    let sawOwn = false;
    let sawOther = false;
    let detailA = "";
    let client: any;
    try {
      client = await connect(mcpUrl(self.slug), tok);
      const lt: any = await client.listTools();
      const callName = findTool(lt.tools || [], "call_aws");
      const r: any = await client.callTool({ name: callName, arguments: { cli_command: "aws s3 ls" } });
      const text = JSON.stringify(r);
      sawOwn = text.includes(self.bucket);
      sawOther = text.includes(other.bucket);
      detailA = `sees own '${self.bucket}'=${sawOwn}, sees other '${other.bucket}'=${sawOther}`;
    } catch (e: any) {
      detailA = String(e?.message || e).slice(0, 160);
    } finally {
      try {
        await client?.close();
      } catch {
        /* ignore */
      }
    }
    report.add({
      id: `ROUTE.user.${self.slug}`,
      ref: ["FR-009"],
      story: "Routing",
      name: `${self.slug} user → its own AWS MCP (sees own bucket, not ${other.slug}'s)`,
      passed: sawOwn && !sawOther,
      detail: detailA,
    });

    // (b) SERVER-SIDE: ministack account state proves the correct MCP → correct account.
    let okOwn = false;
    let hasMarker = false;
    let noOther = false;
    let detailB = "";
    try {
      const buckets = await listBuckets(self.accountId, self.secretAccessKey);
      okOwn = buckets.includes(self.bucket);
      noOther = !buckets.includes(other.bucket);
      hasMarker = await objectExists(self.accountId, self.secretAccessKey, self.bucket, self.marker);
      detailB = `account ${self.accountId}: has ${self.bucket}=${okOwn}, marker=${hasMarker}, lacks ${other.bucket}=${noOther}`;
    } catch (e: any) {
      detailB = String(e?.message || e).slice(0, 160);
    }
    report.add({
      id: `ROUTE.account.${self.slug}`,
      ref: ["FR-009"],
      story: "Routing",
      name: `ministack account ${self.accountId} holds ${self.slug}'s data+marker only (not ${other.slug}'s)`,
      passed: okOwn && hasMarker && noOther,
      detail: detailB,
    });

    // Live per-tenant gateway request count (Prometheus) to annotate the flow edge.
    let requests: number | undefined;
    try {
      const u = `${CONFIG.prometheus}/api/v1/query?query=${encodeURIComponent(`sum(mcp_requests_total{org="${self.slug}"})`)}`;
      const j: any = await (await fetch(u)).json();
      const v = j?.data?.result?.[0]?.value?.[1];
      if (v != null) requests = Math.round(Number(v));
    } catch {
      /* optional */
    }

    // Record the per-realm flow for the network-flow diagram.
    report.addFlow({
      realm: self.slug,
      user: self.user,
      role: self.role,
      account: self.accountId,
      bucket: self.bucket,
      marker: self.marker,
      routedToOwnMcp: sawOwn && !sawOther,
      accountCorrect: okOwn && hasMarker && noOther,
      accountMarker: hasMarker,
      requests,
    });
  }

  // Supplementary: ministack request logs (operation-level; no per-account attribution).
  try {
    // ministack's DEBUG request logs go to stderr → merge with 2>&1.
    const logs = execSync(`docker logs --tail 400 ${CONFIG.ministackContainer} 2>&1`, { encoding: "utf8" });
    const s3lines = logs.split("\n").filter((l) => /service=s3/i.test(l));
    const buckets = [...new Set(s3lines.map((l) => (l.match(/\s\/([a-z0-9][a-z0-9.\-]+)/i) || [])[1]).filter(Boolean))];
    report.add({
      id: "ROUTE.logs",
      ref: ["FR-010"],
      story: "Routing",
      name: "ministack request logs captured (per-bucket evidence; ministack has no per-account attribution)",
      passed: s3lines.length > 0,
      detail: `${s3lines.length} S3 requests logged; buckets touched: ${buckets.join(", ") || "(none)"}`,
    });
  } catch {
    /* optional */
  }
}
