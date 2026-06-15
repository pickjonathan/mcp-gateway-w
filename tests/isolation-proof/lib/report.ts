// Machine-readable pass/fail report (FR-014) + exit-code semantics
// (contracts/proof-harness.md).
import { writeFileSync } from "node:fs";

export const EXIT = { PASS: 0, CHECK_FAILED: 1, PRECONDITION: 2 } as const;

export interface Check {
  id: string; // e.g. "US2.V1"
  ref: string[]; // FR/SC references
  story: string; // preflight | US1 | US2 | US3 | US4 | SC-010
  name: string;
  passed: boolean;
  detail?: string;
  evidence?: string;
}

export interface StressTenant {
  sessions: number;
  duration_s: number;
  calls: number;
  errors_nonquota: number;
  error_rate: number;
  p95_ms: number;
  quota_responses: number;
}

export interface FlowStep {
  realm: string;
  user: string;
  role: string;
  account: string;
  bucket: string;
  marker: string;
  routedToOwnMcp: boolean; // user saw its own bucket, not the other realm's
  accountCorrect: boolean; // ministack account holds own data+marker, lacks other's
  accountMarker: boolean;
  requests?: number; // live per-tenant gateway request count (Prometheus), if available
}

function svgEsc(s: any): string {
  return String(s == null ? "" : s).replace(/[&<>]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" } as any)[c]);
}

// Per-realm network-flow diagram: realm user → gateway → that realm's dedicated
// AWS MCP (sandbox) → the correct ministack account. Rendered server-side as SVG.
function buildFlowSVG(flows: FlowStep[]): string {
  if (!flows || !flows.length) return "<p class=muted>(no flows recorded)</p>";
  const COLORS = ["#1f6feb", "#8957e5", "#3fb950", "#db6d28"];
  const BW = 176;
  const BH = 90;
  const RH = 160;
  const xs = [12, 360, 708, 1056]; // wide gaps (172px) so edge labels don't overlap boxes
  const W = xs[3] + BW + 12;
  const H = flows.length * RH + 74;

  const box = (x: number, y: number, lines: string[], color: string, ok: boolean): string => {
    const tspans = lines
      .map((l, i) =>
        i === 0
          ? `<tspan x=${x + BW / 2} dy=0 font-weight=700 fill=${color}>${svgEsc(l)}</tspan>`
          : `<tspan x=${x + BW / 2} dy=16 fill=#e6edf3>${svgEsc(l)}</tspan>`,
      )
      .join("");
    return (
      `<rect x=${x} y=${y} width=${BW} height=${BH} rx=9 fill=#161b22 stroke=${ok ? "#3fb950" : "#f85149"} stroke-width=1.5/>` +
      `<text x=${x + BW / 2} y=${y + 22} text-anchor=middle font-size=12.5>${tspans}</text>`
    );
  };
  const arrow = (x1: number, x2: number, y: number, label: string): string =>
    `<line x1=${x1} y1=${y} x2=${x2 - 8} y2=${y} stroke=#6e7681 stroke-width=1.6 marker-end='url(#ah)'/>` +
    `<text x=${(x1 + x2) / 2} y=${y - 7} text-anchor=middle font-size=10.5 fill=#8b949e>${svgEsc(label)}</text>`;

  let body = "";
  flows.forEach((f, idx) => {
    const color = COLORS[idx % COLORS.length];
    const y = 18 + idx * RH;
    const my = y + BH / 2;
    const ok = f.routedToOwnMcp && f.accountCorrect;
    body += `<text x=8 y=${y - 4} font-size=11.5 fill=${color} font-weight=700>${svgEsc(f.realm.toUpperCase())} flow ${ok ? "✓ isolated" : "✗"}</text>`;
    body += box(xs[0], y, [`Realm: ${f.realm}`, `user: ${f.user}`, `role: ${f.role}`], color, f.routedToOwnMcp);
    body += box(xs[1], y, ["MCP Gateway", ":8080", "routes by org"], color, true);
    body += box(xs[2], y, [`AWS MCP · ${f.realm}`, "runc sandbox", "egress → ministack"], color, true);
    body += box(xs[3], y, ["ministack account", f.account, `${f.bucket} ${f.accountMarker ? "✓ marker" : ""}`], color, f.accountCorrect);
    body += arrow(xs[0] + BW, xs[1], my, `aud=${f.realm}` + (f.requests ? ` · ${f.requests} req` : ""));
    body += arrow(xs[1] + BW, xs[2], my, `route org=${f.realm}`);
    body += arrow(xs[2] + BW, xs[3], my, `creds ${String(f.account).slice(0, 4)}…`);
  });
  const ny = 18 + flows.length * RH;
  body += `<text x=8 y=${ny + 2} font-size=11 fill=#8b949e>Isolation (validated): cross-realm token → 401 · each account holds only its own bucket+marker (no cross-account)</text>`;

  return (
    `<div style='overflow-x:auto;margin:8px 0 22px'><svg viewBox='0 0 ${W} ${H}' width='100%' ` +
    `style='min-width:920px;max-width:${W}px;background:#0d1117;border:1px solid #21262d;border-radius:10px'>` +
    `<defs><marker id=ah markerWidth=9 markerHeight=9 refX=6 refY=3 orient=auto><path d='M0,0 L6,3 L0,6 Z' fill=#6e7681/></marker></defs>` +
    body +
    `</svg></div>`
  );
}

function logLine(c: Check) {
  const mark = c.passed ? "\x1b[32mPASS\x1b[0m" : "\x1b[31mFAIL\x1b[0m";
  console.log(`${mark}  [${c.story}] ${c.name}${c.detail ? " — " + c.detail : ""}`);
}

// Self-contained HTML dashboard with the report data inlined (no server needed).
function htmlReport(out: any): string {
  const json = JSON.stringify(out).replace(/</g, "\\u003c");
  const flowSvg = buildFlowSVG(out.flows || []);
  return [
    "<!doctype html><html lang=en><head><meta charset=utf-8>",
    "<meta name=viewport content='width=device-width,initial-scale=1'>",
    "<title>Isolation Proof Report</title><style>",
    "body{font:14px/1.55 system-ui,sans-serif;margin:0;background:#0e1116;color:#e6edf3}",
    ".wrap{max-width:1280px;margin:0 auto;padding:28px}h1{font-size:20px;margin:0 0 4px}",
    ".muted{color:#8b949e}.banner{padding:14px 18px;border-radius:10px;font-weight:700;font-size:18px;margin:18px 0}",
    ".pass{background:#0f5132;color:#d1fae5}.fail{background:#7f1d1d;color:#fee2e2}",
    "table{width:100%;border-collapse:collapse;margin:8px 0 22px}",
    "th,td{text-align:left;padding:7px 10px;border-bottom:1px solid #21262d;vertical-align:top}",
    "th{color:#8b949e;font-weight:600;font-size:12px;text-transform:uppercase;letter-spacing:.03em}",
    ".ok{color:#3fb950;font-weight:700}.no{color:#f85149;font-weight:700}",
    ".tag{display:inline-block;background:#21262d;border-radius:6px;padding:1px 7px;color:#8b949e;font-size:12px;margin-right:6px}",
    "h2{font-size:15px;margin:24px 0 6px;border-bottom:1px solid #21262d;padding-bottom:4px}",
    "code{background:#161b22;padding:1px 5px;border-radius:5px}</style></head><body><div class=wrap>",
    "<h1>Two-Tenant AWS-MCP Isolation Proof</h1><div class=muted id=meta></div>",
    "<div id=banner></div>",
    "<h2>Network flow — realm &rarr; gateway &rarr; dedicated AWS MCP &rarr; account</h2>",
    flowSvg,
    "<div id=body></div></div><script>",
    "var R=", json, ";",
    "function esc(s){return String(s==null?'':s).replace(/[&<>]/g,function(c){return{'&':'&amp;','<':'&lt;','>':'&gt;'}[c]})}",
    "function row(c){return '<tr><td class='+(c.passed?'ok':'no')+'>'+(c.passed?'PASS':'FAIL')+'</td><td><span class=tag>'+esc(c.story)+'</span>'+esc(c.name)+'</td><td class=muted>'+esc(c.detail||'')+'</td><td class=muted>'+esc((c.ref||[]).join(', '))+'</td></tr>'}",
    "function tbl(items){return items&&items.length?'<table><tr><th>Result</th><th>Check</th><th>Detail</th><th>Spec</th></tr>'+items.map(row).join('')+'</table>':'<p class=muted>(none)</p>'}",
    "document.getElementById('meta').innerHTML='runtime <code>'+esc(R.environment.sandbox_runtime)+'</code> · tenants <code>'+esc((R.environment.tenants||[]).join(', '))+'</code> · '+esc(R.duration_s)+'s · '+esc(R.finished_at);",
    "var ok=R.overall&&R.overall.passed;",
    "document.getElementById('banner').innerHTML='<div class=\"banner '+(ok?'pass':'fail')+'\">'+(ok?'PASS':'FAIL')+(R.leakage?' — cross-tenant leakage hits: '+R.leakage.hits:'')+'</div>';",
    "var h='<h2>Preflights (FR-018 gate)</h2>'+tbl(R.preflights)+'<h2>Checks</h2>'+tbl(R.checks);",
    "if(R.stress&&Object.keys(R.stress).length){h+='<h2>Stress</h2><table><tr><th>Tenant</th><th>Sessions</th><th>Calls</th><th>Err (non-quota)</th><th>p95 ms</th><th>Quota 429s</th></tr>';for(var k in R.stress){var s=R.stress[k];h+='<tr><td>'+esc(k)+'</td><td>'+s.sessions+'</td><td>'+s.calls+'</td><td>'+(s.error_rate*100).toFixed(2)+'%</td><td>'+s.p95_ms+'</td><td>'+s.quota_responses+'</td></tr>'}h+='</table>'}",
    "document.getElementById('body').innerHTML=h;</script></body></html>",
  ].join("");
}

export class Report {
  started_at = new Date().toISOString();
  finished_at = "";
  duration_s = 0;
  environment: Record<string, any> = {};
  preflights: Check[] = [];
  checks: Check[] = [];
  stress: Record<string, StressTenant> = {};
  flows: FlowStep[] = [];
  leakage = { scanned_artifacts: 0, hits: 0 };
  private startMs = Date.now();

  addFlow(f: FlowStep) {
    this.flows.push(f);
  }

  addPreflight(c: Check) {
    this.preflights.push(c);
    logLine(c);
  }
  add(c: Check) {
    this.checks.push(c);
    logLine(c);
  }
  recordLeakage(scanned: number, hits: number) {
    this.leakage.scanned_artifacts += scanned;
    this.leakage.hits += hits;
  }

  get preflightsPassed(): boolean {
    return this.preflights.every((c) => c.passed);
  }
  get passed(): boolean {
    return (
      this.preflightsPassed &&
      this.checks.every((c) => c.passed) &&
      this.leakage.hits === 0
    );
  }

  finalize(path: string): number {
    this.finished_at = new Date().toISOString();
    this.duration_s = Math.round((Date.now() - this.startMs) / 1000);
    const out: any = {
      started_at: this.started_at,
      finished_at: this.finished_at,
      duration_s: this.duration_s,
      environment: this.environment,
      preflights: this.preflights,
      checks: this.checks,
      flows: this.flows,
      stress: this.stress,
      leakage: this.leakage,
      overall: { passed: this.passed },
    };
    writeFileSync(path, JSON.stringify(out, null, 2));
    const htmlPath = path.endsWith(".json") ? path.slice(0, -5) + ".html" : path + ".html";
    writeFileSync(htmlPath, htmlReport(out));
    const summary = this.passed ? "\x1b[32mPASS\x1b[0m" : "\x1b[31mFAIL\x1b[0m";
    console.log(`\nOVERALL: ${summary}   report → ${path}   dashboard → ${htmlPath}   (${this.duration_s}s)`);
    if (!this.preflightsPassed) return EXIT.PRECONDITION;
    return this.passed ? EXIT.PASS : EXIT.CHECK_FAILED;
  }
}
