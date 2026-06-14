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
    const summary = this.passed ? "\x1b[32mPASS\x1b[0m" : "\x1b[31mFAIL\x1b[0m";
    console.log(`\nOVERALL: ${summary}   report → ${path}   (${this.duration_s}s)`);
    if (!this.preflightsPassed) return EXIT.PRECONDITION;
    return this.passed ? EXIT.PASS : EXIT.CHECK_FAILED;
  }
}
