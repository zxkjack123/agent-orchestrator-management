import { useState } from 'react'
import { Plus, X } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useProjectContext } from '@/app/ProjectContext'
import { useProjectAgents } from '@/features/projects/hooks'
import { StatusBadge } from '@/components/StatusBadge'
import { useTasks, useTaskActions } from './hooks'
import { api } from '@/lib/api-client'
import type { Task } from './types'
import type { TaskSignal } from './api'

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

function PriorityLabel({ priority }: { priority: number }) {
  if (priority >= 10)
    return (
      <span className="inline-flex items-center px-2 py-0.5 text-xs rounded border bg-accent-yellow/20 text-accent-yellow border-accent-yellow/30 font-medium">
        High
      </span>
    )
  if (priority <= -10) return <span className="text-gray-600 text-xs">Low</span>
  return <span className="text-gray-500 text-xs">Normal</span>
}

// ─── Signal Modal ─────────────────────────────────────────────────────────────

const SIGNALS: { value: TaskSignal; label: string; description: string }[] = [
  { value: 'task.completed', label: 'task.completed', description: 'Mark task as fully done' },
  { value: 'handoff.prepared', label: 'handoff.prepared', description: 'Handoff doc is ready for review' },
  { value: 'checkpoint.created', label: 'checkpoint.created', description: 'Intermediate checkpoint saved' },
  { value: 'step.completed', label: 'step.completed', description: 'One step finished' },
]

function SignalModal({
  task,
  onClose,
  onSignal,
}: {
  task: Task
  onClose: () => void
  onSignal: (signal: TaskSignal) => Promise<void>
}) {
  const [selected, setSelected] = useState<TaskSignal>('task.completed')
  const [showMore, setShowMore] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [output, setOutput] = useState<string | null>(null)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setPending(true)
    setError(null)
    try {
      await onSignal(selected)
      setOutput('Signal sent.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed')
    } finally {
      setPending(false)
    }
  }

  const moreSignals = SIGNALS.filter((s) => s.value !== 'task.completed')

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-md p-6 shadow-xl">
        <h3 className="text-sm font-semibold text-gray-300 mb-1">Send Signal</h3>
        <p className="text-xs text-gray-500 mb-4 truncate">
          Task: <span className="text-gray-400">{task.id}</span>{' '}
          <span className="text-gray-600 truncate">{task.title}</span>
        </p>
        {output ? (
          <div className="space-y-3">
            <p className="text-xs text-accent-green">{output}</p>
            <div className="flex justify-end">
              <button onClick={onClose} className={primaryBtn}>Close</button>
            </div>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-3">
            {/* Primary signal */}
            <label
              className={[
                'flex items-start gap-3 px-3 py-2.5 rounded-lg border cursor-pointer transition-colors',
                selected === 'task.completed'
                  ? 'border-accent bg-accent/10'
                  : 'border-surface-border hover:bg-surface',
              ].join(' ')}
            >
              <input
                type="radio"
                name="signal"
                value="task.completed"
                checked={selected === 'task.completed'}
                onChange={() => setSelected('task.completed')}
                className="mt-0.5 accent-accent"
              />
              <div>
                <p className="text-xs font-mono text-gray-200">task.completed</p>
                <p className="text-xs text-gray-500">Mark task as fully done</p>
              </div>
            </label>

            {/* More signals toggle */}
            <button
              type="button"
              onClick={() => setShowMore((v) => !v)}
              className="text-xs text-gray-600 hover:text-gray-400 transition-colors flex items-center gap-1"
            >
              {showMore ? '▾' : '▸'} More signals
            </button>

            {showMore && (
              <div className="space-y-1">
                {moreSignals.map((s) => (
                  <label
                    key={s.value}
                    className={[
                      'flex items-start gap-3 px-3 py-2.5 rounded-lg border cursor-pointer transition-colors',
                      selected === s.value
                        ? 'border-accent bg-accent/10'
                        : 'border-surface-border hover:bg-surface',
                    ].join(' ')}
                  >
                    <input
                      type="radio"
                      name="signal"
                      value={s.value}
                      checked={selected === s.value}
                      onChange={() => setSelected(s.value)}
                      className="mt-0.5 accent-accent"
                    />
                    <div>
                      <p className="text-xs font-mono text-gray-200">{s.label}</p>
                      <p className="text-xs text-gray-500">{s.description}</p>
                    </div>
                  </label>
                ))}
              </div>
            )}

            {error && <p className="text-xs text-accent-red">{error}</p>}
            <div className="flex justify-end gap-2">
              <button type="button" onClick={onClose} className={secondaryBtn}>Cancel</button>
              <button type="submit" disabled={pending} className={primaryBtn}>
                {pending ? 'Sending…' : 'Send Signal'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}

// ─── Accept Result Modal ──────────────────────────────────────────────────────

function AcceptModal({
  task,
  onClose,
  onAccept,
}: {
  task: Task
  onClose: () => void
  onAccept: (force: boolean) => Promise<void>
}) {
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [output, setOutput] = useState<string | null>(null)

  async function run(force: boolean) {
    setPending(true)
    setError(null)
    try {
      await onAccept(force)
      setOutput('Task accepted.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed')
    } finally {
      setPending(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-md p-6 shadow-xl">
        <h3 className="text-sm font-semibold text-gray-300 mb-1">Accept Task</h3>
        <p className="text-xs text-gray-500 mb-4">
          <span className="text-gray-400">{task.id}</span>{' '}
          <span className="text-gray-600">{task.title}</span>
        </p>
        {output ? (
          <div className="space-y-3">
            <p className="text-xs text-accent-green">{output}</p>
            <div className="flex justify-end">
              <button onClick={onClose} className={primaryBtn}>Close</button>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-xs text-gray-400">
              Runs verify checks and merges the task branch. Use <strong>Force</strong> to skip failing checks.
            </p>
            {error && (
              <div className="rounded-lg bg-accent-red/10 border border-accent-red/30 px-3 py-2">
                <p className="text-xs text-accent-red font-medium mb-1">Failed</p>
                <pre className="text-xs text-accent-red/80 whitespace-pre-wrap break-words">{error}</pre>
              </div>
            )}
            <div className="flex justify-end gap-2">
              <button onClick={onClose} className={secondaryBtn}>Cancel</button>
              {error && (
                <button onClick={() => run(true)} disabled={pending} className="px-3 py-1.5 text-xs bg-accent-yellow/20 border border-accent-yellow/40 text-accent-yellow rounded hover:bg-accent-yellow/30 disabled:opacity-50 transition-colors">
                  {pending ? 'Accepting…' : 'Force Accept'}
                </button>
              )}
              <button onClick={() => run(false)} disabled={pending} className={primaryBtn}>
                {pending ? 'Accepting…' : 'Accept'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// ─── Create Task Modal ────────────────────────────────────────────────────────

const TASK_MODES = ['Direct', 'Bugfix', 'Requirements-first', 'Design-first']

function CreateTaskModal({
  projectId,
  onClose,
  onCreate,
}: {
  projectId: string
  onClose: () => void
  onCreate: (
    title: string,
    opts: { description?: string; mode?: string; agent?: string; role?: string },
  ) => Promise<void>
}) {
  const { data: agents = [] } = useProjectAgents(projectId)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [mode, setMode] = useState('Direct')
  const [agent, setAgent] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [output, setOutput] = useState<string | null>(null)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!title.trim()) return
    setPending(true)
    setError(null)
    try {
      await onCreate(title.trim(), { description: description.trim(), mode, agent: agent.trim() })
      setOutput('Task created.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create task')
    } finally {
      setPending(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-surface-raised border border-surface-border rounded-lg w-full max-w-md p-6 shadow-xl">
        <h3 className="text-sm font-semibold text-gray-300 mb-4">Create Task</h3>
        {output ? (
          <div className="space-y-3">
            <p className="text-xs text-accent-green">{output}</p>
            <div className="flex justify-end">
              <button onClick={onClose} className={primaryBtn}>Close</button>
            </div>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-3">
            <div>
              <label className="block text-xs text-gray-500 mb-1">Title <span className="text-accent-red">*</span></label>
              <input
                autoFocus
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="Short task title"
                className="w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent"
              />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Description <span className="text-gray-700">(optional)</span></label>
              <textarea
                rows={3}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="What needs to be done?"
                className="w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent resize-none"
              />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Mode</label>
              <select
                value={mode}
                onChange={(e) => setMode(e.target.value)}
                className="w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent"
              >
                {TASK_MODES.map((m) => <option key={m} value={m}>{m}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Assign to agent <span className="text-gray-700">(optional)</span></label>
              {agents.length > 0 ? (
                <select
                  value={agent}
                  onChange={(e) => setAgent(e.target.value)}
                  className="w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent"
                >
                  <option value="">— unassigned —</option>
                  {agents.map((a) => <option key={a.name} value={a.name}>{a.name}</option>)}
                </select>
              ) : (
                <input
                  type="text"
                  value={agent}
                  onChange={(e) => setAgent(e.target.value)}
                  placeholder="agent-name"
                  className="w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent"
                />
              )}
            </div>
            {error && (
              <div className="rounded-lg bg-accent-red/10 border border-accent-red/30 px-3 py-2">
                <pre className="text-xs text-accent-red whitespace-pre-wrap break-words">{error}</pre>
              </div>
            )}
            <div className="flex justify-end gap-2">
              <button type="button" onClick={onClose} className={secondaryBtn}>Cancel</button>
              <button type="submit" disabled={pending || !title.trim()} className={primaryBtn}>
                {pending ? 'Creating…' : 'Create'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}

// ─── Task Detail Drawer ───────────────────────────────────────────────────────

function TaskDrawer({
  task,
  projectId,
  onClose,
}: {
  task: Task
  projectId: string
  onClose: () => void
}) {
  const { data: artifact } = useQuery({
    queryKey: ['projects', projectId, 'tasks', task.id, 'artifact'],
    queryFn: () =>
      api.get<Record<string, string>>(
        `/api/v1/projects/${projectId}/tasks/${task.id}/artifact`,
      ),
    enabled: !!projectId && !!task.id,
  })

  const [activeTab, setActiveTab] = useState<'overview' | 'task.md' | 'handoff.md' | 'state.md'>('overview')
  const artifactFiles = ['task.md', 'handoff.md', 'state.md'] as const
  const availableTabs = artifactFiles.filter((f) => artifact?.[f])

  return (
    <div className="w-96 shrink-0 border-l border-surface-border bg-surface-raised flex flex-col h-full">
      {/* Header */}
      <div className="flex items-start justify-between px-4 py-3 border-b border-surface-border">
        <div className="min-w-0 flex-1 pr-2">
          <p className="text-xs font-mono text-gray-500">{task.id}</p>
          <p className="text-sm font-medium text-gray-300 mt-0.5 leading-snug">{task.title}</p>
        </div>
        <button onClick={onClose} className="text-gray-600 hover:text-gray-400 transition-colors shrink-0 mt-0.5">
          <X size={14} />
        </button>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-surface-border">
        {(['overview', ...availableTabs] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab as typeof activeTab)}
            className={[
              'px-3 py-2 text-xs border-b-2 transition-colors',
              activeTab === tab
                ? 'border-accent text-accent'
                : 'border-transparent text-gray-600 hover:text-gray-400',
            ].join(' ')}
          >
            {tab === 'overview' ? 'Overview' : tab}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-4">
        {activeTab === 'overview' ? (
          <div className="space-y-3 text-xs">
            <Row label="Status"><StatusBadge status={task.status} /></Row>
            <Row label="Mode"><span className="text-gray-400">{task.mode}</span></Row>
            <Row label="Priority"><span className="text-gray-400">{task.priority >= 10 ? 'High' : task.priority <= -10 ? 'Low' : 'Normal'}</span></Row>
            {task.preferred_agent && <Row label="Agent"><span className="text-gray-400">{task.preferred_agent}</span></Row>}
            {task.preferred_role && <Row label="Role"><span className="text-gray-400">{task.preferred_role}</span></Row>}
            <Row label="Created"><span className="text-gray-500">{task.created_at.slice(0, 16)}</span></Row>
            <Row label="Updated"><span className="text-gray-500">{task.updated_at.slice(0, 16)}</span></Row>
            {task.description && (
              <div className="mt-3">
                <p className="text-gray-500 mb-1.5">Description</p>
                <p className="text-gray-300 leading-relaxed whitespace-pre-wrap">{task.description}</p>
              </div>
            )}
          </div>
        ) : (
          <pre className="text-xs font-mono text-gray-300 whitespace-pre-wrap leading-relaxed">
            {artifact?.[activeTab] ?? ''}
          </pre>
        )}
      </div>
    </div>
  )
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <span className="text-gray-600 shrink-0">{label}</span>
      {children}
    </div>
  )
}

// ─── TasksView ────────────────────────────────────────────────────────────────

type Modal =
  | { type: 'signal'; task: Task }
  | { type: 'accept'; task: Task }
  | { type: 'create' }

const ACTIVE_STATUSES = ['InProgress', 'WaitingApproval', 'WaitingHandoff', 'Blocked', 'NeedsAttention']
const ACCEPT_STATUSES = ['WaitingHandoff', 'NeedsAttention', 'Blocked', 'InProgress']
const CLOSEABLE_STATUSES = ['Done', 'Blocked', 'NeedsAttention', 'Ready', 'InProgress']
const CANCELABLE_STATUSES = ['Draft', 'Planned', 'Ready', 'Blocked', 'NeedsAttention']

type TaskFilter = 'All' | 'Active' | 'Done'

export function TasksView() {
  const { selectedId } = useProjectContext()
  const { data: tasks = [], isLoading, error } = useTasks(selectedId)
  const { signalMutation, acceptMutation, createMutation, closeMutation, cancelMutation } = useTaskActions(selectedId)
  const [modal, setModal] = useState<Modal | null>(null)
  const [filter, setFilter] = useState<TaskFilter>('All')
  const [search, setSearch] = useState('')
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)

  if (!selectedId) return <Empty message="Select a project." />
  if (isLoading) return <Empty message="Loading…" />
  if (error) return <Empty message={`Error: ${(error as Error).message}`} />

  async function handleSignal(taskId: string, signal: TaskSignal) {
    await signalMutation.mutateAsync({ taskId, signal })
  }

  async function handleAccept(taskId: string, force: boolean) {
    await acceptMutation.mutateAsync({ taskId, force })
  }

  async function handleCreate(
    title: string,
    opts: { description?: string; mode?: string; agent?: string; role?: string },
  ) {
    await createMutation.mutateAsync({ title, ...opts })
  }

  function handleClose(taskId: string) {
    closeMutation.mutate({ taskId })
  }

  function handleCancel(taskId: string) {
    cancelMutation.mutate({ taskId })
  }

  const DONE_STATUSES = ['Done', 'Archived', 'Canceled']
  const filteredTasks = tasks.filter((t) => {
    if (filter === 'Active' && DONE_STATUSES.includes(t.status)) return false
    if (filter === 'Done' && !DONE_STATUSES.includes(t.status)) return false
    if (search && !t.title.toLowerCase().includes(search.toLowerCase()) && !t.id.toLowerCase().includes(search.toLowerCase())) return false
    return true
  })

  return (
    <div className="h-full flex overflow-hidden">
      <div className="flex-1 overflow-auto p-4">
      <div className="flex items-center justify-between mb-3 gap-2 flex-wrap">
        <div className="flex items-center gap-2 flex-wrap">
          <h2 className="text-sm font-semibold text-gray-300">Tasks</h2>
          <div className="flex items-center gap-1 bg-surface border border-surface-border rounded p-0.5">
            {(['All', 'Active', 'Done'] as TaskFilter[]).map((f) => (
              <button
                key={f}
                onClick={() => setFilter(f)}
                className={[
                  'px-2.5 py-1 text-xs rounded transition-colors',
                  filter === f
                    ? 'bg-accent/20 text-accent'
                    : 'text-gray-500 hover:text-gray-300',
                ].join(' ')}
              >
                {f}
              </button>
            ))}
          </div>
          <input
            type="text"
            placeholder="Search…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="bg-surface border border-surface-border text-gray-300 text-xs rounded px-2.5 py-1 w-36 focus:outline-none focus:border-accent focus:w-48 transition-all"
          />
        </div>
        <button
          onClick={() => setModal({ type: 'create' })}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-accent text-surface font-medium rounded hover:bg-accent/80 transition-colors"
        >
          <Plus size={12} /> New Task
        </button>
      </div>
      {filteredTasks.length === 0 ? (
        <p className="text-xs text-gray-600">{tasks.length === 0 ? 'No tasks yet.' : 'No tasks match this filter.'}</p>
      ) : (
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-gray-600 border-b border-surface-border">
              <th className="pb-2 font-medium">ID</th>
              <th className="pb-2 font-medium">Title</th>
              <th className="pb-2 font-medium">Status</th>
              <th className="pb-2 font-medium">Mode</th>
              <th className="pb-2 font-medium">Priority</th>
              <th className="pb-2 font-medium">Agent</th>
              <th className="pb-2 font-medium">Updated</th>
              <th className="pb-2 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {filteredTasks.map((task) => (
              <tr
                key={task.id}
                onClick={() => setSelectedTask(selectedTask?.id === task.id ? null : task)}
                className={[
                  'border-b border-surface-border/50 hover:bg-surface-raised cursor-pointer',
                  selectedTask?.id === task.id ? 'bg-accent/5' : '',
                ].join(' ')}
              >
                <td className="py-2 pr-3 text-gray-500 font-mono whitespace-nowrap">{task.id}</td>
                <td className="py-2 pr-3 text-gray-300 max-w-[200px] truncate">{task.title}</td>
                <td className="py-2 pr-3">
                  <StatusBadge status={task.status} />
                </td>
                <td className="py-2 pr-3 text-gray-500">{task.mode}</td>
                <td className="py-2 pr-3">
                  <PriorityLabel priority={task.priority} />
                </td>
                <td className="py-2 pr-3 text-gray-500">
                  {task.preferred_agent ?? task.preferred_role ?? <span className="text-gray-700">—</span>}
                </td>
                <td className="py-2 pr-3 text-gray-600 whitespace-nowrap">
                  {task.updated_at.slice(0, 16)}
                </td>
                <td className="py-2" onClick={(e) => e.stopPropagation()}>
                  <div className="flex items-center gap-2 flex-wrap">
                    {ACTIVE_STATUSES.includes(task.status) && (
                      <button
                        onClick={() => setModal({ type: 'signal', task })}
                        className="text-xs text-gray-400 hover:text-gray-200 border border-surface-border rounded px-2 py-1 hover:border-gray-500 transition-colors"
                      >
                        Signal
                      </button>
                    )}
                    {ACCEPT_STATUSES.includes(task.status) && (
                      <button
                        onClick={() => setModal({ type: 'accept', task })}
                        className="text-xs text-accent-green hover:text-accent-green/80 border border-accent-green/30 rounded px-2 py-1 hover:border-accent-green/60 transition-colors"
                      >
                        Accept
                      </button>
                    )}
                    {CLOSEABLE_STATUSES.includes(task.status) && (
                      <button
                        onClick={() => handleClose(task.id)}
                        disabled={closeMutation.isPending}
                        className="text-xs text-gray-500 hover:text-gray-300 border border-surface-border rounded px-2 py-1 transition-colors disabled:opacity-50"
                      >
                        Close
                      </button>
                    )}
                    {CANCELABLE_STATUSES.includes(task.status) && (
                      <button
                        onClick={() => handleCancel(task.id)}
                        disabled={cancelMutation.isPending}
                        className="text-xs text-accent-red/70 hover:text-accent-red border border-accent-red/20 rounded px-2 py-1 transition-colors disabled:opacity-50"
                      >
                        Cancel
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {modal?.type === 'signal' && (
        <SignalModal
          task={modal.task}
          onClose={() => setModal(null)}
          onSignal={(signal) => handleSignal(modal.task.id, signal)}
        />
      )}
      {modal?.type === 'accept' && (
        <AcceptModal
          task={modal.task}
          onClose={() => setModal(null)}
          onAccept={(force) => handleAccept(modal.task.id, force)}
        />
      )}
      {modal?.type === 'create' && selectedId && (
        <CreateTaskModal
          projectId={selectedId}
          onClose={() => setModal(null)}
          onCreate={handleCreate}
        />
      )}
      </div>

      {selectedTask && selectedId && (
        <TaskDrawer
          task={selectedTask}
          projectId={selectedId}
          onClose={() => setSelectedTask(null)}
        />
      )}
    </div>
  )
}
