import { useQuery } from '@tanstack/react-query'
import { useProjectContext } from '@/app/ProjectContext'
import { StatusBadge } from '@/components/StatusBadge'
import { api } from '@/lib/api-client'
import { projectActionsApi } from '@/lib/project-actions-api'
import { useSessions } from '@/features/sessions/hooks'
import { useTasks, useTaskActions } from '@/features/tasks/hooks'
import { useNotifications } from '@/hooks/useNotifications'
import { useState } from 'react'

// ─── Types ────────────────────────────────────────────────────────────────────

type AgentStatus = {
  name: string
  role: string
  runtime: string
  enabled: boolean
  tmux_pane?: string
  status?: string
  persistent?: boolean
}

type ProjectStatus = {
  project_id: string
  project_name: string
  project_path: string
  agents: AgentStatus[]
  active_count: number
  idle_count: number
}

// ─── Shared ───────────────────────────────────────────────────────────────────

function Empty({ message }: { message: string }) {
  return (
    <div className="h-full flex items-center justify-center text-sm text-gray-600">
      {message}
    </div>
  )
}

function SectionCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="bg-surface-raised border border-surface-border rounded-lg p-4">
      <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">{title}</h3>
      {children}
    </div>
  )
}

// ─── DashboardView ────────────────────────────────────────────────────────────

export function DashboardView() {
  const { selectedId } = useProjectContext()
  const [pausePending, setPausePending] = useState(false)
  const [resumePending, setResumePending] = useState(false)
  const [actionFeedback, setActionFeedback] = useState<string | null>(null)

  const statusQuery = useQuery({
    queryKey: ['projects', selectedId, 'status'],
    queryFn: () => api.get<ProjectStatus>(`/api/v1/projects/${selectedId}/status`),
    enabled: !!selectedId,
    refetchInterval: 5_000,
  })

  const {
    data: sessions = [],
    approve,
    deny,
    recover,
    approveMutation,
    denyMutation,
    recoverMutation,
  } = useSessions(selectedId)

  const { data: tasks = [] } = useTasks(selectedId)
  const { acceptMutation } = useTaskActions(selectedId)

  useNotifications(
    sessions.map((s) => ({ id: s.id, label: s.agent_name, status: s.status })),
    tasks.map((t) => ({ id: t.id, label: t.title, status: t.status })),
    statusQuery.data?.project_name,
  )

  if (!selectedId) return <Empty message="Select a project." />
  if (statusQuery.isLoading) return <Empty message="Loading…" />

  const status = statusQuery.data
  const activeSessions = sessions.filter((s) =>
    ['Working', 'Booting', 'WaitingApproval', 'WaitingHandoff'].includes(s.status),
  )
  const waitingApproval = sessions.filter((s) => s.status === 'WaitingApproval')
  const failedSessions = sessions.filter((s) => s.status === 'Failed')
  const actionItems = tasks.filter((t) => ['WaitingHandoff', 'Blocked', 'NeedsAttention'].includes(t.status))
  const activeTasks = tasks.filter((t) => ['InProgress', 'Ready'].includes(t.status))

  function showFeedback(msg: string) {
    setActionFeedback(msg)
    setTimeout(() => setActionFeedback(null), 3000)
  }

  async function handlePauseAll() {
    if (!selectedId) return
    setPausePending(true)
    try {
      await projectActionsApi.pauseAll(selectedId)
      showFeedback('All sessions paused.')
    } catch (err) {
      showFeedback(err instanceof Error ? err.message : 'Failed')
    } finally {
      setPausePending(false)
    }
  }

  async function handleResumeAll() {
    if (!selectedId) return
    setResumePending(true)
    try {
      await projectActionsApi.resumeAll(selectedId)
      showFeedback('All sessions resumed.')
    } catch (err) {
      showFeedback(err instanceof Error ? err.message : 'Failed')
    } finally {
      setResumePending(false)
    }
  }

  async function handleApprove(sessionId: string) {
    try {
      await approve(sessionId)
      showFeedback('Session approved.')
    } catch (err) {
      showFeedback(err instanceof Error ? err.message : 'Failed to approve')
    }
  }

  async function handleDeny(sessionId: string) {
    try {
      await deny(sessionId)
      showFeedback('Session denied.')
    } catch (err) {
      showFeedback(err instanceof Error ? err.message : 'Failed to deny')
    }
  }

  async function handleRecover(sessionId: string) {
    try {
      await recover(sessionId)
      showFeedback('Recovery initiated.')
    } catch (err) {
      showFeedback(err instanceof Error ? err.message : 'Failed to recover')
    }
  }

  async function handleAccept(taskId: string) {
    try {
      await acceptMutation.mutateAsync({ taskId, force: false })
      showFeedback('Task accepted.')
    } catch (err) {
      showFeedback(err instanceof Error ? err.message : 'Accept failed — use Tasks tab for Force Accept')
    }
  }

  return (
    <div className="h-full overflow-auto p-4 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold text-gray-300">Dashboard</h2>
          {status && (
            <div className="flex items-center gap-2 mt-0.5">
              <p className="text-xs text-gray-500">{status.project_name}</p>
              <p className="text-xs text-gray-700 font-mono" title={status.project_path}>
                {status.project_path}
              </p>
            </div>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={handlePauseAll}
            disabled={pausePending}
            className="px-3 py-1.5 text-xs border border-surface-border text-gray-400 rounded hover:text-accent-yellow hover:border-accent-yellow/40 disabled:opacity-50 transition-colors"
          >
            {pausePending ? '…' : 'Pause All'}
          </button>
          <button
            onClick={handleResumeAll}
            disabled={resumePending}
            className="px-3 py-1.5 text-xs border border-surface-border text-gray-400 rounded hover:text-accent-green hover:border-accent-green/40 disabled:opacity-50 transition-colors"
          >
            {resumePending ? '…' : 'Resume All'}
          </button>
        </div>
      </div>

      {actionFeedback && (
        <p className="text-xs text-accent-green">{actionFeedback}</p>
      )}

      {/* Stats row */}
      <div className="grid grid-cols-4 gap-3">
        {[
          { label: 'Active Sessions', value: status?.active_count ?? 0, color: 'text-accent-green' },
          { label: 'Idle Sessions', value: status?.idle_count ?? 0, color: 'text-gray-400' },
          { label: 'Active Tasks', value: activeTasks.length, color: 'text-accent' },
          { label: 'Need Attention', value: actionItems.length, color: 'text-accent-yellow' },
        ].map((stat) => (
          <div key={stat.label} className="bg-surface-raised border border-surface-border rounded-lg p-3">
            <p className={`text-2xl font-bold ${stat.color}`}>{stat.value}</p>
            <p className="text-xs text-gray-500 mt-0.5">{stat.label}</p>
          </div>
        ))}
      </div>

      {/* Waiting Approval — highest priority */}
      {waitingApproval.length > 0 && (
        <SectionCard title="Waiting Approval">
          <div className="space-y-1">
            {waitingApproval.map((s) => (
              <div key={s.id} className="flex items-center justify-between px-3 py-2 bg-accent-yellow/10 border border-accent-yellow/30 rounded text-xs">
                <div className="flex items-center gap-2 min-w-0">
                  <span className="text-gray-300 font-medium">{s.agent_name}</span>
                  {s.task_id && <span className="text-gray-500 font-mono">{s.task_id}</span>}
                  <StatusBadge status={s.status} />
                </div>
                <div className="flex items-center gap-2 ml-3 shrink-0">
                  <button
                    onClick={() => handleApprove(s.id)}
                    disabled={approveMutation.isPending}
                    className="px-2 py-1 text-xs text-accent-green border border-accent-green/30 rounded hover:bg-accent-green/10 disabled:opacity-50 transition-colors"
                  >
                    Approve
                  </button>
                  <button
                    onClick={() => handleDeny(s.id)}
                    disabled={denyMutation.isPending}
                    className="px-2 py-1 text-xs text-accent-red border border-accent-red/30 rounded hover:bg-accent-red/10 disabled:opacity-50 transition-colors"
                  >
                    Deny
                  </button>
                </div>
              </div>
            ))}
          </div>
        </SectionCard>
      )}

      {/* Failed sessions */}
      {failedSessions.length > 0 && (
        <SectionCard title="Failed Sessions">
          <div className="space-y-1">
            {failedSessions.map((s) => (
              <div key={s.id} className="flex items-center justify-between px-3 py-2 bg-accent-red/10 border border-accent-red/20 rounded text-xs">
                <div className="flex items-center gap-2">
                  <span className="text-gray-300">{s.agent_name}</span>
                  <StatusBadge status={s.status} />
                </div>
                <button
                  onClick={() => handleRecover(s.id)}
                  disabled={recoverMutation.isPending}
                  className="px-2 py-1 text-xs text-accent-yellow border border-accent-yellow/30 rounded hover:bg-accent-yellow/10 disabled:opacity-50 transition-colors"
                >
                  Recover
                </button>
              </div>
            ))}
          </div>
        </SectionCard>
      )}

      {/* Tasks needing action */}
      {actionItems.length > 0 && (
        <SectionCard title="Tasks Needing Attention">
          <div className="space-y-1">
            {actionItems.map((t) => (
              <div key={t.id} className="flex items-center justify-between px-3 py-2 bg-surface border border-surface-border rounded text-xs">
                <div className="flex items-center gap-2 min-w-0">
                  <span className="text-gray-500 font-mono shrink-0">{t.id}</span>
                  <span className="text-gray-300 truncate">{t.title}</span>
                  <StatusBadge status={t.status} />
                </div>
                <button
                  onClick={() => handleAccept(t.id)}
                  disabled={acceptMutation.isPending}
                  className="ml-3 shrink-0 px-2 py-1 text-xs text-accent-green border border-accent-green/30 rounded hover:bg-accent-green/10 disabled:opacity-50 transition-colors"
                >
                  Accept
                </button>
              </div>
            ))}
          </div>
        </SectionCard>
      )}

      {/* Agent overview */}
      {status && status.agents.length > 0 && (
        <SectionCard title="Agents">
          <div className="space-y-1">
            {status.agents.map((agent) => (
              <div key={agent.name} className="flex items-center justify-between px-3 py-2 bg-surface border border-surface-border rounded text-xs">
                <div className="flex items-center gap-3">
                  <span className="text-gray-300 font-medium">{agent.name}</span>
                  <span className="text-gray-600">{agent.role}</span>
                  <span className="text-gray-700 font-mono">{agent.runtime}</span>
                </div>
                <div className="flex items-center gap-1.5">
                  {agent.status ? (
                    <StatusBadge status={agent.status} />
                  ) : (
                    <span className="text-gray-700 text-xs">No session</span>
                  )}
                  {agent.persistent && (
                    <span className="text-xs text-accent/70 border border-accent/20 rounded px-1.5 py-0.5">
                      keep-alive
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </SectionCard>
      )}

      {/* Active sessions */}
      {activeSessions.length > 0 && (
        <SectionCard title="Active Sessions">
          <div className="space-y-1">
            {activeSessions.map((s) => (
              <div key={s.id} className="flex items-center justify-between px-3 py-2 bg-surface border border-surface-border rounded text-xs">
                <div className="flex items-center gap-3">
                  <span className="text-gray-300">{s.agent_name}</span>
                  {s.task_id && <span className="text-gray-600 font-mono">{s.task_id}</span>}
                  {s.tmux_pane && <span className="text-gray-700 font-mono">{s.tmux_pane}</span>}
                </div>
                <StatusBadge status={s.status} />
              </div>
            ))}
          </div>
        </SectionCard>
      )}

      {waitingApproval.length === 0 && failedSessions.length === 0 && actionItems.length === 0 && activeSessions.length === 0 && (
        <div className="flex items-center justify-center py-12 text-xs text-gray-600">
          All clear — no action items.
        </div>
      )}
    </div>
  )
}
