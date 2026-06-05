import { useState } from 'react'
import { Plus } from 'lucide-react'
import { useProjectContext } from '@/app/ProjectContext'
import { useProjectAgents } from '@/features/projects/hooks'
import { StatusBadge } from '@/components/StatusBadge'
import { useSessions } from './hooks'
import { useTasks } from '@/features/tasks/hooks'
import type { Session } from './types'

// ─── Shared primitives ────────────────────────────────────────────────────────

const primaryBtn =
  'px-3 py-1.5 text-xs bg-accent text-surface font-medium rounded hover:bg-accent/80 disabled:opacity-50 transition-colors'
const secondaryBtn =
  'px-3 py-1.5 text-xs bg-surface border border-surface-border text-gray-400 rounded hover:text-gray-300 transition-colors'

function Empty({ message }: { message: string }) {
  return (
    <div className="h-full flex items-center justify-center text-sm text-gray-600">
      {message}
    </div>
  )
}

// ─── Spawn Modal ──────────────────────────────────────────────────────────────

function SpawnModal({
  projectId,
  onClose,
  onSpawn,
}: {
  projectId: string
  onClose: () => void
  onSpawn: (agent: string, mode: 'real' | 'mock', taskId?: string, persistent?: boolean) => Promise<unknown>
}) {
  const { data: agents = [] } = useProjectAgents(projectId)
  const [agent, setAgent] = useState('')
  const [mode, setMode] = useState<'real' | 'mock'>('real')
  const [taskId, setTaskId] = useState('')
  const [persistent, setPersistent] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [output, setOutput] = useState<string | null>(null)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!agent.trim()) return
    setPending(true)
    setError(null)
    try {
      await onSpawn(agent.trim(), mode, taskId.trim() || undefined, persistent)
      setOutput('Session spawned.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to spawn')
    } finally {
      setPending(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-md p-6 shadow-xl">
        <h3 className="text-sm font-semibold text-gray-300 mb-4">Spawn Session</h3>
        {output ? (
          <div className="space-y-3">
            <p className="text-xs text-accent-green">{output}</p>
            <div className="flex justify-end">
              <button onClick={onClose} className={primaryBtn}>Close</button>
            </div>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-3">
            {/* Agent */}
            <div>
              <label className="block text-xs text-gray-500 mb-1">Agent</label>
              {agents.length > 0 ? (
                <select
                  value={agent}
                  onChange={(e) => setAgent(e.target.value)}
                  className="w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent"
                >
                  <option value="">— select agent —</option>
                  {agents.map((a) => (
                    <option key={a.name} value={a.name}>{a.name}</option>
                  ))}
                </select>
              ) : (
                <input
                  autoFocus
                  type="text"
                  placeholder="agent-name"
                  value={agent}
                  onChange={(e) => setAgent(e.target.value)}
                  className="w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent"
                />
              )}
            </div>

            {/* Mode */}
            <div>
              <label className="block text-xs text-gray-500 mb-1">Mode</label>
              <div className="flex gap-2">
                {(['real', 'mock'] as const).map((m) => (
                  <label
                    key={m}
                    className={[
                      'flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-lg border cursor-pointer text-xs transition-colors',
                      mode === m
                        ? 'border-accent bg-accent/10 text-accent'
                        : 'border-surface-border text-gray-500 hover:bg-surface',
                    ].join(' ')}
                  >
                    <input
                      type="radio"
                      name="mode"
                      value={m}
                      checked={mode === m}
                      onChange={() => setMode(m)}
                      className="sr-only"
                    />
                    {m === 'real' ? '⚡ Real' : '🧪 Mock'}
                  </label>
                ))}
              </div>
            </div>

            {/* Task ID (optional) */}
            <div>
              <label className="block text-xs text-gray-500 mb-1">Task ID <span className="text-gray-700">(optional)</span></label>
              <input
                type="text"
                placeholder="TASK-xxx"
                value={taskId}
                onChange={(e) => setTaskId(e.target.value)}
                className="w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent"
              />
            </div>

            {/* Persistent */}
            <div>
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={persistent}
                  onChange={(e) => setPersistent(e.target.checked)}
                  className="accent-accent"
                />
                <span className="text-xs text-gray-400">Keep alive <span className="text-gray-600">(persistent — don't auto-stop on task completion)</span></span>
              </label>
            </div>

            {error && (
              <div className="rounded-lg bg-accent-red/10 border border-accent-red/30 px-3 py-2">
                <pre className="text-xs text-accent-red whitespace-pre-wrap break-words">{error}</pre>
              </div>
            )}
            <div className="flex justify-end gap-2">
              <button type="button" onClick={onClose} className={secondaryBtn}>Cancel</button>
              <button type="submit" disabled={pending || !agent.trim()} className={primaryBtn}>
                {pending ? 'Spawning…' : 'Spawn'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}

// ─── Send Message Modal ───────────────────────────────────────────────────────

function SendMsgModal({
  session,
  onClose,
  onSend,
}: {
  session: Session
  onClose: () => void
  onSend: (sessionId: string, message: string) => Promise<void>
}) {
  const [msg, setMsg] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!msg.trim()) return
    setError(null)
    setSending(true)
    try {
      await onSend(session.id, msg.trim())
      setDone(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send')
    } finally {
      setSending(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-md p-6 shadow-xl">
        <h3 className="text-sm font-semibold text-gray-300 mb-1">Send Message</h3>
        <p className="text-xs text-gray-500 mb-4">
          To: <span className="text-gray-400">{session.agent_name}</span>
        </p>
        {done ? (
          <div className="space-y-3">
            <p className="text-xs text-accent-green">Message sent.</p>
            <div className="flex justify-end">
              <button onClick={onClose} className={primaryBtn}>Close</button>
            </div>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-3">
            <textarea
              autoFocus
              value={msg}
              onChange={(e) => setMsg(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                  e.preventDefault()
                  handleSubmit(e as unknown as React.FormEvent)
                }
              }}
              rows={4}
              placeholder="Type your message… (Ctrl+Enter to send)"
              className="w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent resize-none"
            />
            {error && <p className="text-xs text-accent-red">{error}</p>}
            <div className="flex justify-end gap-2">
              <button type="button" onClick={onClose} className={secondaryBtn}>Cancel</button>
              <button type="submit" disabled={sending || !msg.trim()} className={primaryBtn}>
                {sending ? 'Sending…' : 'Send'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}

// ─── Recover Modal ────────────────────────────────────────────────────────────

function RecoverModal({
  session,
  onClose,
  onRecover,
}: {
  session: Session
  onClose: () => void
  onRecover: (sessionId: string) => Promise<{ status: string; output: string }>
}) {
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [output, setOutput] = useState<string | null>(null)

  async function handleRecover() {
    setPending(true)
    setError(null)
    try {
      const res = await onRecover(session.id)
      setOutput(res.output || 'Recovery initiated.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to recover')
    } finally {
      setPending(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-md p-6 shadow-xl">
        <h3 className="text-sm font-semibold text-gray-300 mb-1">Recover Session</h3>
        <p className="text-xs text-gray-500 mb-4">
          Agent: <span className="text-gray-400">{session.agent_name}</span>
        </p>
        {output ? (
          <div className="space-y-3">
            <pre className="text-xs text-accent-green whitespace-pre-wrap break-words">{output}</pre>
            <div className="flex justify-end">
              <button onClick={onClose} className={primaryBtn}>Close</button>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-xs text-gray-400">
              Diagnoses the failed session and recommends a recovery action.
            </p>
            {error && (
              <div className="rounded-lg bg-accent-red/10 border border-accent-red/30 px-3 py-2">
                <pre className="text-xs text-accent-red whitespace-pre-wrap break-words">{error}</pre>
              </div>
            )}
            <div className="flex justify-end gap-2">
              <button onClick={onClose} className={secondaryBtn}>Cancel</button>
              <button onClick={handleRecover} disabled={pending} className={primaryBtn}>
                {pending ? 'Recovering…' : 'Recover'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// ─── SessionsView ─────────────────────────────────────────────────────────────

type Modal = { type: 'send'; session: Session } | { type: 'spawn' } | { type: 'recover'; session: Session }

const STOPPABLE = ['Working', 'Idle', 'Booting', 'WaitingApproval', 'WaitingHandoff']
const ARCHIVABLE = ['Stopped', 'Failed']
const RESUMABLE = ['Stopped', 'Failed']
const RECOVERABLE = ['Failed']
const APPROVABLE = ['WaitingApproval']

export function SessionsView() {
  const { selectedId } = useProjectContext()
  const {
    data: sessions = [],
    isLoading,
    stop,
    archive,
    sendMessage,
    spawn,
    resume,
    approve,
    deny,
    recover,
    resumeMutation,
    approveMutation,
    denyMutation,
  } = useSessions(selectedId)
  const [modal, setModal] = useState<Modal | null>(null)
  const [activeOnly, setActiveOnly] = useState(false)
  const { data: tasks = [] } = useTasks(selectedId)

  if (!selectedId) return <Empty message="Select a project." />
  if (isLoading) return <Empty message="Loading…" />

  const INACTIVE = ['Stopped', 'Archived', 'Failed', 'Detached']
  const displayed = activeOnly ? sessions.filter((s) => !INACTIVE.includes(s.status)) : sessions

  const taskTitleMap = Object.fromEntries(tasks.map((t) => [t.id, t.title]))

  return (
    <div className="h-full overflow-auto p-4">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-3">
          <h2 className="text-sm font-semibold text-gray-300">Sessions</h2>
          <div className="flex items-center gap-1 bg-surface border border-surface-border rounded p-0.5">
            {(['All', 'Active'] as const).map((label) => (
              <button
                key={label}
                onClick={() => setActiveOnly(label === 'Active')}
                className={[
                  'px-2.5 py-1 text-xs rounded transition-colors',
                  (label === 'Active') === activeOnly
                    ? 'bg-accent/20 text-accent'
                    : 'text-gray-500 hover:text-gray-300',
                ].join(' ')}
              >
                {label}
              </button>
            ))}
          </div>
        </div>
        <button
          onClick={() => setModal({ type: 'spawn' })}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-accent text-surface font-medium rounded hover:bg-accent/80 transition-colors"
        >
          <Plus size={12} /> Spawn Session
        </button>
      </div>

      {displayed.length === 0 ? (
        <p className="text-xs text-gray-600">{activeOnly ? 'No active sessions.' : 'No sessions.'}</p>
      ) : (
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-gray-600 border-b border-surface-border">
              <th className="pb-2 font-medium">Agent</th>
              <th className="pb-2 font-medium">Status</th>
              <th className="pb-2 font-medium">Task</th>
              <th className="pb-2 font-medium">Pane</th>
              <th className="pb-2 font-medium">Created</th>
              <th className="pb-2 font-medium">Flags</th>
              <th className="pb-2 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {displayed.map((s) => (
              <tr key={s.id} className="border-b border-surface-border/50 hover:bg-surface-raised">
                <td className="py-2 pr-4">
                  <span className="text-gray-300">{s.agent_name}</span>
                  <span className="ml-1.5 text-gray-700 font-mono text-xs" title={s.id}>#{s.id.slice(-6)}</span>
                </td>
                <td className="py-2 pr-4"><StatusBadge status={s.status} /></td>
                <td className="py-2 pr-4 max-w-[140px]">
                  {s.task_id ? (
                    <div className="flex flex-col">
                      <span className="text-gray-500 font-mono">{s.task_id}</span>
                      {taskTitleMap[s.task_id] && (
                        <span className="text-gray-600 truncate">{taskTitleMap[s.task_id]}</span>
                      )}
                    </div>
                  ) : <span className="text-gray-700">—</span>}
                </td>
                <td className="py-2 pr-4 text-gray-600 font-mono">{s.tmux_pane ?? '—'}</td>
                <td className="py-2 pr-4 text-gray-600 whitespace-nowrap">{s.created_at.slice(0, 16)}</td>
                <td className="py-2 pr-4">
                  {s.persistent && (
                    <span className="inline-flex items-center px-1.5 py-0.5 text-xs rounded border bg-accent/10 text-accent border-accent/30">
                      persistent
                    </span>
                  )}
                </td>
                <td className="py-2">
                  <div className="flex items-center gap-2 flex-wrap">
                    {APPROVABLE.includes(s.status) && (
                      <button
                        onClick={() => approve(s.id)}
                        disabled={approveMutation.isPending}
                        className="text-xs text-accent-green hover:underline disabled:opacity-50"
                      >
                        Approve
                      </button>
                    )}
                    {APPROVABLE.includes(s.status) && (
                      <button
                        onClick={() => deny(s.id)}
                        disabled={denyMutation.isPending}
                        className="text-xs text-accent-red hover:underline disabled:opacity-50"
                      >
                        Deny
                      </button>
                    )}
                    {STOPPABLE.includes(s.status) && (
                      <button
                        onClick={() => stop(s.agent_name)}
                        className="text-xs text-accent-red hover:underline"
                      >
                        Stop
                      </button>
                    )}
                    {RESUMABLE.includes(s.status) && (
                      <button
                        onClick={() => resume(s.id)}
                        disabled={resumeMutation.isPending}
                        className="text-xs text-accent-green hover:underline disabled:opacity-50"
                      >
                        Resume
                      </button>
                    )}
                    {RECOVERABLE.includes(s.status) && (
                      <button
                        onClick={() => setModal({ type: 'recover', session: s })}
                        className="text-xs text-accent-yellow hover:underline"
                      >
                        Recover
                      </button>
                    )}
                    {ARCHIVABLE.includes(s.status) && (
                      <button
                        onClick={() => archive(s.id)}
                        className="text-xs text-gray-500 hover:text-gray-300 transition-colors"
                      >
                        Archive
                      </button>
                    )}
                    {s.status !== 'Archived' && (
                      <button
                        onClick={() => setModal({ type: 'send', session: s })}
                        className="text-xs text-accent hover:text-accent/80 transition-colors"
                      >
                        Send
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {modal?.type === 'spawn' && selectedId && (
        <SpawnModal
          projectId={selectedId}
          onClose={() => setModal(null)}
          onSpawn={async (agent, mode, taskId, persistent) => spawn(agent, mode, taskId, persistent)}
        />
      )}
      {modal?.type === 'send' && (
        <SendMsgModal
          session={modal.session}
          onClose={() => setModal(null)}
          onSend={sendMessage}
        />
      )}
      {modal?.type === 'recover' && (
        <RecoverModal
          session={modal.session}
          onClose={() => setModal(null)}
          onRecover={recover}
        />
      )}
    </div>
  )
}
