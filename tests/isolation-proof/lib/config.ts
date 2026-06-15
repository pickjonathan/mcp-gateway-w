// Central configuration for the isolation proof. All values overridable via env.
// Model: ONE ministack hosting TWO AWS accounts (one per realm). Each realm has a
// dedicated user with a role that permits the realm's AWS MCP. We prove each
// realm's user is routed to its own MCP → its own AWS account.

export interface TenantCfg {
  slug: string;
  displayName: string;
  // Admin identity (control-plane: register server, set creds, query audit).
  adminEmail: string;
  adminPassword: string;
  // This realm's AWS account on the shared ministack (12-digit key = account id).
  accountId: string;
  secretAccessKey: string;
  bucket: string; // this realm's bucket, created under its account
  marker: string; // unique object key this realm's MCP writes (proof of routing)
  // Dedicated data-plane user + the role that grants access to the AWS MCP.
  role: string;
  user: string;
  password: string;
}

export function env(k: string, def = ""): string {
  return process.env[k] ?? def;
}

export const CONFIG = {
  controlPlane: env("MCP_PROOF_CONTROL_PLANE", "http://localhost:8090"),
  keycloak: env("MCP_PROOF_KEYCLOAK", "http://localhost:8081"),
  prometheus: env("MCP_PROOF_PROMETHEUS", "http://localhost:9090"),
  awsEndpointHost: env("MCP_PROOF_AWS_ENDPOINT", "http://localhost:4566"), // harness view
  awsEndpointInternal: env("MCP_PROOF_AWS_ENDPOINT_INTERNAL", "http://ministack:4566"), // sandbox view
  ministackContainer: env("MCP_PROOF_MINISTACK_CONTAINER", "mcp-runtime-dev-ministack-1"),
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
  kcAdmin: {
    realm: env("MCP_PROOF_KC_ADMIN_REALM", "master"),
    clientId: env("MCP_KEYCLOAK_ADMIN_CLIENT_ID", "mcp-provisioner"),
    clientSecret: env("MCP_KEYCLOAK_ADMIN_SECRET", ""),
    fallbackUser: env("MCP_PROOF_KC_ADMIN_USER", "admin"),
    fallbackPassword: env("MCP_PROOF_KC_ADMIN_PW", "admin"),
  },
  dataPlaneClient: env("MCP_PROOF_MCP_CLIENT", "mcp-client"),
  adminClient: env("MCP_PROOF_ADMIN_CLIENT", "mcp-admin-console"),
  awsServerCommand: env("MCP_PROOF_AWS_MCP_COMMAND", "awslabs.aws-api-mcp-server"),
  mcpRole: env("MCP_PROOF_ROLE", "mcp-user"),
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
    adminPassword: env(`MCP_PROOF_${slug.toUpperCase()}_ADMIN_PW`, `Proof-${slug}-admin-1!`),
    // distinct AWS account per realm: alpha->111111111111, beta->222222222222
    accountId: String((i + 1) % 10).repeat(12),
    secretAccessKey: `proof-secret-${slug}`,
    bucket: `${slug}-data`,
    marker: `proof/${slug}-via-gateway.txt`,
    role: CONFIG.mcpRole,
    user: env(`MCP_PROOF_${slug.toUpperCase()}_USER`, `${slug}-user`),
    password: env(`MCP_PROOF_${slug.toUpperCase()}_PW`, `Proof-${slug}-user-1!`),
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
