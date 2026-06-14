// US2 — cross-tenant access is impossible, and proven (FR-009/010/011).
import { CONFIG, mcpUrl } from "../config.js";
import { http } from "../http.js";
import { userToken } from "../tokens.js";
import { toolsList, callAws } from "../inspector.js";
import { hasDenial } from "../audit.js";
import { bucketAccessible } from "../aws.js";
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
    const srcUrl = mcpUrl(src.t.slug);
    const dstUrl = mcpUrl(dst.t.slug);

    // V1 — src token at dst endpoint: rejected, no dst data, audited
    const v1 = await toolsList(dstUrl, srcTok);
    collected.push(v1.stdout, v1.stderr);
    const v1Denied = !v1.ok || /401|403|unauthor|invalid_token|audience|issuer|forbidden/i.test(v1.stdout + v1.stderr);
    const v1NoData = !v1.stdout.includes(dst.t.bucket);
    const aud1 = await hasDenial(dst.t.slug, dst.adminTok);
    report.add({
      id: `US2.V1.${src.t.slug}->${dst.t.slug}`,
      ref: ["FR-009", "FR-010"],
      story: "US2",
      name: `V1 ${src.t.slug} token rejected at ${dst.t.slug} (+audited)`,
      passed: v1Denied && v1NoData && aud1.found,
      detail: `denied=${v1Denied} noData=${v1NoData} audited=${aud1.found}`,
    });

    // V2 — src catalog hides dst's resources (org-scoped aggregation)
    const v2 = await toolsList(srcUrl, srcTok);
    collected.push(v2.stdout, v2.stderr);
    report.add({
      id: `US2.V2.${src.t.slug}`,
      ref: ["FR-009", "FR-011"],
      story: "US2",
      name: `V2 ${src.t.slug} catalog does not reveal ${dst.t.slug}`,
      passed: v2.ok && !v2.stdout.includes(dst.t.bucket),
      detail: v2.ok ? "" : `tools/list failed: ${(v2.stderr || "").slice(0, 150)}`,
    });

    // V3 — src's admin token against dst's control-plane org → denied
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

    // V4 — via src's OWN server+creds, attempt dst's bucket → denied (account boundary)
    const v4 = await callAws(srcUrl, srcTok, `aws s3 ls s3://${dst.t.bucket}/`);
    collected.push(v4.stdout, v4.stderr);
    const v4Blocked = !v4.ok || /NoSuchBucket|AccessDenied|Not.?Found|does not exist|error/i.test(v4.stdout + v4.stderr);
    const crossOk = await bucketAccessible(src.t.accountId, src.t.secretAccessKey, dst.t.bucket);
    report.add({
      id: `US2.V4.${src.t.slug}->${dst.t.slug}`,
      ref: ["FR-009"],
      story: "US2",
      name: `V4 ${src.t.slug} creds cannot reach ${dst.t.slug}'s bucket`,
      passed: v4Blocked && !crossOk,
      detail: `serverBlocked=${v4Blocked} crossAccountAccess=${crossOk}`,
    });
  }

  // V6 — no secret value leaked in any response collected above
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
