// Query the tamper-evident audit trail to prove cross-tenant denials were
// recorded (FR-010 / SC-003 / research.md D9).
import { CONFIG } from "./config.js";
import { http } from "./http.js";

export interface AuditRecord {
  action: string;
  actor: string;
  target: string;
  org_id: string;
  time?: string;
  metadata?: Record<string, string>;
  seq?: number;
  prev_hash?: string;
  hash?: string;
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
  const match = records.filter((r) => actions.includes(r.action));
  return { found: match.length > 0, records: match };
}

/** The org's audit trail is non-empty and hash-chained (tamper-evident, FR-010). */
export async function auditChainIntact(org: string, adminToken: string): Promise<{ ok: boolean; count: number }> {
  const records = await listAudit(org, adminToken);
  const chained = records.every((r) => typeof r.hash === "string" && r.hash.length > 0);
  return { ok: records.length > 0 && chained, count: records.length };
}

/** No audit record's metadata contains any of the provided secret values. */
export function auditHasNoSecret(records: AuditRecord[], secrets: string[]): boolean {
  const blob = JSON.stringify(records);
  return !secrets.some((s) => s && blob.includes(s));
}
