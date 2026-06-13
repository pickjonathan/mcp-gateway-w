import type { CreateServerInput } from '../../api/servers'

/**
 * JSON Schema for the standard `mcpServers` config (the shape used by Claude
 * Desktop / VS Code / Cursor). Drives Monaco's squiggles/hover/autocomplete and
 * is mirrored to `public/mcp-servers.schema.json` for editing configs in VS Code.
 * Keep the two in sync.
 */
export const mcpServersJsonSchema = {
  $schema: 'http://json-schema.org/draft-07/schema#',
  $id: 'https://mcp.example.com/schemas/mcp-servers.schema.json',
  title: 'MCP servers configuration',
  type: 'object',
  required: ['mcpServers'],
  additionalProperties: false,
  properties: {
    mcpServers: {
      type: 'object',
      description: 'Map of server name → server definition.',
      minProperties: 1,
      additionalProperties: {
        type: 'object',
        additionalProperties: false,
        properties: {
          command: { type: 'string', description: 'Executable for a stdio (sandboxed) server, e.g. "uvx" or "npx".' },
          args: { type: 'array', items: { type: 'string' }, description: 'Arguments passed to the command.' },
          env: {
            type: 'object',
            additionalProperties: { type: 'string' },
            description: 'Environment variables. Stored as server config (visible to admins) — use the Credentials tab for secrets.',
          },
          url: { type: 'string', description: 'Endpoint URL for a remote (HTTP/SSE) server.' },
          type: { enum: ['stdio', 'http', 'sse'], description: 'Optional explicit transport. Inferred from command/url when omitted.' },
          headers: { type: 'object', additionalProperties: { type: 'string' }, description: 'Headers for a remote server (not yet applied by the console).' },
          disabled: { type: 'boolean', description: 'When true the server is created disabled.' },
          autoApprove: { type: 'array', items: { type: 'string' }, description: 'Ignored — the gateway has no per-tool auto-approve.' },
        },
        oneOf: [{ required: ['command'] }, { required: ['url'] }],
      },
    },
  },
} as const

/** A starter document shown when the JSON editor opens empty. */
export const mcpServersTemplate = `{
  "mcpServers": {
    "example-stdio": {
      "command": "uvx",
      "args": ["awslabs.aws-api-mcp-server@latest"],
      "env": { "AWS_REGION": "us-east-1" },
      "disabled": false
    }
  }
}
`

export interface ParseResult {
  inputs: CreateServerInput[]
  errors: string[]
  warnings: string[]
}

function isStringRecord(v: unknown): v is Record<string, string> {
  return (
    typeof v === 'object' &&
    v !== null &&
    !Array.isArray(v) &&
    Object.values(v as Record<string, unknown>).every((x) => typeof x === 'string')
  )
}

/**
 * Parse + validate a standard `mcpServers` config and map each entry to a
 * control-plane CreateServerInput. Pure (no Monaco) so it is unit-testable and
 * also serves as the authoritative check at submit time.
 *
 * - `command` ⇒ stdio server; `url` ⇒ remote_http. Exactly one is required.
 * - `disabled: true` ⇒ created disabled. `env` is carried for stdio servers.
 * - Unsupported keys (`autoApprove`, remote `headers`/`env`) are surfaced as
 *   warnings, not hard errors, so a real-world config still imports.
 */
export function parseMcpServersConfig(text: string): ParseResult {
  const errors: string[] = []
  const warnings: string[] = []
  const inputs: CreateServerInput[] = []

  if (!text.trim()) return { inputs, errors: ['Paste an mcpServers JSON config.'], warnings }

  let root: unknown
  try {
    root = JSON.parse(text)
  } catch (e) {
    return { inputs, errors: [`Invalid JSON: ${e instanceof Error ? e.message : String(e)}`], warnings }
  }

  if (typeof root !== 'object' || root === null || Array.isArray(root)) {
    return { inputs, errors: ['Top level must be an object with an "mcpServers" property.'], warnings }
  }
  const servers = (root as Record<string, unknown>).mcpServers
  if (typeof servers !== 'object' || servers === null || Array.isArray(servers)) {
    return { inputs, errors: ['Missing "mcpServers" object.'], warnings }
  }
  const entries = Object.entries(servers as Record<string, unknown>)
  if (entries.length === 0) return { inputs, errors: ['"mcpServers" has no entries.'], warnings }

  for (const [name, raw] of entries) {
    const at = `"${name}"`
    if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) {
      errors.push(`${at}: must be an object.`)
      continue
    }
    const cfg = raw as Record<string, unknown>
    const hasCommand = typeof cfg.command === 'string' && cfg.command.trim() !== ''
    const urlRaw = typeof cfg.url === 'string' ? cfg.url : undefined
    const hasUrl = typeof urlRaw === 'string' && urlRaw.trim() !== ''

    if (!hasCommand && !hasUrl) {
      errors.push(`${at}: needs a "command" (stdio) or "url" (remote).`)
      continue
    }
    if (hasCommand && hasUrl) {
      errors.push(`${at}: has both "command" and "url" — keep one.`)
      continue
    }

    let args: string[] | undefined
    if (cfg.args !== undefined) {
      if (!Array.isArray(cfg.args) || !cfg.args.every((a) => typeof a === 'string')) {
        errors.push(`${at}: "args" must be an array of strings.`)
        continue
      }
      args = cfg.args as string[]
    }

    let env: Record<string, string> | undefined
    if (cfg.env !== undefined) {
      if (!isStringRecord(cfg.env)) {
        errors.push(`${at}: "env" must be an object of string values.`)
        continue
      }
      env = cfg.env
    }

    let enabled = true
    if (cfg.disabled !== undefined) {
      if (typeof cfg.disabled !== 'boolean') {
        errors.push(`${at}: "disabled" must be a boolean.`)
        continue
      }
      enabled = !cfg.disabled
    }

    if (Array.isArray(cfg.autoApprove) && cfg.autoApprove.length > 0) {
      warnings.push(`${at}: "autoApprove" is not supported and will be ignored.`)
    }

    const base = { slug: name, credential_mode: 'none' as const, allowed_roles: [] as string[], enabled }
    if (hasCommand) {
      if (env && Object.keys(env).length > 0) {
        warnings.push(`${at}: "env" values are stored as server config (visible to admins) — use the Credentials tab for secrets.`)
      }
      inputs.push({
        ...base,
        type: 'stdio',
        command: (cfg.command as string).trim(),
        ...(args ? { args } : {}),
        ...(env ? { env } : {}),
      })
    } else {
      if (env) warnings.push(`${at}: "env" is ignored for remote servers.`)
      if (cfg.headers) warnings.push(`${at}: "headers" are not applied by the console yet.`)
      inputs.push({ ...base, type: 'remote_http', endpoint_url: (urlRaw as string).trim() })
    }
  }

  return { inputs, errors, warnings }
}
