import { useEffect } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { Sidebar } from '@/features/projects/Sidebar'
import { NavTabs } from './NavTabs'
import { useProjectContext } from '@/app/ProjectContext'
import { useProjects } from '@/features/projects/hooks'

export function Layout() {
  const { selectedId, setSelectedId } = useProjectContext()
  const { data: projects = [] } = useProjects()
  const { pathname } = useLocation()

  // Auto-select the first project when none is selected yet.
  useEffect(() => {
    if (!selectedId && projects.length > 0) {
      setSelectedId(projects[0].id)
    }
  }, [projects, selectedId, setSelectedId])

  // Only redirect when sitting at the bare root with a project selected.
  if (selectedId && pathname === '/') {
    return <Navigate to={`/projects/${selectedId}/war-room`} replace />
  }

  return (
    <div className="flex h-screen overflow-hidden bg-surface">
      <Sidebar />
      <div className="flex flex-col flex-1 min-w-0">
        <NavTabs />
        <main className="flex-1 overflow-hidden">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
