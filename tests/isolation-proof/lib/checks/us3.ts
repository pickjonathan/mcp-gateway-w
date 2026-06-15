// US3 — isolation holds under concurrent smoke load (FR-012/013; SC-004/005/006).
import { CONFIG, Flags } from "../config.js";
import { userToken } from "../tokens.js";
import { stressTenant, burstCalls } from "../stress.js";
import { Report } from "../report.js";
import { TenantState } from "../setup.js";

export async function runUS3(report: Report, states: TenantState[], flags: Flags): Promise<void> {
  const [a, b] = states;
  const aTok = await userToken(a.t);
  const bTok = await userToken(b.t);

  let scanned = 0;
  let hits = 0;
  const onResponse = (slug: string, payload: string) => {
    scanned++;
    const other = slug === a.t.slug ? b.t : a.t;
    if (payload.includes(other.bucket) || payload.includes(other.secretAccessKey)) hits++;
  };

  const opts = { concurrency: flags.concurrency, durationSec: flags.stressSeconds };
  const [as_, bs_] = await Promise.all([
    stressTenant(a.t, aTok, opts, { onResponse }),
    stressTenant(b.t, bTok, opts, { onResponse }),
  ]);
  report.stress[a.t.slug] = as_;
  report.stress[b.t.slug] = bs_;
  report.recordLeakage(scanned, hits);

  for (const [slug, s] of [[a.t.slug, as_], [b.t.slug, bs_]] as [string, any][]) {
    report.add({
      id: `US3.budget.${slug}`,
      ref: ["FR-012", "SC-005"],
      story: "US3",
      name: `${slug}: non-quota error < 1% and p95 <= 2s under load`,
      passed: s.error_rate < 0.01 && s.p95_ms <= 2000,
      detail: `calls=${s.calls} err=${(s.error_rate * 100).toFixed(2)}% p95=${s.p95_ms}ms quota=${s.quota_responses}`,
    });
  }
  report.add({
    id: "US3.leakage",
    ref: ["SC-004"],
    story: "US3",
    name: "zero cross-tenant leakage under load",
    passed: hits === 0,
    detail: `scanned ${scanned}, hits ${hits}`,
  });
}

export async function runQuotaIndependence(report: Report, states: TenantState[]): Promise<void> {
  const [a, b] = states;
  const aTok = await userToken(a.t);
  const bTok = await userToken(b.t);
  const n = Math.max(CONFIG.rateOrgPerMin + 20, 40);
  const [aBurst, bSteady] = await Promise.all([burstCalls(a.t, aTok, n), burstCalls(b.t, bTok, 20)]);
  const bTotal = Math.max(1, bSteady.ok + bSteady.err + bSteady.quota);
  const bRate = bSteady.ok / bTotal;
  report.add({
    id: "US3.quota_independence",
    ref: ["FR-013", "SC-006"],
    story: "US3",
    name: "one tenant throttled does not affect the other",
    passed: aBurst.quota > 0 && bRate >= 0.95,
    detail: `${a.t.slug} quota-responses=${aBurst.quota}; ${b.t.slug} ok-rate=${(bRate * 100).toFixed(0)}%`,
  });
}
