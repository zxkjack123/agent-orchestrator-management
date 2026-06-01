import { useProjectContext } from '@/app/ProjectContext'
import { StatusBadge } from '@/components/StatusBadge'
import { useSessions } from './hooks'

export function SessionsView() {
  const { selectedId } = useProjectContext()
  const { data: sessions = [], isLoading, stop } = useSessions(selectedId)

  if (!selectedId) return <Empty message="Select a project." />
  if (isLoading) return <Empty message="Loading…" />

  return (
    <div className="h-full overflow-auto p-4">
      <h2 className="text-sm font-semibold text-gray-300 mb-3">Sessions</h2>
      {sessions.length === 0 ? (
        <p className="text-xs text-gray-600">No sessions.</p>
      ) : (
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-gray-600 border-b border-surface-border">
              <th className="pb-2 font-medium">Agent</th>
              <th className="pb-2 font-medium">Status</th>
              <th className="pb-2 font-medium">Task</th>
              <th className="pb-2 font-medium">Pane</th>
              <th className="pb-2 font-medium">Created</th>
              <th className="pb-2" />
            </tr>
          </thead>
          <tbody>
            {sessions.map((s) => (
              <tr key={s.id} className="border-b border-surface-border/50 hover:bg-surface-raised">
                <td className="py-2 pr-4 text-gray-300">{s.agent_name}</td>
                <td className="py-2 pr-4"><StatusBadge status={s.status} /></td>
                <td className="py-2 pr-4 text-gray-500">{s.task_id ?? '—'}</td>
                <td className="py-2 pr-4 text-gray-600 font-mono">{s.tmux_pane ?? '—'}</td>
                <td className="py-2 pr-4 text-gray-600">{s.created_at.slice(0, 16)}</td>
                <td className="py-2">
                  {['Working', 'Idle', 'Booting'].includes(s.status) && (
                    <button
                      onClick={() => stop(s.agent_name)}
                      className="text-xs text-accent-red hover:underline"
                    >
                      Stop
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

function Empty({ message }: { message: string }) {
  return (
    <div className="h-full flex items-center justify-center text-sm text-gray-600">
      {message}
    </div>
  )
}
