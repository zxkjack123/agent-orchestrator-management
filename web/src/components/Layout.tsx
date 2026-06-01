import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Sidebar } from '@/features/projects/Sidebar'
import { NavTabs } from './NavTabs'
import { useProjectContext } from '@/app/ProjectContext'
import { useProjects } from '@/features/projects/hooks'

export function Layout({ children }: { children: React.ReactNode }) {
  const { selectedId, setSelectedId } = useProjectContext()
  const { data: projects = [] } = useProjects()
  const navigate = useNavigate()

  // Auto-select the first project on load, and navigate to war-room.
  useEffect(() => {
    if (!selectedId && projects.length > 0) {
      const first = projects[0]
      setSelectedId(first.id)
      navigate(`/projects/${first.id}/war-room`, { replace: true })
    }
  }, [projects, selectedId, setSelectedId, navigate])

  // When selection changes, navigate to war-room of the new project.
  useEffect(() => {
    if (selectedId) {
      navigate(`/projects/${selectedId}/war-room`, { replace: true })
    }
  }, [selectedId, navigate])

  return (
    <div className="flex h-screen overflow-hidden bg-surface">
      {/* Left sidebar — project list */}
      <Sidebar />

      {/* Main content */}
      <div className="flex flex-col flex-1 min-w-0">
        <NavTabs />
        <main className="flex-1 overflow-hidden">
          {children}
        </main>
      </div>
    </div>
  )
}
