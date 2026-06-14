// FR-018 gate: the downstream-denial proof is only honest if the emulator
// isolates S3 per account. Probe it; abort loudly (no proof claims) if not.
import { CONFIG, TenantCfg } from "./config.js";
import { createBucket, bucketAccessible, listObjects } from "./aws.js";
import { Report } from "./report.js";

export async function runPreflights(report: Report, ten: TenantCfg[]): Promise<boolean> {
  const [a, b] = ten;

  // 1. endpoint reachable: the request must get an S3-level RESPONSE (even an
  //    error like NoSuchBucket) — a transport/connection error means the emulator
  //    is not actually there, so this MUST fail (not silently pass).
  let endpointOk = false;
  let detail = "";
  try {
    await listObjects(a.accountId, a.secretAccessKey, "preflight-probe-nonexistent");
    endpointOk = true; // a 2xx (unlikely) still proves reachability
  } catch (e: any) {
    // AWS SDK v3 sets $metadata.httpStatusCode only when an HTTP response was
    // received — i.e. we reached the emulator. A bare ECONNREFUSED/timeout has none.
    if (e?.$metadata?.httpStatusCode) {
      endpointOk = true;
    } else {
      detail = `cannot reach emulator at ${CONFIG.awsEndpoint}: ${String(e?.name || e?.message || e)}`;
    }
  }
  report.addPreflight({ id: "PF1", ref: ["FR-001"], story: "preflight", name: "endpoint_override (emulator reachable)", passed: endpointOk, detail });
  if (!endpointOk) return false;

  // 2. per-account S3 isolation: account A's bucket must be INACCESSIBLE to account B.
  await createBucket(a);
  const visibleToB = await bucketAccessible(b.accountId, b.secretAccessKey, a.bucket);
  const isolated = !visibleToB;
  report.addPreflight({
    id: "PF2",
    ref: ["FR-018"],
    story: "preflight",
    name: "s3_per_account_isolation",
    passed: isolated,
    detail: isolated
      ? `account ${b.accountId} cannot access account ${a.accountId}'s bucket "${a.bucket}"`
      : `ABORT: emulator does NOT isolate S3 per account — account ${b.accountId} can access "${a.bucket}". The downstream-denial proof would be dishonest (research.md D1). The gateway boundary (US2 V1–V3) is unaffected, but this run cannot honestly assert V4.`,
  });
  return isolated;
}
