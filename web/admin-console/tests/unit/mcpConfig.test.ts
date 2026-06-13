import { describe, it, expect } from 'vitest'
import { parseMcpServersConfig } from '../../src/features/servers/mcpConfig'

// The exact config from the feature request.
const EXAMPLE = `{
  "mcpServers": {
    "awslabs.aws-api-mcp-server": {
      "command": "uvx",
      "args": ["awslabs.aws-api-mcp-server@latest"],
      "env": { "AWS_REGION": "us-east-1" },
      "disabled": false,
      "autoApprove": []
    }
  }
}`

describe('parseMcpServersConfig', () => {
  it('maps the standard stdio example to a CreateServerInput', () => {
    const { inputs, errors, warnings } = parseMcpServersConfig(EXAMPLE)
    expect(errors).toEqual([])
    expect(inputs).toHaveLength(1)
    expect(inputs[0]).toEqual({
      slug: 'awslabs.aws-api-mcp-server',
      type: 'stdio',
      command: 'uvx',
      args: ['awslabs.aws-api-mcp-server@latest'],
      env: { AWS_REGION: 'us-east-1' },
      credential_mode: 'none',
      allowed_roles: [],
      enabled: true,
    })
    // env present → a "secrets go in Credentials" warning, but not a blocker.
    expect(warnings.some((w) => w.includes('env'))).toBe(true)
  })

  it('imports every entry (bulk)', () => {
    const cfg = JSON.stringify({
      mcpServers: {
        a: { command: 'npx', args: ['-y', 'pkg'] },
        b: { url: 'https://mcp.example.org/sse' },
      },
    })
    const { inputs, errors } = parseMcpServersConfig(cfg)
    expect(errors).toEqual([])
    expect(inputs.map((i) => i.slug)).toEqual(['a', 'b'])
    expect(inputs[0].type).toBe('stdio')
    expect(inputs[1]).toMatchObject({ type: 'remote_http', endpoint_url: 'https://mcp.example.org/sse' })
  })

  it('maps disabled:true to enabled:false', () => {
    const { inputs } = parseMcpServersConfig(JSON.stringify({ mcpServers: { x: { command: 'uvx', disabled: true } } }))
    expect(inputs[0].enabled).toBe(false)
  })

  it('rejects invalid JSON', () => {
    const { errors, inputs } = parseMcpServersConfig('{ not json')
    expect(inputs).toEqual([])
    expect(errors[0]).toMatch(/Invalid JSON/)
  })

  it('requires an mcpServers object', () => {
    expect(parseMcpServersConfig('{}').errors[0]).toMatch(/mcpServers/)
    expect(parseMcpServersConfig(JSON.stringify({ mcpServers: {} })).errors[0]).toMatch(/no entries/)
  })

  it('requires command or url per entry', () => {
    const { errors } = parseMcpServersConfig(JSON.stringify({ mcpServers: { bad: { args: ['x'] } } }))
    expect(errors[0]).toMatch(/needs a "command".*or "url"/)
  })

  it('rejects a wrong args type', () => {
    const { errors } = parseMcpServersConfig(JSON.stringify({ mcpServers: { x: { command: 'c', args: 'nope' } } }))
    expect(errors[0]).toMatch(/"args" must be an array/)
  })

  it('warns (does not fail) on autoApprove', () => {
    const { errors, warnings } = parseMcpServersConfig(
      JSON.stringify({ mcpServers: { x: { command: 'c', autoApprove: ['tool'] } } }),
    )
    expect(errors).toEqual([])
    expect(warnings.some((w) => w.includes('autoApprove'))).toBe(true)
  })
})
