// Two-Tenant AWS-MCP Isolation Proof — orchestrator (FR-015).
// preflight (FR-018) → setup → US1 → US2 → SC-010 → US3 → report → teardown.
import { parseFlags, tenants, CONFIG } from "./lib/config.js";
import { Report } from "./lib/report.js";
import { operatorToken } from "./lib/tokens.js";
import { provisionTenant } from "./lib/tenants.js";
import { setupTenant, TenantState } from "./lib/setup.js";
import { teardownTenant, verifyClean } from "./lib/teardown.js";
import { runPreflights } from "./lib/preflight.js";
import { runUS1 } from "./lib/checks/us1.js";
import { runUS2 } from "./lib/checks/us2.js";
import { runEgress } from "./lib/checks/egress.js";
import { runUS3, runQuotaIndependence } from "./lib/checks/us3.js";

async function finish(report: Report, flags: any, states: TenantState[], opToken: string): Promise<never> {
  if (!flags.keep && opToken) {
    try {
      for (const st of states) await teardownTenant(opToken, st);
      const clean = await verifyClean(opToken, tenants(flags.tenants));
      report.add({ id: "US4.teardown", ref: ["SC-008"], story: "US4", name: "teardown leaves a clean state", passed: clean.clean, detail: clean.detail });
    } catch (e: any) {
      report.add({ id: "US4.teardown", ref: ["SC-008"], story: "US4", name: "teardown leaves a clean state", passed: false, detail: String(e?.message || e) });
    }
  }
  const code = report.finalize(flags.report);
  process.exit(code);
}

async function main(): Promise<void> {
  const flags = parseFlags(process.argv.slice(2));
  const ten = tenants(flags.tenants);
  const report = new Report();
  report.environment = {
    sandbox_runtime: process.env.MCP_SANDBOX_RUNTIME || "gvisor",
    aws_endpoint: CONFIG.awsEndpoint,
    tenants: ten.map((t) => t.slug),
    egress_network: process.env.MCP_SANDBOX_EGRESS_NETWORK || "mcp-sandbox-egress",
  };

  const states: TenantState[] = [];
  let opToken = "";
  try {
    opToken = await operatorToken();

    // FR-018 gate — must pass before any proof claim
    const pfOk = await runPreflights(report, ten);
    if (!pfOk) return await finish(report, flags, states, opToken);

    // setup
    for (const t of ten) await provisionTenant(opToken, t);
    for (const t of ten) states.push(await setupTenant(t));

    // proofs
    await runUS1(report, states);
    await runUS2(report, states);
    await runEgress(report);
    if (!flags.noStress) {
      await runUS3(report, states, flags);
      await runQuotaIndependence(report, states);
    }
  } catch (e: any) {
    report.add({ id: "harness.error", ref: [], story: "US4", name: "harness ran without a fatal error", passed: false, detail: String(e?.stack || e?.message || e) });
  }
  return await finish(report, flags, states, opToken);
}

main();
