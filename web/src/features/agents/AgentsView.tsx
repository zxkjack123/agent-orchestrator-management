import { useState, useEffect } from 'react'
import { useProjectContext } from '@/app/ProjectContext'
import { StatusBadge } from '@/components/StatusBadge'
import { useAgents, useAgentInstructions } from './hooks'
import { sessionsApi } from '@/features/sessions/api'
import { useRoles, useClasses, useClassDetail, useSystemTemplate } from '@/features/roles/hooks'
import { rolesApi } from '@/features/roles/api'
import type { Agent, AddAgentForm, SpawnSessionForm, SpawnSessionResult } from './types'

// ─── helpers ──────────────────────────────────────────────────────────────────

function Empty({ message }: { message: string }) {
  return (
    <div className="h-full flex items-center justify-center text-sm text-gray-600">
      {message}
    </div>
  )
}

// ─── Add Agent Modal ─────────────────────────────────────────────────────────

interface AddAgentModalProps {
  projectId: string
  onClose: () => void
  onSubmit: (form: AddAgentForm) => Promise<unknown>
}

const RUNTIMES = ['claude', 'codex', 'gemini', 'kiro', 'generic']

function AddAgentModal({ projectId, onClose, onSubmit }: AddAgentModalProps) {
  const { data: roles = [] } = useRoles(projectId)
  const { data: classes = [] } = useClasses(projectId)
  const [form, setForm] = useState<AddAgentForm>({
    name: '',
    role: '',
    runtime: 'claude',
    model: '',
    enabled: true,
  })

  useEffect(() => {
    if (roles.length > 0 && !form.role) {
      setForm((f) => ({ ...f, role: roles[0].name }))
    }
  }, [roles])
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSaving(true)
    try {
      await onSubmit({ ...form, model: form.model || undefined })
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add agent')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-md p-6 shadow-xl">
        <h3 className="text-sm font-semibold text-gray-300 mb-4">Add Agent</h3>
        <form onSubmit={handleSubmit} className="space-y-3">
          <Field label="Name">
            <input
              required
              autoFocus
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              className={inputCls}
              placeholder="e.g. builder-1"
            />
          </Field>
          <Field label="Role">
            <select
              value={form.role}
              onChange={(e) => setForm({ ...form, role: e.target.value })}
              className={inputCls}
            >
              {roles.map((r) => (
                <option key={r.name} value={r.name}>{r.name}</option>
              ))}
            </select>
            {(() => {
              const selectedRole = roles.find((r) => r.name === form.role)
              const hint = selectedRole?.description || classes.find((c) => c.name === selectedRole?.class)?.description
              return hint ? (
                <p className="mt-1 text-[11px] text-gray-500 leading-snug">{hint}</p>
              ) : null
            })()}
          </Field>
          <Field label="Runtime">
            <select
              value={form.runtime}
              onChange={(e) => setForm({ ...form, runtime: e.target.value })}
              className={inputCls}
            >
              {RUNTIMES.map((r) => (
                <option key={r} value={r}>{r}</option>
              ))}
            </select>
          </Field>
          <Field label="Model (optional)">
            <input
              value={form.model ?? ''}
              onChange={(e) => setForm({ ...form, model: e.target.value })}
              className={inputCls}
              placeholder="e.g. claude-sonnet-4-5"
            />
          </Field>
          <Field label="">
            <label className="flex items-center gap-2 text-xs text-gray-400 cursor-pointer">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
                className="accent-accent"
              />
              Enabled
            </label>
          </Field>
          {error && <p className="text-xs text-accent-red">{error}</p>}
          <div className="flex justify-end gap-2 pt-2">
            <button type="button" onClick={onClose} className={secondaryBtn}>
              Cancel
            </button>
            <button type="submit" disabled={saving} className={primaryBtn}>
              {saving ? 'Adding…' : 'Add Agent'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Edit Model Modal ─────────────────────────────────────────────────────────

interface EditModelModalProps {
  agent: Agent
  onClose: () => void
  onSubmit: (model: string) => Promise<unknown>
}

function EditModelModal({ agent, onClose, onSubmit }: EditModelModalProps) {
  const [model, setModel] = useState(agent.model ?? '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSaving(true)
    try {
      await onSubmit(model)
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update model')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-sm p-6 shadow-xl">
        <h3 className="text-sm font-semibold text-gray-300 mb-4">
          Edit Model — <span className="text-accent">{agent.name}</span>
        </h3>
        <form onSubmit={handleSubmit} className="space-y-3">
          <Field label="Model">
            <input
              autoFocus
              value={model}
              onChange={(e) => setModel(e.target.value)}
              className={inputCls}
              placeholder="e.g. claude-sonnet-4-5"
            />
          </Field>
          {error && <p className="text-xs text-accent-red">{error}</p>}
          <div className="flex justify-end gap-2 pt-2">
            <button type="button" onClick={onClose} className={secondaryBtn}>
              Cancel
            </button>
            <button type="submit" disabled={saving} className={primaryBtn}>
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Spawn Session Modal ──────────────────────────────────────────────────────

interface SpawnModalProps {
  agent: Agent
  onClose: () => void
  onSubmit: (form: SpawnSessionForm) => Promise<SpawnSessionResult>
}

function SpawnSessionModal({ agent, onClose, onSubmit }: SpawnModalProps) {
  const [taskId, setTaskId] = useState('')
  const [mode, setMode] = useState('real')
  const [persistent, setPersistent] = useState(false)
  const [spawning, setSpawning] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<{ status: string; output: string } | null>(null)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSpawning(true)
    try {
      const res = await onSubmit({
        agent: agent.name,
        task_id: taskId || undefined,
        mode,
        ...(persistent ? { persistent: true } : {}),
      })
      setResult(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Spawn failed')
    } finally {
      setSpawning(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-md p-6 shadow-xl">
        <h3 className="text-sm font-semibold text-gray-300 mb-4">
          Spawn Session — <span className="text-accent">{agent.name}</span>
        </h3>
        {result ? (
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <StatusBadge status={result.status} />
              <span className="text-xs text-gray-400">Session spawned</span>
            </div>
            {result.output && (
              <pre className="text-xs text-gray-400 bg-surface p-3 rounded border border-surface-border overflow-auto max-h-40">
                {result.output}
              </pre>
            )}
            <div className="flex justify-end pt-2">
              <button onClick={onClose} className={primaryBtn}>Close</button>
            </div>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-3">
            <Field label="Task ID (optional)">
              <input
                value={taskId}
                onChange={(e) => setTaskId(e.target.value)}
                className={inputCls}
                placeholder="e.g. TASK-001"
              />
            </Field>
            <Field label="Mode">
              <select
                value={mode}
                onChange={(e) => setMode(e.target.value)}
                className={inputCls}
              >
                <option value="real">real</option>
                <option value="mock">mock</option>
              </select>
            </Field>
            <Field label="">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={persistent}
                  onChange={(e) => setPersistent(e.target.checked)}
                  className="accent-accent"
                />
                <span className="text-xs text-gray-400">Keep alive <span className="text-gray-600">(persistent — don't auto-stop on task completion)</span></span>
              </label>
            </Field>
            {error && <p className="text-xs text-accent-red">{error}</p>}
            <div className="flex justify-end gap-2 pt-2">
              <button type="button" onClick={onClose} className={secondaryBtn}>
                Cancel
              </button>
              <button type="submit" disabled={spawning} className={primaryBtn}>
                {spawning ? 'Spawning…' : 'Spawn'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}

// ─── Confirm Remove Modal ─────────────────────────────────────────────────────

interface ConfirmRemoveModalProps {
  agent: Agent
  onClose: () => void
  onConfirm: () => Promise<void>
}

function ConfirmRemoveModal({ agent, onClose, onConfirm }: ConfirmRemoveModalProps) {
  const [removing, setRemoving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleConfirm() {
    setError(null)
    setRemoving(true)
    try {
      await onConfirm()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove agent')
      setRemoving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-sm p-6 shadow-xl">
        <h3 className="text-sm font-semibold text-gray-300 mb-2">Remove Agent</h3>
        <p className="text-xs text-gray-400 mb-4">
          Remove <span className="text-accent-red font-medium">{agent.name}</span>? This cannot be undone.
        </p>
        {error && <p className="text-xs text-accent-red mb-3">{error}</p>}
        <div className="flex justify-end gap-2">
          <button onClick={onClose} className={secondaryBtn}>Cancel</button>
          <button onClick={handleConfirm} disabled={removing} className={dangerBtn}>
            {removing ? 'Removing…' : 'Remove'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ─── Shared UI primitives ────────────────────────────────────────────────────

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      {label && <label className="block text-xs text-gray-500 mb-1">{label}</label>}
      {children}
    </div>
  )
}

const inputCls =
  'w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-1.5 focus:outline-none focus:border-accent'
const primaryBtn =
  'px-3 py-1.5 text-xs bg-accent text-surface font-medium rounded hover:bg-accent/80 disabled:opacity-50 transition-colors'
const secondaryBtn =
  'px-3 py-1.5 text-xs bg-surface border border-surface-border text-gray-400 rounded hover:text-gray-300 transition-colors'
const dangerBtn =
  'px-3 py-1.5 text-xs bg-accent-red/20 border border-accent-red/40 text-accent-red font-medium rounded hover:bg-accent-red/30 disabled:opacity-50 transition-colors'

// ─── Custom Instructions Panel ───────────────────────────────────────────────

interface CustomInstructionsPanelProps {
  projectId: string
  agent: Agent
}

function CustomInstructionsPanel({ projectId, agent }: CustomInstructionsPanelProps) {
  const { instructions, isLoading, setInstructions, isSaving, saveError, saveResult } =
    useAgentInstructions(projectId, agent.name)
  const [draft, setDraft] = useState<string | null>(null)
  const [feedback, setFeedback] = useState<string | null>(null)
  const [activeWarning, setActiveWarning] = useState(false)
  const [respawning, setRespawning] = useState(false)

  useEffect(() => {
    if (draft === null && !isLoading) {
      setDraft(instructions)
    }
  }, [instructions, isLoading, draft])

  useEffect(() => {
    if (saveResult) {
      if (saveResult.active_session) {
        setActiveWarning(true)
        setFeedback(null)
      } else {
        setFeedback('Saved')
        setActiveWarning(false)
        const t = setTimeout(() => setFeedback(null), 2000)
        return () => clearTimeout(t)
      }
    }
  }, [saveResult])

  async function handleSave() {
    if (draft === null) return
    setFeedback(null)
    setActiveWarning(false)
    await setInstructions(draft)
  }

  async function handleClear() {
    setDraft('')
    setFeedback(null)
    setActiveWarning(false)
    await setInstructions('')
  }

  async function handleStopAndRespawn() {
    setRespawning(true)
    try {
      const sessions = await sessionsApi.list(projectId, true)
      const agentSessions = sessions.filter((s) => s.agent_name === agent.name)
      await Promise.all(agentSessions.map((s) => sessionsApi.stop(projectId, s.id)))
      await sessionsApi.spawn(projectId, agent.name, 'real')
      setActiveWarning(false)
      setFeedback('Respawned')
      const t = setTimeout(() => setFeedback(null), 3000)
      return () => clearTimeout(t)
    } catch (err) {
      setFeedback(err instanceof Error ? err.message : 'Respawn failed')
    } finally {
      setRespawning(false)
    }
  }

  return (
    <div className="px-4 py-3 bg-surface border-t border-surface-border">
      <p className="text-xs font-medium text-gray-500 mb-2">Custom Instructions</p>
      {isLoading ? (
        <p className="text-xs text-gray-600">Loading…</p>
      ) : (
        <div className="space-y-2">
          <textarea
            value={draft ?? ''}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="Add custom instructions for this agent…"
            rows={4}
            className="w-full bg-surface-raised border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent resize-y font-mono"
          />
          {saveError && (
            <p className="text-xs text-accent-red">
              {saveError instanceof Error ? saveError.message : 'Save failed'}
            </p>
          )}
          {activeWarning && (
            <div className="flex items-start gap-2 rounded border border-accent-yellow/40 bg-accent-yellow/10 px-3 py-2 text-xs text-accent-yellow">
              <span className="flex-1">
                <span className="font-medium">{agent.name}</span> is currently active — changes will take effect after you stop and respawn.
              </span>
              <button
                onClick={handleStopAndRespawn}
                disabled={respawning}
                className="shrink-0 px-2 py-0.5 bg-accent-yellow/20 border border-accent-yellow/40 rounded hover:bg-accent-yellow/30 disabled:opacity-50 transition-colors"
              >
                {respawning ? 'Working…' : 'Stop & Respawn'}
              </button>
            </div>
          )}
          <div className="flex items-center gap-2">
            <button
              onClick={handleSave}
              disabled={isSaving || draft === null}
              className={primaryBtn}
            >
              {isSaving ? 'Saving…' : 'Save'}
            </button>
            <button
              onClick={handleClear}
              disabled={isSaving}
              className={secondaryBtn}
            >
              Clear
            </button>
            {feedback && (
              <span className="text-xs text-accent-green">{feedback}</span>
            )}
          </div>
        </div>
      )}
    </div>
  )
}


// ─── Role Template Panel ─────────────────────────────────────────────────────

interface RoleTemplatePanelProps {
  projectId: string
  agent: Agent
}

function RoleTemplatePanel({ projectId, agent }: RoleTemplatePanelProps) {
  const { data: roles = [] } = useRoles(projectId)
  const roleClass = roles.find((r) => r.name === agent.role)?.class ?? null
  const { data: detail, isLoading } = useClassDetail(projectId, roleClass)
  const [draft, setDraft] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [feedback, setFeedback] = useState<string | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [previewing, setPreviewing] = useState(false)

  useEffect(() => {
    if (draft === null && detail) setDraft(detail.content)
  }, [detail, draft])

  async function handleSave() {
    if (!roleClass || !draft) return
    setSaving(true)
    setFeedback(null)
    try {
      await rolesApi.setClass(projectId, roleClass, draft)
      setFeedback('Saved')
      const t = setTimeout(() => setFeedback(null), 2000)
      return () => clearTimeout(t)
    } catch (err) {
      setFeedback(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  async function handlePreview() {
    if (!roleClass) return
    setPreviewing(true)
    try {
      const res = await rolesApi.previewClass(projectId, roleClass, {
        runtime: agent.runtime,
        role: agent.role,
        agent: agent.name,
      })
      setPreview(res.rendered)
    } catch {
      setPreview(null)
    } finally {
      setPreviewing(false)
    }
  }

  return (
    <div className="px-4 py-3 bg-surface border-t border-surface-border">
      <div className="flex items-center justify-between mb-2">
        <div>
          <p className="text-xs font-medium text-gray-500">
            Role Template
            {roleClass && (
              <span className="ml-1.5 font-mono text-accent text-[10px]">[class: {roleClass}]</span>
            )}
          </p>
          <p className="text-[10px] text-gray-600 mt-0.5">
            Shared by all agents using this class. Changes apply at next spawn.
          </p>
        </div>
      </div>

      {isLoading || !roleClass ? (
        <p className="text-xs text-gray-600">{!roleClass ? 'Role class not found' : 'Loading…'}</p>
      ) : detail?.is_protected ? (
        <div className="space-y-2">
          <div className="flex items-start gap-2 rounded border border-blue-800/40 bg-blue-900/10 px-3 py-2 text-xs text-blue-400">
            <span className="flex-1">
              <span className="font-medium">{roleClass}</span> is a built-in class embedded in AOM.
              To customise it, create a project-level override:
            </span>
          </div>
          <code className="block text-xs font-mono text-accent-yellow bg-surface border border-surface-border rounded px-3 py-2">
            aom class override {roleClass}
          </code>
          <pre className="text-[10px] text-gray-500 font-mono whitespace-pre-wrap bg-surface-raised border border-surface-border rounded p-3 max-h-48 overflow-auto">
            {detail.content}
          </pre>
        </div>
      ) : (
        <div className="space-y-2">
          <textarea
            value={draft ?? detail?.content ?? ''}
            onChange={(e) => setDraft(e.target.value)}
            rows={6}
            className="w-full bg-surface-raised border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent resize-y font-mono"
          />
          {preview && (
            <pre className="text-[10px] text-gray-500 font-mono whitespace-pre-wrap bg-surface border border-surface-border rounded p-3 max-h-48 overflow-auto">
              {preview}
            </pre>
          )}
          <div className="flex items-center gap-2">
            <button onClick={handleSave} disabled={saving || !draft} className={primaryBtn}>
              {saving ? 'Saving…' : 'Save'}
            </button>
            <button onClick={handlePreview} disabled={previewing} className={secondaryBtn}>
              {previewing ? 'Loading…' : preview ? 'Refresh Preview' : 'Preview Profile'}
            </button>
            {preview && (
              <button onClick={() => setPreview(null)} className={secondaryBtn}>Hide</button>
            )}
            {feedback && <span className="text-xs text-accent-green">{feedback}</span>}
          </div>
        </div>
      )}
    </div>
  )
}

// ─── System Template Panel ────────────────────────────────────────────────────

function SystemTemplatePanel() {
  const { data, isLoading } = useSystemTemplate()

  return (
    <div className="px-4 py-3 bg-surface border-t border-surface-border">
      <div className="flex items-center gap-2 mb-2">
        <p className="text-xs font-medium text-gray-500">System Template</p>
        <span className="text-[10px] px-1.5 py-0.5 rounded bg-blue-900/30 text-blue-400 border border-blue-800/50 font-mono">read-only</span>
      </div>
      <p className="text-[10px] text-gray-600 mb-2">
        AOM system protocol embedded in the binary — injected into every profile automatically.
        Covers: AOM Workflow, Team Communication, Collaboration Routines, Constraints.
      </p>
      {isLoading ? (
        <p className="text-xs text-gray-600">Loading…</p>
      ) : (
        <pre className="text-[10px] text-gray-500 font-mono whitespace-pre-wrap bg-surface-raised border border-surface-border rounded p-3 max-h-64 overflow-auto leading-relaxed">
          {data?.content ?? '(not available)'}
        </pre>
      )}
    </div>
  )
}

// ─── Expanded Agent Panel ─────────────────────────────────────────────────────

type ExpandTab = 'instructions' | 'role-template' | 'system'

interface ExpandedAgentPanelProps {
  projectId: string
  agent: Agent
}

function ExpandedAgentPanel({ projectId, agent }: ExpandedAgentPanelProps) {
  const [tab, setTab] = useState<ExpandTab>('instructions')

  const TABS: { id: ExpandTab; label: string }[] = [
    { id: 'instructions', label: 'Custom Instructions' },
    { id: 'role-template', label: 'Role Template' },
    { id: 'system', label: 'System Template' },
  ]

  return (
    <div className="border-t border-surface-border/50">
      <div className="flex gap-0 border-b border-surface-border bg-surface px-4 pt-2">
        {TABS.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={[
              'text-xs px-3 py-1.5 border-b-2 transition-colors',
              tab === t.id
                ? 'border-accent text-accent'
                : 'border-transparent text-gray-600 hover:text-gray-400',
            ].join(' ')}
          >
            {t.label}
          </button>
        ))}
      </div>
      {tab === 'instructions' && <CustomInstructionsPanel projectId={projectId} agent={agent} />}
      {tab === 'role-template' && <RoleTemplatePanel projectId={projectId} agent={agent} />}
      {tab === 'system' && <SystemTemplatePanel />}
    </div>
  )
}

// ─── Provision Modal ──────────────────────────────────────────────────────────

function ProvisionModal({
  agent,
  onClose,
  onProvision,
}: {
  agent: Agent
  onClose: () => void
  onProvision: (name: string) => Promise<{ status: string; output: string }>
}) {
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [output, setOutput] = useState<string | null>(null)

  async function handle() {
    setPending(true)
    setError(null)
    try {
      const res = await onProvision(agent.name)
      setOutput(res.output || 'Workspace provisioned.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to provision')
    } finally {
      setPending(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-md p-6 shadow-xl">
        <h3 className="text-sm font-semibold text-gray-300 mb-1">Provision Workspace</h3>
        <p className="text-xs text-gray-500 mb-4">
          Agent: <span className="text-accent">{agent.name}</span>
        </p>
        {output ? (
          <div className="space-y-3">
            <pre className="text-xs text-accent-green whitespace-pre-wrap break-words bg-surface p-3 rounded border border-surface-border max-h-48 overflow-auto">{output}</pre>
            <div className="flex justify-end">
              <button onClick={onClose} className={primaryBtn}>Close</button>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-xs text-gray-400">
              Creates a permanent git worktree at <code className="text-gray-300">.aom/agents/{agent.name}/workspace/</code> on branch <code className="text-gray-300">agents/{agent.name}</code>.
            </p>
            <p className="text-xs text-gray-500">
              After provisioning, this agent stays in the same workspace across all tasks (Workspace mode).
            </p>
            {error && (
              <div className="rounded-lg bg-accent-red/10 border border-accent-red/30 px-3 py-2">
                <pre className="text-xs text-accent-red whitespace-pre-wrap break-words">{error}</pre>
              </div>
            )}
            <div className="flex justify-end gap-2">
              <button onClick={onClose} className={secondaryBtn}>Cancel</button>
              <button onClick={handle} disabled={pending} className={primaryBtn}>
                {pending ? 'Provisioning…' : 'Provision'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// ─── Main View ────────────────────────────────────────────────────────────────

type Modal =
  | { type: 'add' }
  | { type: 'edit-model'; agent: Agent }
  | { type: 'spawn'; agent: Agent }
  | { type: 'remove'; agent: Agent }
  | { type: 'provision'; agent: Agent }

export function AgentsView() {
  const { selectedId } = useProjectContext()
  const { data: agents = [], isLoading, error, addAgent, updateAgent, removeAgent, spawnSession, provisionAgent } =
    useAgents(selectedId)
  const [modal, setModal] = useState<Modal | null>(null)
  const [expandedAgent, setExpandedAgent] = useState<string | null>(null)

  function toggleExpand(agentName: string) {
    setExpandedAgent((prev) => (prev === agentName ? null : agentName))
  }

  if (!selectedId) return <Empty message="Select a project." />
  if (isLoading) return <Empty message="Loading…" />
  if (error) return <Empty message={`Error: ${(error as Error).message}`} />

  return (
    <div className="h-full overflow-auto p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-semibold text-gray-300">Agents</h2>
        <button
          onClick={() => setModal({ type: 'add' })}
          className={primaryBtn}
        >
          + Add Agent
        </button>
      </div>

      {/* Table */}
      {agents.length === 0 ? (
        <p className="text-xs text-gray-600">
          No agents. Add one or run: <code className="text-gray-500">aom agent add</code>
        </p>
      ) : (
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-gray-600 border-b border-surface-border">
              <th className="pb-2 w-6"></th>
              <th className="pb-2 font-medium">Name</th>
              <th className="pb-2 font-medium">Role</th>
              <th className="pb-2 font-medium">Runtime</th>
              <th className="pb-2 font-medium">Model</th>
              <th className="pb-2 font-medium">Mode</th>
              <th className="pb-2 font-medium">Status</th>
              <th className="pb-2 font-medium">Enabled</th>
              <th className="pb-2 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {agents.map((agent) => (
              <>
                <tr
                  key={agent.name}
                  className="border-b border-surface-border/50 hover:bg-surface-raised"
                >
                  <td className="py-2 pr-1">
                    <button
                      onClick={() => toggleExpand(agent.name)}
                      className="text-gray-600 hover:text-gray-400 transition-colors"
                      title="Toggle custom instructions"
                    >
                      {expandedAgent === agent.name ? '▾' : '▸'}
                    </button>
                  </td>
                  <td className="py-2 pr-4 text-gray-300 font-medium">{agent.name}</td>
                  <td className="py-2 pr-4 text-gray-500">{agent.role}</td>
                  <td className="py-2 pr-4 text-gray-500">{agent.runtime}</td>
                  <td className="py-2 pr-4 text-gray-600">
                    {agent.model ?? (
                      <span className="text-gray-700 italic">default</span>
                    )}
                  </td>
                  <td className="py-2 pr-4">
                    {agent.workspace_path ? (
                      <span className="inline-flex items-center px-2 py-0.5 text-xs rounded border bg-accent/10 text-accent border-accent/30 font-medium">
                        Workspace
                      </span>
                    ) : (
                      <span className="text-gray-600 text-xs">Traditional</span>
                    )}
                  </td>
                  <td className="py-2 pr-4">
                    {agent.status ? <StatusBadge status={agent.status} /> : <span className="text-gray-700">—</span>}
                  </td>
                  <td className="py-2 pr-4">
                    <button
                      onClick={() =>
                        updateAgent(agent.name, { enabled: !agent.enabled })
                      }
                      className={[
                        'px-2 py-0.5 rounded text-xs border transition-colors',
                        agent.enabled
                          ? 'bg-accent-green/20 text-accent-green border-accent-green/30 hover:bg-accent-green/30'
                          : 'bg-gray-700/40 text-gray-500 border-gray-700 hover:border-gray-600',
                      ].join(' ')}
                    >
                      {agent.enabled ? 'Yes' : 'No'}
                    </button>
                  </td>
                  <td className="py-2">
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => setModal({ type: 'edit-model', agent })}
                        className="text-xs text-gray-500 hover:text-gray-300 transition-colors"
                      >
                        Edit Model
                      </button>
                      <span className="text-gray-700">|</span>
                      <button
                        onClick={() => setModal({ type: 'spawn', agent })}
                        className="text-xs text-accent hover:text-accent/80 transition-colors"
                      >
                        Spawn
                      </button>
                      {!agent.workspace_path && (
                        <>
                          <span className="text-gray-700">|</span>
                          <button
                            onClick={() => setModal({ type: 'provision', agent })}
                            className="text-xs text-accent-yellow hover:text-accent-yellow/80 transition-colors"
                          >
                            Provision
                          </button>
                        </>
                      )}
                      <span className="text-gray-700">|</span>
                      <button
                        onClick={() => setModal({ type: 'remove', agent })}
                        className="text-xs text-accent-red hover:text-accent-red/80 transition-colors"
                      >
                        Remove
                      </button>
                    </div>
                  </td>
                </tr>
                {expandedAgent === agent.name && selectedId && (
                  <tr key={`${agent.name}-expanded`} className="border-b border-surface-border/50">
                    <td colSpan={9} className="p-0">
                      <ExpandedAgentPanel projectId={selectedId} agent={agent} />
                    </td>
                  </tr>
                )}
              </>
            ))}
          </tbody>
        </table>
      )}

      {/* Modals */}
      {modal?.type === 'add' && (
        <AddAgentModal
          projectId={selectedId}
          onClose={() => setModal(null)}
          onSubmit={addAgent}
        />
      )}
      {modal?.type === 'edit-model' && (
        <EditModelModal
          agent={modal.agent}
          onClose={() => setModal(null)}
          onSubmit={(model) => updateAgent(modal.agent.name, { model })}
        />
      )}
      {modal?.type === 'spawn' && (
        <SpawnSessionModal
          agent={modal.agent}
          onClose={() => setModal(null)}
          onSubmit={spawnSession}
        />
      )}
      {modal?.type === 'remove' && (
        <ConfirmRemoveModal
          agent={modal.agent}
          onClose={() => setModal(null)}
          onConfirm={() => removeAgent(modal.agent.name)}
        />
      )}
      {modal?.type === 'provision' && (
        <ProvisionModal
          agent={modal.agent}
          onClose={() => setModal(null)}
          onProvision={provisionAgent}
        />
      )}
    </div>
  )
}
