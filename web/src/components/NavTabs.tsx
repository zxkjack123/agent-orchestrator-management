import { NavLink } from 'react-router-dom'
import { useProjectContext } from '@/app/ProjectContext'

const TABS = [
  { label: 'Dashboard',   path: 'dashboard' },
  { label: 'Agents',      path: 'agents' },
  { label: 'Roles',       path: 'roles' },
  { label: 'War Room',    path: 'war-room' },
  { label: 'Sessions',    path: 'sessions' },
  { label: 'Tasks',       path: 'tasks' },
  { label: 'Mailbox',     path: 'mailbox' },
  { label: 'Channel',     path: 'events' },
  { label: 'Requests',    path: 'requests' },
  { label: 'Team Brief',  path: 'team-brief' },
  { label: 'Merge',       path: 'merge' },
  { label: 'Metrics',     path: 'metrics' },
  { label: 'Doctor',      path: 'doctor' },
] as const

export function NavTabs() {
  const { selectedId } = useProjectContext()
  if (!selectedId) return null

  return (
    <nav className="flex gap-1 px-4 border-b border-surface-border bg-surface-raised">
      {TABS.map((tab) => (
        <NavLink
          key={tab.path}
          to={`/projects/${selectedId}/${tab.path}`}
          className={({ isActive }) =>
            [
              'px-4 py-2.5 text-xs font-medium border-b-2 transition-colors',
              isActive
                ? 'border-accent text-accent'
                : 'border-transparent text-gray-500 hover:text-gray-300',
            ].join(' ')
          }
        >
          {tab.label}
        </NavLink>
      ))}
    </nav>
  )
}
