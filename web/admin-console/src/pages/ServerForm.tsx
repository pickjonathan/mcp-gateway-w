import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  useServer,
  useCreateServer,
  useUpdateServer,
  type CreateServerInput,
  type CredentialMode,
  type ServerType,
} from '../api/servers'
import { ApiError } from '../api/client'
import { Button } from '../design-system/components/core/Button'
import { Tile } from '../design-system/components/core/Tile'
import { TextInput } from '../design-system/components/forms/TextInput'
import { Select } from '../design-system/components/forms/Select'
import { Toggle } from '../design-system/components/forms/Toggle'
import { InlineNotification } from '../design-system/components/feedback/InlineNotification'
import { useNotify } from '../app/feedback/notifications'

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
    <section style={{ maxWidth: 640 }}>
      <h1 style={{ fontSize: 'var(--type-scale-06)', marginBottom: 'var(--spacing-05)' }}>
        {editing ? 'Edit server' : 'Add server'}
      </h1>
      <Tile>
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
      </Tile>
    </section>
  )
}
