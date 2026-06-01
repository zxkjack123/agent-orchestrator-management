import { Monitor } from 'lucide-react'
import { useProjectContext } from '@/app/ProjectContext'
import { useProjectAgents } from '@/features/projects/hooks'
import { TerminalPane } from '@/features/terminal/TerminalPane'
import { useSessions } from '@/features/sessions/hooks'

export function WarRoom() {
  const { selectedId } = useProjectContext()
  const { data: agents = [], isLoading } = useProjectAgents(selectedId)
  const { stop } = useSessions(selectedId)

  // Only show agents that have an active tmux pane.
  const activeAgents = agents.filter((a) => a.tmux_pane)

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
    <div className="h-full p-3 grid gap-3 auto-rows-fr" style={gridStyle(activeAgents.length)}>
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
  )
}

// gridStyle computes CSS columns based on pane count for a clean war room layout.
function gridStyle(count: number): React.CSSProperties {
  const cols = count <= 1 ? 1 : count <= 4 ? 2 : 3
  return { gridTemplateColumns: `repeat(${cols}, 1fr)` }
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="h-full flex flex-col items-center justify-center gap-3 text-gray-600">
      <Monitor size={40} strokeWidth={1} />
      <p className="text-sm">{message}</p>
    </div>
  )
}
