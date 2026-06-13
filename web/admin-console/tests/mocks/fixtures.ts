// Sample control-plane responses for component/contract tests (shapes per the
// 001 API + the consumed contract).
export const servers = [
  {
    id: 'srv_a1',
    slug: 'sequential-thinking',
    type: 'stdio',
    enabled: true,
    health: 'healthy',
    credential_mode: 'none',
    allowed_roles: [],
    command: 'npx',
    args: ['-y', '@modelcontextprotocol/server-sequential-thinking'],
    created_at: '2026-06-13T10:00:00Z',
  },
  {
    id: 'srv_b2',
    slug: 'weather',
    type: 'remote_http',
    enabled: true,
    health: 'healthy',
    credential_mode: 'org_shared',
    credential_set: true,
    allowed_roles: ['engineers'],
    endpoint_url: 'https://mcp.example.org/weather',
    created_at: '2026-06-13T11:00:00Z',
  },
]

export const auditEvents = [
  { seq: 3, time: '2026-06-13T12:02:00Z', actor: 'user-1', action: 'server.create', target: 'weather', metadata: {} },
  { seq: 2, time: '2026-06-13T12:01:00Z', actor: 'unknown', action: 'auth.denied', target: '/mcp', metadata: { reason: 'invalid_token' } },
  { seq: 1, time: '2026-06-13T12:00:00Z', actor: 'admin', action: 'credentials.put', target: 'weather', metadata: { scope: 'org' } },
]

export const quotas = { org_per_min: 600, user_per_min: 60 }
