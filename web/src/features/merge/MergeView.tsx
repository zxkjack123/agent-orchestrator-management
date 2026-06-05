import { useState } from 'react'
import { useProjectContext } from '@/app/ProjectContext'
import { useTasks } from '@/features/tasks/hooks'
import { api } from '@/lib/api-client'

function Empty({ message }: { message: string }) {
  return (
    <div className="h-full flex items-center justify-center text-sm text-gray-600">
      {message}
    </div>
  )
}

type StepResult = { label: string; output: string; error?: string }

export function MergeView() {
  const { selectedId } = useProjectContext()
  const { data: tasks = [] } = useTasks(selectedId)

  const [taskId, setTaskId] = useState('')
  const [steps, setSteps] = useState<StepResult[]>([])
  const [pending, setPending] = useState<string | null>(null)

  if (!selectedId) return <Empty message="Select a project." />

  const doneTasks = tasks.filter((t) => ['Done', 'WaitingHandoff'].includes(t.status))

  async function run(label: string, fn: () => Promise<{ output: string }>) {
    setPending(label)
    try {
      const res = await fn()
      setSteps((prev) => [...prev, { label, output: res.output }])
    } catch (err) {
      setSteps((prev) => [
        ...prev,
        { label, output: '', error: err instanceof Error ? err.message : 'Failed' },
      ])
    } finally {
      setPending(null)
    }
  }

  async function handleCheck() {
    if (!taskId) return
    setSteps([])
    await run('merge check', () =>
      api.post<{ status: string; output: string }>(
        `/api/v1/projects/${selectedId}/merge/check`,
        { task_id: taskId },
      ),
    )
  }

  async function handlePrepare() {
    if (!taskId) return
    await run('merge prepare', () =>
      api.post<{ status: string; output: string }>(
        `/api/v1/projects/${selectedId}/merge/prepare`,
        { task_id: taskId },
      ),
    )
  }

  async function handleCommit() {
    if (!taskId) return
    await run('merge commit', () =>
      api.post<{ status: string; output: string }>(
        `/api/v1/projects/${selectedId}/merge/commit`,
        { task_id: taskId },
      ),
    )
  }

  return (
    <div className="h-full overflow-auto p-4 space-y-4">
      <h2 className="text-sm font-semibold text-gray-300">Merge</h2>

      {/* Task selector */}
      <div className="bg-surface-raised border border-surface-border rounded-lg p-4 space-y-3">
        <p className="text-xs text-gray-500">Select task to merge</p>
        {doneTasks.length > 0 ? (
          <select
            value={taskId}
            onChange={(e) => setTaskId(e.target.value)}
            className="w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent"
          >
            <option value="">— select task —</option>
            {doneTasks.map((t) => (
              <option key={t.id} value={t.id}>
                {t.id} — {t.title}
              </option>
            ))}
          </select>
        ) : (
          <input
            type="text"
            placeholder="TASK-xxx"
            value={taskId}
            onChange={(e) => setTaskId(e.target.value)}
            className="w-full bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent"
          />
        )}
      </div>

      {/* Actions */}
      <div className="grid grid-cols-3 gap-3">
        <button
          onClick={handleCheck}
          disabled={!!pending || !taskId}
          className="px-3 py-3 text-xs bg-surface-raised border border-surface-border text-gray-400 rounded-lg hover:text-gray-300 hover:border-gray-500 disabled:opacity-50 transition-colors"
        >
          <p className="font-medium">Check</p>
          <p className="text-gray-600 mt-0.5">Verify readiness</p>
        </button>
        <button
          onClick={handlePrepare}
          disabled={!!pending || !taskId}
          className="px-3 py-3 text-xs bg-surface-raised border border-surface-border text-gray-400 rounded-lg hover:text-gray-300 hover:border-accent/40 disabled:opacity-50 transition-colors"
        >
          <p className="font-medium">Prepare</p>
          <p className="text-gray-600 mt-0.5">Create merge plan</p>
        </button>
        <button
          onClick={handleCommit}
          disabled={!!pending || !taskId}
          className="px-3 py-3 text-xs bg-accent/20 border border-accent/30 text-accent rounded-lg hover:bg-accent/30 disabled:opacity-50 transition-colors"
        >
          <p className="font-medium">Commit</p>
          <p className="text-accent/60 mt-0.5">Merge to main</p>
        </button>
      </div>

      {pending && (
        <p className="text-xs text-gray-500 animate-pulse">Running {pending}…</p>
      )}

      {/* Step log */}
      {steps.length > 0 && (
        <div className="space-y-2">
          {steps.map((s, i) => (
            <div
              key={i}
              className={`rounded-lg border p-3 ${
                s.error
                  ? 'bg-accent-red/10 border-accent-red/20'
                  : 'bg-surface-raised border-surface-border'
              }`}
            >
              <p className="text-xs font-mono text-gray-500 mb-1">$ aom {s.label}</p>
              {s.error ? (
                <pre className="text-xs text-accent-red whitespace-pre-wrap">{s.error}</pre>
              ) : (
                <pre className="text-xs text-gray-300 whitespace-pre-wrap">{s.output || '(ok)'}</pre>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
