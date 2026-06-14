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

function logLine(c: Check) {
  const mark = c.passed ? "\x1b[32mPASS\x1b[0m" : "\x1b[31mFAIL\x1b[0m";
  console.log(`${mark}  [${c.story}] ${c.name}${c.detail ? " — " + c.detail : ""}`);
}

// Self-contained HTML dashboard with the report data inlined (no server needed).
function htmlReport(out: any): string {
  const json = JSON.stringify(out).replace(/</g, "\\u003c");
  return [
    "<!doctype html><html lang=en><head><meta charset=utf-8>",
    "<meta name=viewport content='width=device-width,initial-scale=1'>",
    "<title>Isolation Proof Report</title><style>",
    "body{font:14px/1.55 system-ui,sans-serif;margin:0;background:#0e1116;color:#e6edf3}",
    ".wrap{max-width:1000px;margin:0 auto;padding:28px}h1{font-size:20px;margin:0 0 4px}",
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
    "<div id=banner></div><div id=body></div></div><script>",
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
  leakage = { scanned_artifacts: 0, hits: 0 };
  private startMs = Date.now();

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
