import { useState, useEffect, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Trash2, Folder, FolderOpen, ChevronRight, ArrowLeft, FolderPlus } from 'lucide-react'
import { useAddProject, useProjects, useRemoveProject } from './hooks'
import { useProjectContext } from '@/app/ProjectContext'
import { fsApi, projectInitApi, type FsBrowseResult } from './api'
import type { Project } from './types'

export function Sidebar() {
  const { data: projects = [] } = useProjects()
  const { selectedId, setSelectedId } = useProjectContext()
  const navigate = useNavigate()
  const [showAdd, setShowAdd] = useState(false)

  function selectProject(id: string) {
    setSelectedId(id)
    navigate(`/projects/${id}/war-room`)
  }

  return (
    <aside className="flex flex-col w-14 bg-surface-raised border-r border-surface-border h-screen">
      {/* Project list */}
      <div className="flex-1 flex flex-col items-center py-3 gap-2 overflow-y-auto">
        {projects.map((p) => (
          <ProjectIcon
            key={p.id}
            project={p}
            selected={p.id === selectedId}
            onSelect={() => selectProject(p.id)}
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

// ─── DirBrowser ───────────────────────────────────────────────────────────────

function DirBrowser({ onSelect, onClose }: { onSelect: (path: string) => void; onClose: () => void }) {
  const [result, setResult] = useState<FsBrowseResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [creatingFolder, setCreatingFolder] = useState(false)
  const [newFolderName, setNewFolderName] = useState('')
  const [mkdirError, setMkdirError] = useState('')
  const newFolderInputRef = useRef<HTMLInputElement>(null)

  const navigate = useCallback((path?: string) => {
    setLoading(true)
    setError('')
    fsApi.browse(path)
      .then(setResult)
      .catch(() => setError('Cannot read directory'))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { navigate() }, [navigate])

  useEffect(() => {
    if (creatingFolder) newFolderInputRef.current?.focus()
  }, [creatingFolder])

  function startNewFolder() {
    setCreatingFolder(true)
    setNewFolderName('')
    setMkdirError('')
  }

  function cancelNewFolder() {
    setCreatingFolder(false)
    setNewFolderName('')
    setMkdirError('')
  }

  function confirmNewFolder() {
    if (!result?.path || !newFolderName.trim()) return
    fsApi.mkdir(result.path, newFolderName.trim())
      .then((res) => {
        setCreatingFolder(false)
        setNewFolderName('')
        navigate(res.path) // navigate into new folder
      })
      .catch((e) => setMkdirError(e.message ?? 'Cannot create folder'))
  }

  return (
    <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-[60]" onClick={onClose}>
      <div
        className="bg-surface-raised border border-surface-border rounded-xl shadow-2xl w-[480px] max-h-[70vh] flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center gap-2 px-4 py-3 border-b border-surface-border flex-none">
          <FolderOpen size={15} className="text-accent flex-none" />
          <span className="text-xs text-gray-300 font-mono truncate flex-1 min-w-0">
            {result?.path ?? '…'}
          </span>
          <button
            onClick={() => result?.parent && navigate(result.parent)}
            disabled={!result?.parent}
            className="flex items-center gap-1 text-xs text-gray-400 hover:text-gray-200 disabled:opacity-30 transition-colors"
          >
            <ArrowLeft size={13} /> Up
          </button>
        </div>

        {/* New folder input (inline) */}
        {creatingFolder && (
          <div className="px-4 py-2 border-b border-surface-border flex-none bg-surface/50">
            <div className="flex items-center gap-2">
              <FolderPlus size={13} className="text-yellow-400 flex-none" />
              <input
                ref={newFolderInputRef}
                type="text"
                placeholder="New folder name"
                value={newFolderName}
                onChange={(e) => { setNewFolderName(e.target.value); setMkdirError('') }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') confirmNewFolder()
                  if (e.key === 'Escape') cancelNewFolder()
                }}
                className="flex-1 bg-surface border border-surface-border rounded px-2 py-1 text-xs text-gray-200 placeholder-gray-600 focus:outline-none focus:border-accent"
              />
              <button
                onClick={confirmNewFolder}
                disabled={!newFolderName.trim()}
                className="px-2 py-1 text-xs bg-yellow-500/20 text-yellow-300 rounded hover:bg-yellow-500/30 disabled:opacity-40 transition-colors"
              >
                Create
              </button>
              <button onClick={cancelNewFolder} className="text-xs text-gray-500 hover:text-gray-300 transition-colors">
                ✕
              </button>
            </div>
            {mkdirError && <p className="mt-1 text-xs text-accent-red">{mkdirError}</p>}
          </div>
        )}

        {/* Entry list */}
        <div className="flex-1 overflow-y-auto py-1">
          {loading && (
            <p className="text-xs text-gray-500 px-4 py-3">Loading…</p>
          )}
          {error && (
            <p className="text-xs text-accent-red px-4 py-3">{error}</p>
          )}
          {!loading && result && result.entries.length === 0 && !creatingFolder && (
            <p className="text-xs text-gray-600 px-4 py-3">No subdirectories</p>
          )}
          {!loading && result?.entries.map((e) => (
            <button
              key={e.path}
              onClick={() => navigate(e.path)}
              className="w-full flex items-center gap-2 px-4 py-2 text-sm text-gray-300 hover:bg-surface transition-colors text-left"
            >
              <Folder size={14} className="text-yellow-500/70 flex-none" />
              <span className="flex-1 truncate">{e.name}</span>
              <ChevronRight size={13} className="text-gray-600 flex-none" />
            </button>
          ))}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between px-4 py-3 border-t border-surface-border flex-none gap-3">
          <button
            onClick={startNewFolder}
            disabled={!result?.path}
            className="flex items-center gap-1.5 text-xs text-yellow-400 hover:text-yellow-300 disabled:opacity-30 transition-colors"
          >
            <FolderPlus size={13} /> New Folder
          </button>
          <div className="flex gap-2 flex-none">
            <button
              onClick={onClose}
              className="px-3 py-1.5 text-xs text-gray-500 hover:text-gray-300 transition-colors"
            >
              Cancel
            </button>
            <button
              disabled={!result?.path}
              onClick={() => result?.path && onSelect(result.path)}
              className="px-3 py-1.5 text-xs bg-accent text-white rounded-lg hover:bg-accent/90 disabled:opacity-50 transition-colors"
            >
              Select this folder
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

// ─── AddProjectModal ──────────────────────────────────────────────────────────

function AddProjectModal({ onClose }: { onClose: () => void }) {
  const [path, setPath] = useState('')
  const [showBrowser, setShowBrowser] = useState(false)
  const [initStatus, setInitStatus] = useState<'idle' | 'running' | 'done' | 'error'>('idle')
  const [initOutput, setInitOutput] = useState('')
  const add = useAddProject()
  const { setSelectedId } = useProjectContext()
  const navigate = useNavigate()

  function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!path.trim()) return
    addProject(path.trim())
  }

  function addProject(p: string) {
    add.mutate(p, {
      onSuccess: (proj) => {
        setSelectedId(proj.id)
        navigate(`/projects/${proj.id}/war-room`)
        onClose()
      },
    })
  }

  function runInit() {
    if (!path.trim()) return
    setInitStatus('running')
    setInitOutput('')
    projectInitApi.init(path.trim())
      .then((res) => {
        setInitOutput(res.output)
        if (res.status === 'ok') {
          setInitStatus('done')
          // Auto-add project after successful init
          addProject(path.trim())
        } else {
          setInitStatus('error')
        }
      })
      .catch((e) => {
        setInitOutput(e.message ?? 'Init failed')
        setInitStatus('error')
      })
  }

  return (
    <>
      <div
        className="fixed inset-0 bg-black/60 flex items-center justify-center z-50"
        onClick={onClose}
      >
        <form
          onSubmit={submit}
          onClick={(e) => e.stopPropagation()}
          className="bg-surface-raised border border-surface-border rounded-xl p-6 w-[440px] shadow-2xl"
        >
          <h2 className="text-sm font-semibold text-gray-200 mb-4">Add AOM Project</h2>
          <div className="flex gap-2">
            <input
              autoFocus
              type="text"
              placeholder="/path/to/your/project"
              value={path}
              onChange={(e) => { setPath(e.target.value); setInitStatus('idle'); setInitOutput('') }}
              className="flex-1 min-w-0 bg-surface border border-surface-border rounded-lg px-3 py-2 text-sm text-gray-200 placeholder-gray-600 focus:outline-none focus:border-accent"
            />
            <button
              type="button"
              onClick={() => setShowBrowser(true)}
              className="flex-none px-2.5 py-2 bg-surface border border-surface-border rounded-lg text-gray-400 hover:text-gray-200 hover:border-accent transition-colors"
              title="Browse…"
            >
              <Folder size={15} />
            </button>
          </div>

          {/* Init output */}
          {initOutput && (
            <pre className={[
              'mt-3 p-3 rounded-lg text-xs font-mono whitespace-pre-wrap max-h-40 overflow-y-auto',
              initStatus === 'error' ? 'bg-red-900/20 text-red-300 border border-red-800/40' : 'bg-surface text-gray-400 border border-surface-border',
            ].join(' ')}>
              {initOutput}
            </pre>
          )}

          {add.error && (
            <p className="mt-2 text-xs text-accent-red">{(add.error as Error).message}</p>
          )}

          <div className="mt-4 flex gap-2 justify-between items-center">
            {/* Init AOM button */}
            <button
              type="button"
              disabled={!path.trim() || initStatus === 'running'}
              onClick={runInit}
              className="px-3 py-2 text-xs bg-yellow-500/15 text-yellow-300 border border-yellow-600/30 rounded-lg hover:bg-yellow-500/25 disabled:opacity-40 transition-colors"
              title="Run aom project init in this directory"
            >
              {initStatus === 'running' ? 'Initializing…' : 'Init AOM'}
            </button>

            <div className="flex gap-2">
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 text-xs text-gray-500 hover:text-gray-300 transition-colors"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={add.isPending || !path.trim()}
                className="px-4 py-2 text-xs bg-accent text-white rounded-lg hover:bg-accent/90 disabled:opacity-50 transition-colors"
              >
                {add.isPending ? 'Adding…' : 'Add Project'}
              </button>
            </div>
          </div>
        </form>
      </div>

      {showBrowser && (
        <DirBrowser
          onSelect={(p) => { setPath(p); setShowBrowser(false); setInitStatus('idle'); setInitOutput('') }}
          onClose={() => setShowBrowser(false)}
        />
      )}
    </>
  )
}
