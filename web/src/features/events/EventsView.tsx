import { useEffect, useRef, useState } from 'react'
import { useProjectContext } from '@/app/ProjectContext'
import { api } from '@/lib/api-client'
import { projectActionsApi } from '@/lib/project-actions-api'
import { openWS } from '@/lib/ws-client'

export function EventsView() {
  const { selectedId } = useProjectContext()
  const [lines, setLines] = useState<string[]>([])
  const [wsStatus, setWsStatus] = useState<'connecting' | 'connected' | 'disconnected'>('disconnected')
  const [message, setMessage] = useState('')
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState<string | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)

  // Load history on mount, then stream new events via WS
  useEffect(() => {
    if (!selectedId) return
    setLines([])
    setWsStatus('connecting')

    // Fetch existing channel history first
    api.get<{ lines: string[] }>(`/api/v1/projects/${selectedId}/channel`)
      .then((res) => setLines(res.lines ?? []))
      .catch(() => {})

    const cleanup = openWS(`/ws/events/${selectedId}`, {
      onOpen: () => setWsStatus('connected'),
      onClose: () => setWsStatus('disconnected'),
      onMessage: (msg) => {
        if (msg.type === 'event') {
          setLines((prev) => [...prev.slice(-500), msg.data])
        }
      },
    })
    return cleanup
  }, [selectedId])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines])

  async function handleSend(e: React.FormEvent) {
    e.preventDefault()
    if (!message.trim() || !selectedId) return
    setSending(true)
    setSendError(null)
    try {
      await projectActionsApi.channel(selectedId, message.trim())
      setMessage('')
    } catch (err) {
      setSendError(err instanceof Error ? err.message : 'Failed to send')
    } finally {
      setSending(false)
    }
  }

  if (!selectedId) {
    return <Empty message="Select a project." />
  }

  const wsColor = wsStatus === 'connected' ? 'bg-accent-green' : wsStatus === 'connecting' ? 'bg-accent-yellow' : 'bg-gray-600'

  return (
    <div className="h-full flex flex-col">
      {/* Event stream */}
      <div className="flex-1 overflow-auto p-4 font-mono text-xs">
        <div className="flex items-center gap-2 mb-3">
          <h2 className="text-sm font-semibold text-gray-300 font-sans">Channel (live)</h2>
          <span className={`w-2 h-2 rounded-full ${wsColor}`} title={wsStatus} />
          <span className="text-xs text-gray-600 capitalize">{wsStatus}</span>
        </div>
        {lines.length === 0 ? (
          <p className="text-gray-600">No channel messages yet.</p>
        ) : (
          lines.map((line, i) => (
            <div key={i} className="text-gray-400 py-0.5 border-b border-surface-border/30">
              {line}
            </div>
          ))
        )}
        <div ref={bottomRef} />
      </div>

      {/* Channel compose */}
      <div className="border-t border-surface-border bg-surface-raised p-3">
        <form onSubmit={handleSend} className="flex gap-2">
          <textarea
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                e.preventDefault()
                handleSend(e as unknown as React.FormEvent)
              }
            }}
            placeholder="Post to channel… (Ctrl+Enter to send)"
            rows={2}
            className="flex-1 bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent resize-none font-sans"
          />
          <button
            type="submit"
            disabled={sending || !message.trim()}
            className="px-4 py-2 text-xs bg-accent text-surface font-medium rounded hover:bg-accent/80 disabled:opacity-50 transition-colors self-end"
          >
            {sending ? 'Posting…' : 'Post'}
          </button>
        </form>
        {sendError && <p className="text-xs text-accent-red mt-1">{sendError}</p>}
      </div>
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
