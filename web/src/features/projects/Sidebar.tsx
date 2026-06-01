import { useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { useAddProject, useProjects, useRemoveProject } from './hooks'
import { useProjectContext } from '@/app/ProjectContext'
import type { Project } from './types'

export function Sidebar() {
  const { data: projects = [] } = useProjects()
  const { selectedId, setSelectedId } = useProjectContext()
  const [showAdd, setShowAdd] = useState(false)

  return (
    <aside className="flex flex-col w-14 bg-surface-raised border-r border-surface-border h-screen">
      {/* Project list */}
      <div className="flex-1 flex flex-col items-center py-3 gap-2 overflow-y-auto">
        {projects.map((p) => (
          <ProjectIcon
            key={p.id}
            project={p}
            selected={p.id === selectedId}
            onSelect={() => setSelectedId(p.id)}
          />
        ))}
      </div>

      {/* Add project button */}
      <div className="flex items-center justify-center py-3 border-t border-surface-border">
        <button
          onClick={() => setShowAdd(true)}
          className="w-9 h-9 rounded-lg flex items-center justify-center text-gray-500 hover:text-accent hover:bg-surface transition-colors"
          title="Add project"
        >
          <Plus size={18} />
        </button>
      </div>

      {showAdd && <AddProjectModal onClose={() => setShowAdd(false)} />}
    </aside>
  )
}

// ─── ProjectIcon ─────────────────────────────────────────────────────────────

function ProjectIcon({
  project,
  selected,
  onSelect,
}: {
  project: Project
  selected: boolean
  onSelect: () => void
}) {
  const remove = useRemoveProject()
  const initials = project.name.slice(0, 2).toUpperCase()

  return (
    <div className="relative group">
      <button
        onClick={onSelect}
        title={project.name}
        className={[
          'w-9 h-9 rounded-lg flex items-center justify-center text-xs font-bold transition-all',
          selected
            ? 'bg-accent text-white shadow-lg shadow-accent/30 rounded-xl'
            : 'bg-surface text-gray-400 hover:bg-surface-border hover:text-gray-200',
        ].join(' ')}
      >
        {initials}
      </button>

      {/* Delete on hover */}
      <button
        onClick={(e) => {
          e.stopPropagation()
          remove.mutate(project.id)
        }}
        className="absolute -top-1 -right-1 w-4 h-4 rounded-full bg-accent-red text-white items-center justify-center hidden group-hover:flex"
        title="Remove project"
      >
        <Trash2 size={9} />
      </button>
    </div>
  )
}

// ─── AddProjectModal ──────────────────────────────────────────────────────────

function AddProjectModal({ onClose }: { onClose: () => void }) {
  const [path, setPath] = useState('')
  const add = useAddProject()
  const { setSelectedId } = useProjectContext()

  function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!path.trim()) return
    add.mutate(path.trim(), {
      onSuccess: (proj) => {
        setSelectedId(proj.id)
        onClose()
      },
    })
  }

  return (
    <div
      className="fixed inset-0 bg-black/60 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <form
        onSubmit={submit}
        onClick={(e) => e.stopPropagation()}
        className="bg-surface-raised border border-surface-border rounded-xl p-6 w-96 shadow-2xl"
      >
        <h2 className="text-sm font-semibold text-gray-200 mb-4">Add AOM Project</h2>
        <input
          autoFocus
          type="text"
          placeholder="/path/to/your/project"
          value={path}
          onChange={(e) => setPath(e.target.value)}
          className="w-full bg-surface border border-surface-border rounded-lg px-3 py-2 text-sm text-gray-200 placeholder-gray-600 focus:outline-none focus:border-accent"
        />
        {add.error && (
          <p className="mt-2 text-xs text-accent-red">{(add.error as Error).message}</p>
        )}
        <div className="mt-4 flex gap-2 justify-end">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-xs text-gray-500 hover:text-gray-300 transition-colors"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={add.isPending}
            className="px-4 py-2 text-xs bg-accent text-white rounded-lg hover:bg-accent/90 disabled:opacity-50 transition-colors"
          >
            {add.isPending ? 'Adding…' : 'Add Project'}
          </button>
        </div>
      </form>
    </div>
  )
}
