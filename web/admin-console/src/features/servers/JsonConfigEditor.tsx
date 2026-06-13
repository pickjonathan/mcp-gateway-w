import { useEffect } from 'react'
import Editor, { loader } from '@monaco-editor/react'
// Import only the editor API + the JSON language (not the full monaco-editor
// bundle with every language grammar) to keep the lazy chunk lean.
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api'
import 'monaco-editor/esm/vs/language/json/monaco.contribution'
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import jsonWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker'
import { mcpServersJsonSchema } from './mcpConfig'

// Bundle Monaco locally (no CDN) and spin up only the base + JSON language
// workers. This whole module is lazy-loaded by ServerForm, so it (and Monaco)
// only enter the bundle execution path when the JSON editor is actually opened.
Object.assign(globalThis, {
  MonacoEnvironment: {
    getWorker: (_workerId: string, label: string) =>
      label === 'json' ? new jsonWorker() : new editorWorker(),
  },
})
loader.config({ monaco })

const SCHEMA_URI = 'https://mcp.example.com/schemas/mcp-servers.schema.json'

interface Props {
  value: string
  onChange: (value: string) => void
  /** Reports the number of error-severity markers Monaco currently shows. */
  onValidate?: (errorCount: number) => void
}

/**
 * Monaco (the VS Code editor engine) bound to the mcpServers JSON Schema, so
 * the admin gets live squiggles, hover docs, and autocomplete while pasting a
 * server config. Default export so ServerForm can React.lazy() it.
 */
export default function JsonConfigEditor({ value, onChange, onValidate }: Props) {
  useEffect(() => {
    monaco.languages.json.jsonDefaults.setDiagnosticsOptions({
      validate: true,
      enableSchemaRequest: false,
      schemas: [{ uri: SCHEMA_URI, fileMatch: ['*'], schema: mcpServersJsonSchema }],
    })
  }, [])

  return (
    <div style={{ border: '1px solid var(--border-strong-01)', borderRadius: 2, overflow: 'hidden' }}>
      <Editor
        height="320px"
        defaultLanguage="json"
        theme="light"
        value={value}
        onChange={(v) => onChange(v ?? '')}
        onValidate={(markers) =>
          onValidate?.(markers.filter((m) => m.severity === monaco.MarkerSeverity.Error).length)
        }
        options={{
          minimap: { enabled: false },
          lineNumbers: 'on',
          scrollBeyondLastLine: false,
          fontSize: 13,
          tabSize: 2,
          automaticLayout: true,
          formatOnPaste: true,
          renderLineHighlight: 'none',
        }}
      />
    </div>
  )
}
