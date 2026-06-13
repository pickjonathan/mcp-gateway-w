import { lazy, Suspense, useEffect, useMemo, useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  useServer,
  useCreateServer,
  useUpdateServer,
  useImportServers,
  type CreateServerInput,
  type CredentialMode,
  type ImportResult,
  type ServerType,
} from '../api/servers'
import { ApiError } from '../api/client'
import { parseMcpServersConfig, mcpServersTemplate } from '../features/servers/mcpConfig'
import { Button } from '../design-system/components/core/Button'
import { Tile } from '../design-system/components/core/Tile'
import { TextInput } from '../design-system/components/forms/TextInput'
import { Select } from '../design-system/components/forms/Select'
import { Toggle } from '../design-system/components/forms/Toggle'
import { InlineNotification } from '../design-system/components/feedback/InlineNotification'
import { Loading } from '../app/feedback/states'
import { useNotify } from '../app/feedback/notifications'

// Monaco is heavy; load it only when the JSON tab is opened (keeps it out of
// the form-mode path and out of jsdom tests entirely).
const JsonConfigEditor = lazy(() => import('../features/servers/JsonConfigEditor'))

interface Errors {
  slug?: string
  endpoint?: string
  command?: string
  form?: string
}

export function ServerForm() {
  const { id } = useParams()
  const editing = Boolean(id)
  const navigate = useNavigate()
  const notify = useNotify()
  const { data: existing } = useServer(id ?? '')
  const create = useCreateServer()
  const update = useUpdateServer()

  const [slug, setSlug] = useState('')
  const [type, setType] = useState<ServerType>('remote_http')
  const [endpoint, setEndpoint] = useState('')
  const [command, setCommand] = useState('')
  const [argsText, setArgsText] = useState('')
  const [credMode, setCredMode] = useState<CredentialMode>('none')
  const [rolesText, setRolesText] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [errors, setErrors] = useState<Errors>({})

  // JSON-import mode (Add only): paste a standard mcpServers config.
  const [mode, setMode] = useState<'form' | 'json'>('form')
  const [jsonText, setJsonText] = useState(mcpServersTemplate)
  const [importResults, setImportResults] = useState<ImportResult[] | null>(null)
  const importMut = useImportServers()
  const parsed = useMemo(() => parseMcpServersConfig(jsonText), [jsonText])

  function onImport() {
    setImportResults(null)
    if (parsed.errors.length > 0 || parsed.inputs.length === 0) return
    importMut.mutate(parsed.inputs, {
      onSuccess: (results) => {
        setImportResults(results)
        const ok = results.filter((r) => r.ok).length
        const failed = results.length - ok
        if (failed === 0) {
          notify('success', `Imported ${ok} server${ok === 1 ? '' : 's'}`)
          navigate('/servers')
        } else {
          notify('error', `Imported ${ok}/${results.length}`, `${failed} failed — see details below`)
        }
      },
      onError: (e) => notify('error', 'Import failed', e instanceof Error ? e.message : String(e)),
    })
  }

  useEffect(() => {
    if (!existing) return
    setSlug(existing.slug)
    setType(existing.type)
    setEndpoint(existing.endpoint_url ?? '')
    setCommand(existing.command ?? '')
    setArgsText((existing.args ?? []).join(' '))
    setCredMode(existing.credential_mode)
    setRolesText(existing.allowed_roles.join(', '))
    setEnabled(existing.enabled)
  }, [existing])

  function validate(): boolean {
    const e: Errors = {}
    if (!slug.trim()) e.slug = 'Name is required'
    if (type === 'remote_http' && !endpoint.trim()) e.endpoint = 'Endpoint URL is required'
    if (type === 'stdio' && !command.trim()) e.command = 'Command is required'
    setErrors(e)
    return Object.keys(e).length === 0
  }

  function onSubmit(ev: FormEvent) {
    ev.preventDefault()
    if (!validate()) return
    const input: CreateServerInput = {
      slug: slug.trim(),
      type,
      credential_mode: credMode,
      allowed_roles: rolesText.split(',').map((r) => r.trim()).filter(Boolean),
      enabled,
      ...(type === 'remote_http'
        ? { endpoint_url: endpoint.trim() }
        : { command: command.trim(), args: argsText.split(/\s+/).filter(Boolean) }),
    }
    const onError = (err: unknown) => {
      if (err instanceof ApiError && (err.status === 409 || err.status === 422)) {
        setErrors((x) => ({ ...x, slug: 'That name is already in use' }))
      } else {
        const msg = err instanceof Error ? err.message : 'Save failed'
        setErrors((x) => ({ ...x, form: msg }))
        notify('error', 'Save failed', msg)
      }
    }
    if (editing && id) {
      update.mutate(
        { id, input },
        { onSuccess: () => { notify('success', 'Server updated'); navigate(`/servers/${id}`) }, onError },
      )
    } else {
      create.mutate(input, {
        onSuccess: () => { notify('success', 'Server created'); navigate('/servers') },
        onError,
      })
    }
  }

  const busy = create.isPending || update.isPending

  return (
    <section style={{ maxWidth: mode === 'json' ? 860 : 640 }}>
      <h1 style={{ fontSize: 'var(--type-scale-06)', marginBottom: 'var(--spacing-05)' }}>
        {editing ? 'Edit server' : 'Add server'}
      </h1>

      {!editing && (
        <div role="tablist" aria-label="Add method" style={{ display: 'flex', gap: 4, marginBottom: 'var(--spacing-05)' }}>
          <Button kind={mode === 'form' ? 'primary' : 'secondary'} size="sm" type="button" onClick={() => setMode('form')}>
            Form
          </Button>
          <Button kind={mode === 'json' ? 'primary' : 'secondary'} size="sm" type="button" onClick={() => setMode('json')}>
            Paste JSON
          </Button>
        </div>
      )}

      <Tile>
        {mode === 'form' ? (
        <form onSubmit={onSubmit} style={{ display: 'grid', gap: 'var(--spacing-05)' }}>
          {errors.form && <InlineNotification kind="error" title="Could not save" subtitle={errors.form} />}

          <TextInput
            id="slug"
            label="Name (slug)"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            invalid={Boolean(errors.slug)}
            invalidText={errors.slug}
            placeholder="e.g. sequential-thinking"
          />

          <Select id="type" label="Type" value={type} onChange={(e) => setType(e.target.value as ServerType)}>
            <option value="remote_http">Remote (HTTP)</option>
            <option value="stdio">Stdio (sandboxed)</option>
          </Select>

          {type === 'remote_http' ? (
            <TextInput
              id="endpoint"
              label="Endpoint URL"
              value={endpoint}
              onChange={(e) => setEndpoint(e.target.value)}
              invalid={Boolean(errors.endpoint)}
              invalidText={errors.endpoint}
              placeholder="https://mcp.example.org/server"
            />
          ) : (
            <>
              <TextInput
                id="command"
                label="Command"
                value={command}
                onChange={(e) => setCommand(e.target.value)}
                invalid={Boolean(errors.command)}
                invalidText={errors.command}
                placeholder="npx"
              />
              <TextInput
                id="args"
                label="Arguments (space-separated)"
                value={argsText}
                onChange={(e) => setArgsText(e.target.value)}
                placeholder="-y @modelcontextprotocol/server-sequential-thinking"
              />
            </>
          )}

          <Select
            id="credMode"
            label="Credentials"
            value={credMode}
            onChange={(e) => setCredMode(e.target.value as CredentialMode)}
          >
            <option value="none">None</option>
            <option value="org_shared">Organization-shared</option>
            <option value="per_user">Per-user</option>
          </Select>

          <TextInput
            id="roles"
            label="Allowed roles (comma-separated; blank = all members)"
            value={rolesText}
            onChange={(e) => setRolesText(e.target.value)}
            placeholder="engineers, admins"
          />

          <Toggle id="enabled" label="Enabled" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />

          <div style={{ display: 'flex', gap: 'var(--spacing-04)' }}>
            <Button kind="primary" type="submit" disabled={busy}>
              {editing ? 'Save changes' : 'Create server'}
            </Button>
            <Button kind="secondary" type="button" onClick={() => navigate('/servers')} disabled={busy}>
              Cancel
            </Button>
          </div>
        </form>
        ) : (
          <div style={{ display: 'grid', gap: 'var(--spacing-05)' }}>
            <p style={{ color: 'var(--text-secondary)', fontSize: 'var(--type-scale-02)', margin: 0 }}>
              Paste a standard <code>mcpServers</code> config (Claude Desktop / VS&nbsp;Code format). It is validated
              against the MCP schema as you type; every entry is imported.
            </p>

            <Suspense fallback={<Loading label="Loading editor…" />}>
              <JsonConfigEditor
                value={jsonText}
                onChange={(v) => {
                  setJsonText(v)
                  setImportResults(null)
                }}
              />
            </Suspense>

            {parsed.errors.length > 0 ? (
              <InlineNotification kind="error" title="Fix before importing" subtitle={parsed.errors.join(' · ')} hideClose />
            ) : (
              <InlineNotification
                kind="success"
                title={`${parsed.inputs.length} server${parsed.inputs.length === 1 ? '' : 's'} ready to import`}
                subtitle={parsed.inputs.map((i) => `${i.slug} (${i.type === 'stdio' ? 'stdio' : 'remote'})`).join(', ')}
                hideClose
              />
            )}

            {parsed.warnings.map((w, i) => (
              <InlineNotification key={i} kind="warning" title="Note" subtitle={w} hideClose />
            ))}

            {importResults && (
              <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'grid', gap: 4 }}>
                {importResults.map((r) => (
                  <li
                    key={r.slug}
                    style={{
                      fontSize: 'var(--type-scale-02)',
                      color: r.ok ? 'var(--support-success, #198038)' : 'var(--support-error, #da1e28)',
                    }}
                  >
                    {r.ok ? '✓' : '✗'} {r.slug}
                    {r.error ? ` — ${r.error}` : ''}
                  </li>
                ))}
              </ul>
            )}

            <div style={{ display: 'flex', gap: 'var(--spacing-04)' }}>
              <Button
                kind="primary"
                type="button"
                disabled={parsed.errors.length > 0 || parsed.inputs.length === 0 || importMut.isPending}
                onClick={onImport}
              >
                {importMut.isPending
                  ? 'Importing…'
                  : `Import ${parsed.inputs.length} server${parsed.inputs.length === 1 ? '' : 's'}`}
              </Button>
              <Button kind="secondary" type="button" onClick={() => navigate('/servers')} disabled={importMut.isPending}>
                Cancel
              </Button>
            </div>
          </div>
        )}
      </Tile>
    </section>
  )
}
