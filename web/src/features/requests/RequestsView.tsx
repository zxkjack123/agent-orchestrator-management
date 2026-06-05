import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useProjectContext } from '@/app/ProjectContext'
import { api } from '@/lib/api-client'

type TaskRequest = {
  id: string
  title: string
  requested_by: string
  parent_task: string
  priority: string
  status: string
  reason: string
}

function Empty({ message }: { message: string }) {
  return (
    <div className="h-full flex items-center justify-center text-sm text-gray-600">
      {message}
    </div>
  )
}

function StatusPill({ status }: { status: string }) {
  const cls =
    status === 'pending'
      ? 'bg-accent-yellow/20 text-accent-yellow border-accent-yellow/30'
      : status === 'approved'
      ? 'bg-accent-green/20 text-accent-green border-accent-green/30'
      : 'bg-accent-red/20 text-accent-red border-accent-red/30'
  return (
    <span className={`inline-flex px-2 py-0.5 text-xs rounded border ${cls}`}>{status}</span>
  )
}

export function RequestsView() {
  const { selectedId } = useProjectContext()
  const qc = useQueryClient()
  const [feedback, setFeedback] = useState<string | null>(null)

  const { data: requests = [], isLoading } = useQuery({
    queryKey: ['projects', selectedId, 'requests'],
    queryFn: () => api.get<TaskRequest[]>(`/api/v1/projects/${selectedId}/requests`),
    enabled: !!selectedId,
    refetchInterval: 10_000,
  })

  const approveMutation = useMutation({
    mutationFn: (rid: string) =>
      api.post<{ status: string; output: string }>(
        `/api/v1/projects/${selectedId}/requests/${rid}/approve`,
        {},
      ),
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ['projects', selectedId] })
      setFeedback(res.output || 'Approved.')
      setTimeout(() => setFeedback(null), 4000)
    },
    onError: (err) => {
      setFeedback(err instanceof Error ? err.message : 'Failed to approve')
      setTimeout(() => setFeedback(null), 4000)
    },
  })

  const rejectMutation = useMutation({
    mutationFn: (rid: string) =>
      api.post<{ status: string; output: string }>(
        `/api/v1/projects/${selectedId}/requests/${rid}/reject`,
        {},
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['projects', selectedId] })
      setFeedback('Rejected.')
      setTimeout(() => setFeedback(null), 3000)
    },
    onError: (err) => {
      setFeedback(err instanceof Error ? err.message : 'Failed to reject')
      setTimeout(() => setFeedback(null), 3000)
    },
  })

  if (!selectedId) return <Empty message="Select a project." />
  if (isLoading) return <Empty message="Loading…" />

  const pending = requests.filter((r) => r.status === 'pending')
  const others = requests.filter((r) => r.status !== 'pending')

  return (
    <div className="h-full overflow-auto p-4 space-y-4">
      <h2 className="text-sm font-semibold text-gray-300">Agent Requests</h2>

      {feedback && (
        <div className="px-3 py-2 bg-surface-raised border border-surface-border rounded text-xs text-accent-green">
          {feedback}
        </div>
      )}

      {requests.length === 0 ? (
        <p className="text-xs text-gray-600">No requests yet.</p>
      ) : (
        <div className="space-y-4">
          {pending.length > 0 && (
            <div>
              <p className="text-xs text-gray-500 uppercase tracking-wider mb-2">Pending ({pending.length})</p>
              <div className="space-y-2">
                {pending.map((req) => (
                  <div
                    key={req.id}
                    className="bg-surface-raised border border-accent-yellow/30 rounded-lg p-4"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-gray-300 font-medium text-sm">{req.title}</span>
                          <StatusPill status={req.status} />
                          {req.priority && req.priority !== '-' && (
                            <span className="text-xs text-gray-600 border border-surface-border rounded px-1.5 py-0.5">
                              {req.priority}
                            </span>
                          )}
                        </div>
                        <p className="text-xs text-gray-600 mt-1 font-mono">{req.id}</p>
                        {req.requested_by && req.requested_by !== '-' && (
                          <p className="text-xs text-gray-500 mt-1">From: {req.requested_by}</p>
                        )}
                        {req.reason && req.reason !== '-' && (
                          <p className="text-xs text-gray-500 mt-1 italic">"{req.reason}"</p>
                        )}
                        {req.parent_task && req.parent_task !== '-' && (
                          <p className="text-xs text-gray-600 mt-1">Parent: {req.parent_task}</p>
                        )}
                      </div>
                      <div className="flex items-center gap-2 shrink-0">
                        <button
                          onClick={() => approveMutation.mutate(req.id)}
                          disabled={approveMutation.isPending}
                          className="px-3 py-1.5 text-xs text-accent-green border border-accent-green/30 rounded hover:bg-accent-green/10 disabled:opacity-50 transition-colors"
                        >
                          Approve
                        </button>
                        <button
                          onClick={() => rejectMutation.mutate(req.id)}
                          disabled={rejectMutation.isPending}
                          className="px-3 py-1.5 text-xs text-accent-red border border-accent-red/20 rounded hover:bg-accent-red/10 disabled:opacity-50 transition-colors"
                        >
                          Reject
                        </button>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {others.length > 0 && (
            <div>
              <p className="text-xs text-gray-500 uppercase tracking-wider mb-2">History</p>
              <div className="space-y-1">
                {others.map((req) => (
                  <div
                    key={req.id}
                    className="flex items-center justify-between px-3 py-2 bg-surface border border-surface-border rounded text-xs"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="text-gray-500 font-mono shrink-0">{req.id}</span>
                      <span className="text-gray-400 truncate">{req.title}</span>
                    </div>
                    <StatusPill status={req.status} />
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
