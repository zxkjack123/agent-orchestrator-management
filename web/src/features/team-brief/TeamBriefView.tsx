import { useEffect, useState } from 'react'
import { useProjectContext } from '@/app/ProjectContext'
import { api } from '@/lib/api-client'

export function TeamBriefView() {
  const { selectedId } = useProjectContext()
  const [content, setContent] = useState('')
  const [draft, setDraft] = useState('')
  const [editing, setEditing] = useState(false)
  const [loading, setLoading] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [saving, setSaving] = useState(false)
  const [pushing, setPushing] = useState(false)
  const [lastPushed, setLastPushed] = useState<Date | null>(null)
  const [feedback, setFeedback] = useState<{ msg: string; ok: boolean } | null>(null)

  useEffect(() => {
    if (!selectedId) return
    setLoading(true)
    api
      .get<{ content: string }>(`/api/v1/projects/${selectedId}/team-brief`)
      .then((res) => setContent(res.content ?? ''))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [selectedId])

  function flash(msg: string, ok = true) {
    setFeedback({ msg, ok })
    setTimeout(() => setFeedback(null), 5000)
  }

  async function handleGenerate() {
    if (!selectedId) return
    setGenerating(true)
    try {
      const res = await api.post<{ content: string }>(
        `/api/v1/projects/${selectedId}/team-brief/generate`,
        {}
      )
      setContent(res.content ?? '')
      setEditing(false)
      flash('Generated from project state')
    } catch (err) {
      flash(err instanceof Error ? err.message : 'Generate failed', false)
    } finally {
      setGenerating(false)
    }
  }

  async function handleSave() {
    if (!selectedId) return
    setSaving(true)
    try {
      await api.put(`/api/v1/projects/${selectedId}/team-brief`, { content: draft })
      setContent(draft)
      setEditing(false)
      flash('Saved')
    } catch {
      flash('Save failed', false)
    } finally {
      setSaving(false)
    }
  }

  async function handlePush() {
    if (!selectedId) return
    setPushing(true)
    try {
      await api.post(`/api/v1/projects/${selectedId}/team-brief/push`, {})
      setLastPushed(new Date())
      flash('Pushed to team channel and agent worktrees')
    } catch (err) {
      flash(err instanceof Error ? err.message : 'Push failed', false)
    } finally {
      setPushing(false)
    }
  }

  if (!selectedId) {
    return (
      <div className="h-full flex items-center justify-center text-sm text-gray-600">
        Select a project.
      </div>
    )
  }

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center text-sm text-gray-600">
        Loading…
      </div>
    )
  }

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-surface-border bg-surface-raised shrink-0">
        <div>
          <h2 className="text-sm font-semibold text-gray-300">Team Brief</h2>
          <p className="text-[11px] text-gray-600 mt-0.5">
            Shared project context — every agent reads this at session start
          </p>
        </div>
        <div className="flex items-center gap-2">
          {feedback && (
            <span className={`text-xs ${feedback.ok ? 'text-accent-green' : 'text-accent-red'}`}>
              {feedback.msg}
            </span>
          )}
          {editing ? (
            <>
              <button
                onClick={() => setEditing(false)}
                className="px-3 py-1.5 text-xs bg-surface border border-surface-border text-gray-400 rounded hover:text-gray-300 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                disabled={saving}
                className="px-3 py-1.5 text-xs bg-accent text-surface font-medium rounded hover:bg-accent/80 disabled:opacity-50 transition-colors"
              >
                {saving ? 'Saving…' : 'Save'}
              </button>
            </>
          ) : (
            <>
              <button
                onClick={handleGenerate}
                disabled={generating}
                className="px-3 py-1.5 text-xs bg-surface border border-surface-border text-gray-400 rounded hover:text-gray-300 disabled:opacity-50 transition-colors"
                title="Auto-generate from current tasks, sessions, and agents"
              >
                {generating ? 'Generating…' : '⟳ Generate'}
              </button>
              {content && (
                <>
                  <button
                    onClick={() => { setDraft(content); setEditing(true) }}
                    className="px-3 py-1.5 text-xs bg-surface border border-surface-border text-gray-400 rounded hover:text-gray-300 transition-colors"
                  >
                    Edit
                  </button>
                  <button
                    onClick={handlePush}
                    disabled={pushing}
                    className="px-3 py-1.5 text-xs bg-accent text-surface font-medium rounded hover:bg-accent/80 disabled:opacity-50 transition-colors"
                    title="Broadcast to team channel and copy to all active agent worktrees"
                  >
                    {pushing ? 'Pushing…' : '↑ Push to Team'}
                  </button>
                </>
              )}
            </>
          )}
        </div>
      </div>

      {/* Body */}
      <div className="flex-1 min-h-0 overflow-auto p-4">
        {editing ? (
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            className="w-full h-full min-h-[400px] bg-surface border border-surface-border text-gray-300 text-xs font-mono rounded px-3 py-2 focus:outline-none focus:border-accent resize-none"
            placeholder="Write the team brief in Markdown…"
            autoFocus
          />
        ) : content ? (
          <pre className="font-mono text-xs text-gray-300 whitespace-pre-wrap leading-relaxed">
            {content}
          </pre>
        ) : (
          <div className="flex flex-col items-center justify-center py-20 gap-4">
            <p className="text-sm text-gray-500">No team brief yet</p>
            <p className="text-xs text-gray-600 text-center max-w-sm">
              Generate one from current project state, or write it manually.
              Every agent will read it automatically at session start.
            </p>
            <div className="flex gap-2">
              <button
                onClick={handleGenerate}
                disabled={generating}
                className="px-4 py-2 text-xs bg-accent text-surface font-medium rounded hover:bg-accent/80 disabled:opacity-50 transition-colors"
              >
                {generating ? 'Generating…' : '⟳ Generate from Project State'}
              </button>
              <button
                onClick={() => { setDraft(''); setEditing(true) }}
                className="px-4 py-2 text-xs bg-surface border border-surface-border text-gray-400 rounded hover:text-gray-300 transition-colors"
              >
                Write Manually
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Footer hint */}
      {content && !editing && (
        <div className="px-4 py-2 border-t border-surface-border bg-surface-raised shrink-0 flex items-center justify-between">
          <p className="text-[11px] text-gray-600">
            Generate updates from live project data · Push broadcasts to team channel + agent worktrees
          </p>
          {lastPushed && (
            <span className="text-[11px] text-accent-green/70 shrink-0 ml-4">
              ✓ pushed {lastPushed.toLocaleTimeString()}
            </span>
          )}
        </div>
      )}
    </div>
  )
}
