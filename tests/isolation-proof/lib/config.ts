// Central configuration for the isolation proof. All values overridable via env
// so the harness runs against any environment; deterministic by default (SC-009).

export interface TenantCfg {
  slug: string;
  displayName: string;
  adminEmail: string;
  accountId: string; // 12-digit AWS account id == AWS_ACCESS_KEY_ID (ministack, research.md D1)
  secretAccessKey: string;
  bucket: string;
  user: string; // realm user the harness authenticates as (admin role)
  password: string; // known password the harness sets via the Keycloak Admin API
}

export function env(k: string, def = ""): string {
  return process.env[k] ?? def;
}

export const CONFIG = {
  controlPlane: env("MCP_PROOF_CONTROL_PLANE", "http://localhost:8090"),
  keycloak: env("MCP_PROOF_KEYCLOAK", "http://localhost:8081"),
  awsEndpoint: env("MCP_PROOF_AWS_ENDPOINT", "http://localhost:4566"), // harness/host view
  awsEndpointInternal: env("MCP_PROOF_AWS_ENDPOINT_INTERNAL", "http://ministack:4566"), // sandbox view
  baseDomain: env("MCP_PROOF_BASE_DOMAIN", "mcp.example.com"),
  gatewayPort: env("MCP_PROOF_GATEWAY_PORT", "8080"),
  region: env("MCP_PROOF_AWS_REGION", "us-east-1"),
  adminAudience: env("MCP_PROOF_ADMIN_AUDIENCE", "https://api.mcp.example.com"),
  platformRealm: env("MCP_PROOF_PLATFORM_REALM", "_platform"),
  operator: {
    client: env("MCP_PROOF_OPERATOR_CLIENT", "mcp-platform"),
    user: env("MCP_PROOF_OPERATOR", "operator"),
    password: env("MCP_PROOF_OPERATOR_PW", "operator"),
  },
  // Keycloak Admin API access (to set known test-user passwords/roles per realm).
  kcAdmin: {
    realm: env("MCP_PROOF_KC_ADMIN_REALM", "master"),
    clientId: env("MCP_KEYCLOAK_ADMIN_CLIENT_ID", "mcp-provisioner"),
    clientSecret: env("MCP_KEYCLOAK_ADMIN_SECRET", ""),
    fallbackUser: env("MCP_PROOF_KC_ADMIN_USER", "admin"),
    fallbackPassword: env("MCP_PROOF_KC_ADMIN_PW", "admin"),
  },
  dataPlaneClient: env("MCP_PROOF_MCP_CLIENT", "mcp-client"),
  adminClient: env("MCP_PROOF_ADMIN_CLIENT", "mcp-admin-console"),
  // The console-script command the pre-baked AWS MCP server is launched as
  // (deploy/sandbox-images/Dockerfile).
  awsServerCommand: env("MCP_PROOF_AWS_MCP_COMMAND", "awslabs.aws-api-mcp-server"),
  rateOrgPerMin: Number(env("MCP_PROOF_RATE_ORG_PER_MIN", "120")),
};

export function mcpUrl(slug: string): string {
  return `http://${slug}.${CONFIG.baseDomain}:${CONFIG.gatewayPort}/mcp`;
}

export function tenants(slugsCsv = "alpha,beta"): TenantCfg[] {
  const slugs = slugsCsv.split(",").map((s) => s.trim()).filter(Boolean);
  return slugs.map((slug, i) => ({
    slug,
    displayName: slug.charAt(0).toUpperCase() + slug.slice(1),
    adminEmail: `admin@${slug}.example`,
    // deterministic single-digit-repeat account id: alpha->111111111111, beta->222222222222
    accountId: String((i + 1) % 10).repeat(12),
    secretAccessKey: `proof-secret-${slug}`,
    bucket: `${slug}-data`,
    user: env(`MCP_PROOF_${slug.toUpperCase()}_USER`, `admin@${slug}.example`),
    password: env(`MCP_PROOF_${slug.toUpperCase()}_PW`, `Proof-${slug}-123!`),
  }));
}

export interface Flags {
  tenants: string;
  concurrency: number;
  stressSeconds: number;
  report: string;
  keep: boolean;
  noStress: boolean;
}

export function parseFlags(argv: string[]): Flags {
  const f: Flags = {
    tenants: "alpha,beta",
    concurrency: 10,
    stressSeconds: 60,
    report: "report.json",
    keep: false,
    noStress: false,
  };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--tenants") f.tenants = argv[++i];
    else if (a === "--concurrency") f.concurrency = Number(argv[++i]);
    else if (a === "--stress-seconds") f.stressSeconds = Number(argv[++i]);
    else if (a === "--report") f.report = argv[++i];
    else if (a === "--keep") f.keep = true;
    else if (a === "--no-stress") f.noStress = true;
  }
  return f;
}
