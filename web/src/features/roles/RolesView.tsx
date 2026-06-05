import { useState } from 'react'
import { useProjectContext } from '@/app/ProjectContext'
import { useRoles, useClasses, useClassDetail, useSystemTemplate } from './hooks'
import { rolesApi } from './api'
import type { CreateRoleForm } from './types'

// ─── Shared primitives ────────────────────────────────────────────────────────

function Empty({ message }: { message: string }) {
  return (
    <div className="h-full flex items-center justify-center text-sm text-gray-600">
      {message}
    </div>
  )
}

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

const SOURCE_BADGE: Record<string, string> = {
  builtin: 'bg-blue-900/30 text-blue-400 border border-blue-800/50',
  custom: 'bg-green-900/30 text-green-400 border border-green-800/50',
  'builtin-overridden': 'bg-yellow-900/30 text-yellow-400 border border-yellow-800/50',
}

const WORKTREE_MODES = ['dedicated-writer', 'read-only']
const CHECKPOINT_OPTIONS = ['required', 'optional']
const SESSION_MODES = ['interactive', 'headless']
// Fallback used only while classes are loading
const BUILTIN_CLASSES_FALLBACK = ['builder', 'frontend', 'reviewer', 'orchestrator', 'researcher', 'generic']

// ─── Create Role Wizard ───────────────────────────────────────────────────────

interface WizardProps {
  projectId: string
  onClose: () => void
  onCreated: () => void
}

const WIZARD_STEPS = ['Basic Info', 'Behavior', 'Role Template', 'Confirm']

const defaultTemplate = (name: string) =>
  `## Responsibilities
- Complete the assigned task according to the task artifacts and current session state
- Deliver clear, well-structured output that the operator and teammates can act on

## Work Standards
- Read task.md and state.md before starting — understand scope and prior progress
- Flag ambiguities early — update state.md with open questions rather than guessing

## ${name.charAt(0).toUpperCase() + name.slice(1)}-Specific Instructions
<!-- Add domain-specific guidance here -->

## Custom Instructions
<!-- Add project-specific or agent-specific instructions here. This section is managed by the operator and will not be overwritten by AOM system updates. -->`

function CreateRoleWizard({ projectId, onClose, onCreated }: WizardProps) {
  const [step, setStep] = useState(0)
  const { data: availableClasses = [] } = useClasses(projectId)
  const classOptions = availableClasses.length > 0 ? availableClasses : BUILTIN_CLASSES_FALLBACK.map((c) => ({ name: c, description: '' }))
  const [form, setForm] = useState<CreateRoleForm>({
    name: '',
    class: '',
    worktree_mode: 'dedicated-writer',
    checkpoint_expectation: 'required',
    default_session_mode: 'interactive',
    description: '',
  })
  const [classContent, setClassContent] = useState('')
  const [useExistingClass, setUseExistingClass] = useState(true)
  const [previewHtml, setPreviewHtml] = useState<string | null>(null)
  const [previewing, setPreviewing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function patchForm(patch: Partial<CreateRoleForm>) {
    setForm((f) => ({ ...f, ...patch }))
  }

  async function loadPreview() {
    if (!form.class) return
    setPreviewing(true)
    try {
      const res = await rolesApi.previewClass(projectId, form.class, { runtime: 'claude' })
      setPreviewHtml(res.rendered)
    } catch {
      setPreviewHtml(null)
    } finally {
      setPreviewing(false)
    }
  }

  async function handleFinish() {
    setError(null)
    setSaving(true)
    try {
      // If creating a new custom class, save the template first.
      if (!useExistingClass && form.class && classContent) {
        await rolesApi.setClass(projectId, form.class, classContent)
      }
      await rolesApi.createRole(projectId, form)
      onCreated()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create role')
      setSaving(false)
    }
  }

  const canNext = () => {
    if (step === 0) return !!form.name.trim() && !!form.class.trim()
    if (step === 2 && !useExistingClass) return !!classContent.trim()
    return true
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-2xl shadow-xl flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3 border-b border-surface-border shrink-0">
          <h3 className="text-sm font-semibold text-gray-300">Create Role</h3>
          <button onClick={onClose} className="text-gray-600 hover:text-gray-300 text-lg leading-none">×</button>
        </div>

        {/* Step indicator */}
        <div className="flex gap-0 px-5 py-2 border-b border-surface-border shrink-0">
          {WIZARD_STEPS.map((label, i) => (
            <div key={label} className="flex items-center gap-1.5 mr-4">
              <span className={[
                'flex items-center justify-center w-5 h-5 rounded-full text-[10px] font-bold shrink-0',
                i < step ? 'bg-accent/60 text-surface' :
                i === step ? 'bg-accent text-surface' :
                'bg-surface border border-surface-border text-gray-600',
              ].join(' ')}>{i + 1}</span>
              <span className={`text-xs ${i === step ? 'text-gray-300' : 'text-gray-600'}`}>{label}</span>
            </div>
          ))}
        </div>

        {/* Content */}
        <div className="flex-1 overflow-auto px-5 py-4 space-y-4">
          {step === 0 && (
            <>
              <Field label="Role Name">
                <input
                  autoFocus
                  value={form.name}
                  onChange={(e) => patchForm({ name: e.target.value })}
                  className={inputCls}
                  placeholder="e.g. researcher, analyst, qa"
                />
                <p className="text-[10px] text-gray-600 mt-1">Used as the role identifier in agents.yaml — lowercase, no spaces</p>
              </Field>

              <Field label="Description (optional)">
                <input
                  value={form.description}
                  onChange={(e) => patchForm({ description: e.target.value })}
                  className={inputCls}
                  placeholder="e.g. Analyses market data and produces weekly reports"
                />
                <p className="text-[10px] text-gray-600 mt-1">Shown in class list and Add Agent modal to help the team pick the right role</p>
              </Field>

              <div>
                <label className="block text-xs text-gray-500 mb-2">Class Template</label>
                <div className="flex gap-3 mb-2">
                  <label className="flex items-center gap-1.5 text-xs text-gray-400 cursor-pointer">
                    <input type="radio" checked={useExistingClass} onChange={() => setUseExistingClass(true)} className="accent-accent" />
                    Use existing class
                  </label>
                  <label className="flex items-center gap-1.5 text-xs text-gray-400 cursor-pointer">
                    <input type="radio" checked={!useExistingClass} onChange={() => { setUseExistingClass(false); if (!form.class) patchForm({ class: form.name }) }} className="accent-accent" />
                    Create new class
                  </label>
                </div>
                {useExistingClass ? (
                  <>
                    <select
                      value={form.class}
                      onChange={(e) => patchForm({ class: e.target.value })}
                      className={inputCls}
                    >
                      <option value="">— select a class —</option>
                      {classOptions.map((c) => (
                        <option key={c.name} value={c.name}>{c.name}</option>
                      ))}
                    </select>
                    {form.class && (() => {
                      const desc = classOptions.find((c) => c.name === form.class)?.description
                      return desc ? <p className="mt-1 text-[11px] text-gray-500">{desc}</p> : null
                    })()}
                  </>
                ) : (
                  <input
                    value={form.class}
                    onChange={(e) => patchForm({ class: e.target.value })}
                    className={inputCls}
                    placeholder="e.g. researcher (new custom class name)"
                  />
                )}
              </div>
            </>
          )}

          {step === 1 && (
            <>
              <Field label="Worktree Mode">
                <select
                  value={form.worktree_mode}
                  onChange={(e) => patchForm({ worktree_mode: e.target.value })}
                  className={inputCls}
                >
                  {WORKTREE_MODES.map((m) => <option key={m} value={m}>{m}</option>)}
                </select>
                <p className="text-[10px] text-gray-600 mt-1">
                  dedicated-writer = agent gets its own git worktree for writing files.<br />
                  read-only = agent only reads (orchestrators, reviewers, researchers).
                </p>
              </Field>
              <Field label="Checkpoint Expectation">
                <select
                  value={form.checkpoint_expectation}
                  onChange={(e) => patchForm({ checkpoint_expectation: e.target.value })}
                  className={inputCls}
                >
                  {CHECKPOINT_OPTIONS.map((o) => <option key={o} value={o}>{o}</option>)}
                </select>
              </Field>
              <Field label="Session Mode">
                <select
                  value={form.default_session_mode}
                  onChange={(e) => patchForm({ default_session_mode: e.target.value })}
                  className={inputCls}
                >
                  {SESSION_MODES.map((m) => <option key={m} value={m}>{m}</option>)}
                </select>
              </Field>
            </>
          )}

          {step === 2 && (
            <>
              {useExistingClass ? (
                <div className="space-y-2">
                  <p className="text-xs text-gray-400">
                    Using built-in class <span className="text-accent font-mono">{form.class}</span>.
                    The template below is read-only. Use <code className="text-gray-300">aom class override {form.class}</code> to customise it.
                  </p>
                  <button onClick={loadPreview} disabled={previewing} className={secondaryBtn}>
                    {previewing ? 'Loading…' : 'Preview Full Profile'}
                  </button>
                  {previewHtml && (
                    <pre className="text-[10px] text-gray-500 font-mono whitespace-pre-wrap bg-surface border border-surface-border rounded p-3 max-h-64 overflow-auto">
                      {previewHtml}
                    </pre>
                  )}
                </div>
              ) : (
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <p className="text-xs text-gray-400">
                      Define the <span className="text-accent font-mono">{form.class}</span> class template (Responsibilities, Work Standards, domain-specific guidance).
                      The AOM system protocol (team communication, signals) is injected automatically.
                    </p>
                    <button
                      onClick={() => { if (!classContent) setClassContent(defaultTemplate(form.class)) }}
                      className={`${secondaryBtn} shrink-0 ml-2`}
                    >
                      Load starter
                    </button>
                  </div>
                  <textarea
                    value={classContent}
                    onChange={(e) => setClassContent(e.target.value)}
                    placeholder="## Responsibilities&#10;- ..."
                    rows={14}
                    className="w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent resize-y font-mono"
                  />
                  <div className="flex items-center gap-2">
                    <button
                      onClick={async () => {
                        if (!classContent.trim()) return
                        setPreviewing(true)
                        try {
                          // Use a temporary save-and-preview pattern via existing preview endpoint
                          // by posting to setClass first then previewing, then deleting if needed.
                          // Simpler: just show the raw template content as-is.
                          setPreviewHtml(classContent)
                        } finally {
                          setPreviewing(false)
                        }
                      }}
                      className={secondaryBtn}
                    >
                      Show Template
                    </button>
                  </div>
                </div>
              )}
            </>
          )}

          {step === 3 && (
            <div className="space-y-3">
              <p className="text-xs font-medium text-gray-400">Review & Confirm</p>
              <div className="bg-surface border border-surface-border rounded p-3 space-y-1.5 text-xs">
                <Row label="Role name" value={form.name} />
                <Row label="Class" value={form.class} />
                <Row label="Worktree mode" value={form.worktree_mode} />
                <Row label="Checkpoint" value={form.checkpoint_expectation} />
                <Row label="Session mode" value={form.default_session_mode} />
                {!useExistingClass && <Row label="New class template" value={`will be saved to .aom/templates/profiles/${form.class}.md.tmpl`} />}
              </div>
              {error && <p className="text-xs text-accent-red">{error}</p>}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex justify-between items-center px-5 py-3 border-t border-surface-border shrink-0">
          <button onClick={step === 0 ? onClose : () => setStep(s => s - 1)} className={secondaryBtn}>
            {step === 0 ? 'Cancel' : '← Back'}
          </button>
          {step < WIZARD_STEPS.length - 1 ? (
            <button onClick={() => setStep(s => s + 1)} disabled={!canNext()} className={primaryBtn}>
              Next →
            </button>
          ) : (
            <button onClick={handleFinish} disabled={saving} className={primaryBtn}>
              {saving ? 'Creating…' : 'Create Role'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex gap-2">
      <span className="text-gray-600 w-36 shrink-0">{label}:</span>
      <span className="text-gray-300 font-mono">{value}</span>
    </div>
  )
}

// ─── Class Editor Panel ───────────────────────────────────────────────────────

function ClassEditorPanel({
  projectId,
  className,
  onClose,
}: {
  projectId: string
  className: string
  onClose: () => void
}) {
  const { data: detail, isLoading } = useClassDetail(projectId, className)
  const { setClass } = useClasses(projectId)
  const [draft, setDraft] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [feedback, setFeedback] = useState<string | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [previewing, setPreviewing] = useState(false)

  const content = draft ?? detail?.content ?? ''

  if (!detail && !isLoading) return null

  async function handleSave() {
    if (!draft) return
    setSaving(true)
    setFeedback(null)
    try {
      await setClass(className, draft)
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
    setPreviewing(true)
    try {
      const res = await rolesApi.previewClass(projectId, className)
      setPreview(res.rendered)
    } catch {
      setPreview(null)
    } finally {
      setPreviewing(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-4xl shadow-xl flex flex-col max-h-[90vh]">
        <div className="flex items-center justify-between px-5 py-3 border-b border-surface-border shrink-0">
          <div>
            <h3 className="text-sm font-semibold text-gray-300">
              Class: <span className="text-accent font-mono">{className}</span>
            </h3>
            {detail && (
              <span className={`text-[10px] px-1.5 py-0.5 rounded font-mono mt-0.5 inline-block ${SOURCE_BADGE[detail.source] ?? ''}`}>
                {detail.source}
              </span>
            )}
          </div>
          <button onClick={onClose} className="text-gray-600 hover:text-gray-300 text-lg leading-none">×</button>
        </div>

        <div className={`flex-1 overflow-hidden ${preview ? 'grid grid-cols-2 gap-0' : 'flex flex-col'}`}>
          {/* Editor */}
          <div className="flex flex-col flex-1 min-h-0 overflow-hidden px-5 py-3">
            {detail?.is_protected ? (
              <div className="flex flex-col gap-2 flex-1 min-h-0">
                <p className="text-xs text-gray-500 shrink-0">
                  This is a built-in class embedded in the AOM binary.
                  To customise it, create a project-level override:
                </p>
                <code className="block text-xs font-mono text-accent-yellow bg-surface border border-surface-border rounded px-3 py-2 shrink-0">
                  aom class override {className}
                </code>
                <pre className="text-[10px] text-gray-500 font-mono whitespace-pre-wrap bg-surface border border-surface-border rounded p-3 flex-1 overflow-auto">
                  {content}
                </pre>
              </div>
            ) : (
              <div className="flex flex-col gap-2 flex-1 min-h-0">
                <p className="text-xs text-gray-600 shrink-0">
                  Zone B — Role Template. Shared by all agents using this class.
                  The AOM system protocol is injected automatically (Zone A).
                </p>
                <textarea
                  value={draft ?? detail?.content ?? ''}
                  onChange={(e) => setDraft(e.target.value)}
                  className="flex-1 bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent resize-none font-mono min-h-[200px]"
                />
                <div className="flex items-center gap-2 shrink-0">
                  <button onClick={handleSave} disabled={saving || !draft} className={primaryBtn}>
                    {saving ? 'Saving…' : 'Save'}
                  </button>
                  <button onClick={handlePreview} disabled={previewing} className={secondaryBtn}>
                    {previewing ? 'Loading…' : preview ? 'Refresh Preview' : 'Preview Full Profile'}
                  </button>
                  {preview && (
                    <button onClick={() => setPreview(null)} className={secondaryBtn}>Hide Preview</button>
                  )}
                  {feedback && <span className="text-xs text-accent-green">{feedback}</span>}
                </div>
              </div>
            )}
          </div>

          {/* Preview */}
          {preview && (
            <div className="border-l border-surface-border overflow-auto px-5 py-3">
              <p className="text-xs font-medium text-gray-500 mb-2 shrink-0">Full Profile Preview</p>
              <pre className="text-[10px] text-gray-400 font-mono whitespace-pre-wrap leading-relaxed">
                {preview}
              </pre>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ─── Roles Table ──────────────────────────────────────────────────────────────

function RolesTable({
  projectId,
  onCreateRole,
}: {
  projectId: string
  onCreateRole: () => void
}) {
  const { data: roles = [], isLoading, deleteRole } = useRoles(projectId)
  const [deleting, setDeleting] = useState<string | null>(null)

  async function handleDelete(name: string) {
    if (!confirm(`Delete role "${name}"? This cannot be undone.`)) return
    setDeleting(name)
    try {
      await deleteRole(name)
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Delete failed')
    } finally {
      setDeleting(null)
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wide">Roles</h3>
        <button onClick={onCreateRole} className={primaryBtn}>+ Create Role</button>
      </div>
      {isLoading ? (
        <p className="text-xs text-gray-600">Loading…</p>
      ) : roles.length === 0 ? (
        <p className="text-xs text-gray-600">No roles defined. Create one or run: <code className="text-gray-500">aom role create</code></p>
      ) : (
        <table className="w-full text-xs">
          <thead>
            <tr className="text-gray-600 border-b border-surface-border">
              <th className="text-left py-1.5 pr-4 font-medium">Name</th>
              <th className="text-left py-1.5 pr-4 font-medium">Class</th>
              <th className="text-left py-1.5 pr-4 font-medium">Description</th>
              <th className="text-left py-1.5 pr-4 font-medium">Worktree</th>
              <th className="text-left py-1.5 pr-4 font-medium">Checkpoint</th>
              <th className="text-left py-1.5 font-medium">Agents</th>
              <th></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-surface-border/30">
            {roles.map((r) => (
              <tr key={r.name} className="hover:bg-surface/50">
                <td className="py-2 pr-4 font-mono text-gray-300">{r.name}</td>
                <td className="py-2 pr-4 font-mono text-accent">{r.class}</td>
                <td className="py-2 pr-4 text-xs text-gray-400 max-w-xs">{r.description || '—'}</td>
                <td className="py-2 pr-4 text-gray-500">{r.worktree_mode}</td>
                <td className="py-2 pr-4 text-gray-500">{r.checkpoint_expectation}</td>
                <td className="py-2 text-gray-500">{r.agents_using?.join(', ') || '—'}</td>
                <td className="py-2 text-right">
                  <button
                    onClick={() => handleDelete(r.name)}
                    disabled={deleting === r.name || (r.agents_using?.length ?? 0) > 0}
                    className={`${dangerBtn} text-[10px] px-2 py-0.5`}
                    title={(r.agents_using?.length ?? 0) > 0 ? 'Remove agents first' : 'Delete role'}
                  >
                    {deleting === r.name ? '…' : 'Delete'}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

// ─── Classes Table ────────────────────────────────────────────────────────────

function ClassesTable({
  projectId,
  onEditClass,
}: {
  projectId: string
  onEditClass: (name: string) => void
}) {
  const { data: classes = [], isLoading, deleteClass } = useClasses(projectId)
  const [deleting, setDeleting] = useState<string | null>(null)

  async function handleDelete(name: string, source: string) {
    const msg = source === 'builtin-overridden'
      ? `Revert class "${name}" to the built-in default? Your project override will be deleted.`
      : `Delete custom class "${name}"?`
    if (!confirm(msg)) return
    setDeleting(name)
    try {
      await deleteClass(name)
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Delete failed')
    } finally {
      setDeleting(null)
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wide">Classes</h3>
        <p className="text-[10px] text-gray-600">Custom: <code>.aom/templates/profiles/&lt;class&gt;.md.tmpl</code></p>
      </div>
      {isLoading ? (
        <p className="text-xs text-gray-600">Loading…</p>
      ) : (
        <table className="w-full text-xs">
          <thead>
            <tr className="text-gray-600 border-b border-surface-border">
              <th className="text-left py-1.5 pr-4 font-medium">Name</th>
              <th className="text-left py-1.5 pr-4 font-medium">Source</th>
              <th className="text-left py-1.5 pr-4 font-medium">Description</th>
              <th className="text-left py-1.5 font-medium">Roles Using</th>
              <th></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-surface-border/30">
            {classes.map((c) => (
              <tr key={c.name} className="hover:bg-surface/50">
                <td className="py-2 pr-4 font-mono text-gray-300">{c.name}</td>
                <td className="py-2 pr-4">
                  <span className={`px-1.5 py-0.5 rounded text-[10px] font-mono ${SOURCE_BADGE[c.source] ?? ''}`}>
                    {c.source}
                  </span>
                </td>
                <td className="py-2 pr-4 text-xs text-gray-400 max-w-xs">{c.description || '—'}</td>
                <td className="py-2 text-gray-500">{c.roles_using?.join(', ') || '—'}</td>
                <td className="py-2 text-right flex items-center justify-end gap-1.5">
                  <button
                    onClick={() => onEditClass(c.name)}
                    className={`${secondaryBtn} text-[10px] px-2 py-0.5`}
                  >
                    {c.source === 'builtin' ? 'View' : 'Edit'}
                  </button>
                  {c.source !== 'builtin' && (
                    <button
                      onClick={() => handleDelete(c.name, c.source)}
                      disabled={deleting === c.name}
                      className={`${dangerBtn} text-[10px] px-2 py-0.5`}
                    >
                      {deleting === c.name ? '…' : c.source === 'builtin-overridden' ? 'Revert' : 'Delete'}
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

// ─── System Template Section ──────────────────────────────────────────────────

function SystemTemplateSection() {
  const { data, isLoading } = useSystemTemplate()
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="border border-surface-border rounded p-3">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-xs font-medium text-gray-400">System Template (Zone A)</p>
          <p className="text-[10px] text-gray-600 mt-0.5">
            AOM system protocol — embedded in binary, injected into every profile, read-only.
            Covers: AOM Workflow, Team Communication, Collaboration Routines.
          </p>
        </div>
        <button onClick={() => setExpanded(e => !e)} className={secondaryBtn}>
          {expanded ? 'Hide' : 'View'}
        </button>
      </div>
      {expanded && (
        <div className="mt-3">
          {isLoading ? (
            <p className="text-xs text-gray-600">Loading…</p>
          ) : (
            <pre className="text-[10px] text-gray-500 font-mono whitespace-pre-wrap bg-surface border border-surface-border rounded p-3 max-h-80 overflow-auto leading-relaxed">
              {data?.content ?? ''}
            </pre>
          )}
        </div>
      )}
    </div>
  )
}

// ─── Main View ────────────────────────────────────────────────────────────────

export function RolesView() {
  const { selectedId } = useProjectContext()
  const [showWizard, setShowWizard] = useState(false)
  const [editingClass, setEditingClass] = useState<string | null>(null)
  const { data: _roles, refetch } = useRoles(selectedId)

  if (!selectedId) return <Empty message="Select a project." />

  return (
    <div className="h-full overflow-auto p-4 space-y-6">
      {/* Header */}
      <div>
        <h2 className="text-sm font-semibold text-gray-300">Roles & Classes</h2>
        <p className="text-xs text-gray-600 mt-0.5">
          Roles define agent behavior. Classes are the profile templates behind roles.
        </p>
      </div>

      {/* System Template */}
      <SystemTemplateSection />

      {/* Roles Table */}
      <RolesTable
        projectId={selectedId}
        onCreateRole={() => setShowWizard(true)}
      />

      {/* Classes Table */}
      <ClassesTable
        projectId={selectedId}
        onEditClass={(name) => setEditingClass(name)}
      />

      {/* Wizard */}
      {showWizard && (
        <CreateRoleWizard
          projectId={selectedId}
          onClose={() => setShowWizard(false)}
          onCreated={() => { setShowWizard(false); refetch() }}
        />
      )}

      {/* Class Editor */}
      {editingClass && (
        <ClassEditorPanel
          projectId={selectedId}
          className={editingClass}
          onClose={() => setEditingClass(null)}
        />
      )}
    </div>
  )
}
