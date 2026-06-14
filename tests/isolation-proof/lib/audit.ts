// Query the tamper-evident audit trail to prove cross-tenant denials were
// recorded (FR-010 / SC-003 / research.md D9).
import { CONFIG } from "./config.js";
import { http } from "./http.js";

export interface AuditRecord {
  Action: string;
  Actor: string;
  Target: string;
  OrgID: string;
  Time?: string;
  Metadata?: Record<string, string>;
}

export async function listAudit(org: string, adminToken: string): Promise<AuditRecord[]> {
  const r = await http("GET", `${CONFIG.controlPlane}/v1/orgs/${org}/audit`, { token: adminToken });
  return Array.isArray(r.json) ? r.json : [];
}

/** A denial record exists for `org` (auth.denied / authz.denied by default). */
export async function hasDenial(
  org: string,
  adminToken: string,
  actions: string[] = ["auth.denied", "authz.denied"],
): Promise<{ found: boolean; records: AuditRecord[] }> {
  const records = await listAudit(org, adminToken);
  const found = records.some((r) => actions.includes(r.Action));
  return { found, records: records.filter((r) => actions.includes(r.Action)) };
}

/** No audit record's metadata contains any of the provided secret values. */
export function auditHasNoSecret(records: AuditRecord[], secrets: string[]): boolean {
  const blob = JSON.stringify(records);
  return !secrets.some((s) => s && blob.includes(s));
}
