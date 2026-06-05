import { useState } from 'react'
import { AlertTriangle, Monitor } from 'lucide-react'
import { useProjectContext } from '@/app/ProjectContext'
import { useProjectAgents } from '@/features/projects/hooks'
import { projectsApi } from '@/features/projects/api'
import { useQueryClient } from '@tanstack/react-query'
import { TerminalPane } from '@/features/terminal/TerminalPane'
import { useSessions } from '@/features/sessions/hooks'

export function WarRoom() {
  const { selectedId } = useProjectContext()
  const { data: agents = [], isLoading } = useProjectAgents(selectedId)
  const { stop } = useSessions(selectedId)
  const qc = useQueryClient()
  const [isolatingAll, setIsolatingAll] = useState(false)

  async function handleIsolateAll() {
    if (!selectedId) return
    setIsolatingAll(true)
    try {
      await Promise.all(
        sharedAgents.map((a) => projectsApi.isolateSession(selectedId, a.name, 'real'))
      )
      await qc.refetchQueries({ queryKey: ['projects', selectedId, 'agents'] })
      await qc.refetchQueries({ queryKey: ['projects', selectedId, 'sessions'] })
    } finally {
      setIsolatingAll(false)
    }
  }

  // Only show agents with a live session (not Stopped/Archived/Failed).
  const activeAgents = agents.filter(
    (a) => a.tmux_pane && ['Idle', 'Working', 'Booting', 'WaitingApproval', 'WaitingHandoff'].includes(a.status ?? ''),
  )

  const sharedAgents = activeAgents.filter((a) => a.is_shared_session)

  if (!selectedId) {
    return <EmptyState message="Select a project from the sidebar to begin." />
  }

  if (isLoading) {
    return <EmptyState message="Loading agents…" />
  }

  if (activeAgents.length === 0) {
    return (
      <EmptyState message="No active sessions. Spawn an agent to get started." />
    )
  }

  return (
    <div className="h-full flex flex-col overflow-hidden">
      {sharedAgents.length > 0 && (
        <div className="flex-none mx-3 mt-3 p-3 rounded-lg border border-yellow-500/40 bg-yellow-500/10 flex items-start gap-2">
          <AlertTriangle size={16} className="text-yellow-400 mt-0.5 flex-none" />
          <div className="flex-1 min-w-0">
            <p className="text-xs text-yellow-300 font-medium mb-1">Shared session detected</p>
            <p className="text-xs text-yellow-200/70 mb-2">
              {sharedAgents.map((a) => a.name).join(', ')} share a tmux window — terminal view may be clipped. Isolate each agent for a full-size dedicated terminal.
            </p>
            <button
              disabled={isolatingAll}
              onClick={handleIsolateAll}
              className="px-2 py-1 text-xs rounded bg-yellow-500/30 border border-yellow-500/60 text-yellow-100 font-medium hover:bg-yellow-500/40 disabled:opacity-50 transition-colors"
            >
              {isolatingAll ? 'Isolating…' : 'Isolate All'}
            </button>
          </div>
        </div>
      )}
      <div className="flex-1 p-3 grid gap-3 auto-rows-fr min-h-0" style={gridStyle(activeAgents.length)}>
        {activeAgents.map((agent) => (
          <TerminalPane
            key={agent.tmux_pane}
            agentName={agent.name}
            paneId={agent.tmux_pane!}
            status={agent.status}
            onStop={() => stop(agent.name)}
          />
        ))}
      </div>
    </div>
  )
}

// gridStyle computes explicit CSS grid tracks so each cell gets an equal share
// of the container height from the start — avoids xterm.js fit timing issues.
function gridStyle(count: number): React.CSSProperties {
  const cols = count <= 1 ? 1 : count <= 4 ? 2 : 3
  const rows = Math.ceil(count / cols)
  return {
    gridTemplateColumns: `repeat(${cols}, 1fr)`,
    gridTemplateRows: `repeat(${rows}, 1fr)`,
  }
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="h-full flex flex-col items-center justify-center gap-3 text-gray-600">
      <Monitor size={40} strokeWidth={1} />
      <p className="text-sm">{message}</p>
    </div>
  )
}
