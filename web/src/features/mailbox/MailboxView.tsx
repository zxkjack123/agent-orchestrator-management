import { useEffect, useRef, useState } from 'react'
import { useProjectContext } from '@/app/ProjectContext'
import { useAgents } from '@/features/agents/hooks'
import { api } from '@/lib/api-client'
import { projectActionsApi } from '@/lib/project-actions-api'
import { openWS } from '@/lib/ws-client'
import type { Session } from '@/features/sessions/types'

const OPERATOR_INBOX = '__operator__'

function Empty({ message }: { message: string }) {
  return (
    <div className="h-full flex items-center justify-center text-sm text-gray-600">
      {message}
    </div>
  )
}

export function MailboxView() {
  const { selectedId } = useProjectContext()
  const { data: agents = [], isLoading: agentsLoading } = useAgents(selectedId)

  const [selectedAgent, setSelectedAgent] = useState<string>('')
  const [lines, setLines] = useState<string[]>([])
  const [wsStatus, setWsStatus] = useState<'connecting' | 'connected' | 'disconnected'>('disconnected')
  const [message, setMessage] = useState('')
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState<string | null>(null)
  const [broadcastMsg, setBroadcastMsg] = useState('')
  const [broadcastPending, setBroadcastPending] = useState(false)
  const [broadcastError, setBroadcastError] = useState<string | null>(null)
  const [broadcastDone, setBroadcastDone] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)

  // Load mailbox history + connect WebSocket when agent changes
  useEffect(() => {
    if (!selectedId || !selectedAgent) {
      setLines([])
      setWsStatus('disconnected')
      return
    }
    setLines([])
    setWsStatus('connecting')

    // Resolve actual mailbox name (operator inbox = "operator")
    const mailboxName = selectedAgent === OPERATOR_INBOX ? 'operator' : selectedAgent

    // Fetch history first
    api.get<{ lines: string[] }>(`/api/v1/projects/${selectedId}/mailbox/${mailboxName}`)
      .then((res) => setLines(res.lines ?? []))
      .catch(() => {})

    const cleanup = openWS(`/ws/mailbox/${selectedId}/${mailboxName}`, {
      onOpen: () => setWsStatus('connected'),
      onClose: () => setWsStatus('disconnected'),
      onMessage: (msg) => {
        if (msg.type === 'output' || msg.type === 'event') {
          setLines((prev) => [...prev.slice(-1000), msg.data])
        }
      },
    })
    return cleanup
  }, [selectedId, selectedAgent])

  // Auto-scroll
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines])

  // Reset selection when project changes
  useEffect(() => {
    setSelectedAgent('')
    setLines([])
  }, [selectedId])

  async function handleSend(e: React.FormEvent) {
    e.preventDefault()
    if (!message.trim() || !selectedId || !selectedAgent) return

    setSending(true)
    setSendError(null)

    try {
      if (selectedAgent === OPERATOR_INBOX) {
        setSendError("Can't send from operator inbox — select an agent to message.")
        return
      }

      // Find the most recent session for this agent
      const sessions = await api.get<Session[]>(
        `/api/v1/projects/${selectedId}/sessions`,
      )
      const agentSessions = (sessions as Session[])
        .filter((s) => s.agent_name === selectedAgent)
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())

      if (agentSessions.length === 0) {
        setSendError('No session found for this agent. Start a session first.')
        return
      }

      const sid = agentSessions[0].id
      await api.post<void>(`/api/v1/projects/${selectedId}/sessions/${sid}/send`, {
        message: message.trim(),
      })
      setMessage('')
    } catch (err) {
      setSendError(err instanceof Error ? err.message : 'Failed to send message')
    } finally {
      setSending(false)
    }
  }

  async function handleBroadcast(e: React.FormEvent) {
    e.preventDefault()
    if (!broadcastMsg.trim() || !selectedId) return
    setBroadcastPending(true)
    setBroadcastError(null)
    try {
      await projectActionsApi.broadcast(selectedId, broadcastMsg.trim())
      setBroadcastMsg('')
      setBroadcastDone(true)
      setTimeout(() => setBroadcastDone(false), 3000)
    } catch (err) {
      setBroadcastError(err instanceof Error ? err.message : 'Failed to broadcast')
    } finally {
      setBroadcastPending(false)
    }
  }

  if (!selectedId) return <Empty message="Select a project." />
  if (agentsLoading) return <Empty message="Loading…" />

  const wsIndicator =
    wsStatus === 'connected'
      ? 'bg-accent-green'
      : wsStatus === 'connecting'
      ? 'bg-accent-yellow'
      : 'bg-gray-600'

  return (
    <div className="h-full flex flex-col">
      {/* Toolbar */}
      <div className="flex items-center gap-3 px-4 py-3 border-b border-surface-border bg-surface-raised flex-wrap">
        <h2 className="text-sm font-semibold text-gray-300">Mailbox</h2>
        <select
          value={selectedAgent}
          onChange={(e) => setSelectedAgent(e.target.value)}
          className="bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-1.5 focus:outline-none focus:border-accent"
        >
          <option value="">— Select —</option>
          <option value={OPERATOR_INBOX}>📥 My Inbox (replies from agents)</option>
          <optgroup label="Agent inboxes">
            {agents.map((a) => (
              <option key={a.name} value={a.name}>
                {a.name} ({a.role})
              </option>
            ))}
          </optgroup>
        </select>
        {selectedAgent && (
          <div className="ml-auto flex items-center gap-1.5">
            <span className={`w-2 h-2 rounded-full ${wsIndicator}`} />
            <span className="text-xs text-gray-600 capitalize">{wsStatus}</span>
          </div>
        )}
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-auto p-4 font-mono text-xs">
        {!selectedAgent ? (
          <p className="text-gray-600">Select an agent to view mailbox.</p>
        ) : lines.length === 0 ? (
          <p className="text-gray-600">Waiting for mailbox content…</p>
        ) : (
          lines.map((line, i) => (
            <div
              key={i}
              className="text-gray-400 py-0.5 border-b border-surface-border/20 whitespace-pre-wrap"
            >
              {line}
            </div>
          ))
        )}
        <div ref={bottomRef} />
      </div>

      {/* Send form */}
      {selectedAgent && (
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
              placeholder="Type a message… (Ctrl+Enter to send)"
              rows={2}
              className="flex-1 bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent resize-none"
            />
            <button
              type="submit"
              disabled={sending || !message.trim()}
              className="px-4 py-2 text-xs bg-accent text-surface font-medium rounded hover:bg-accent/80 disabled:opacity-50 transition-colors self-end"
            >
              {sending ? 'Sending…' : 'Send'}
            </button>
          </form>
          {sendError && (
            <p className="text-xs text-accent-red mt-1">{sendError}</p>
          )}
        </div>
      )}

      {/* Broadcast form */}
      <div className="border-t border-surface-border bg-surface p-3">
        <p className="text-xs text-gray-600 mb-2">Broadcast to all agents</p>
        <form onSubmit={handleBroadcast} className="flex gap-2">
          <textarea
            value={broadcastMsg}
            onChange={(e) => setBroadcastMsg(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                e.preventDefault()
                handleBroadcast(e as unknown as React.FormEvent)
              }
            }}
            placeholder="Broadcast message to all active sessions… (Ctrl+Enter)"
            rows={2}
            className="flex-1 bg-surface border border-surface-border text-gray-300 text-xs rounded px-3 py-2 focus:outline-none focus:border-accent-yellow resize-none"
          />
          <button
            type="submit"
            disabled={broadcastPending || !broadcastMsg.trim()}
            className="px-4 py-2 text-xs bg-accent-yellow/20 border border-accent-yellow/40 text-accent-yellow rounded hover:bg-accent-yellow/30 disabled:opacity-50 transition-colors self-end"
          >
            {broadcastPending ? 'Sending…' : 'Broadcast'}
          </button>
        </form>
        {broadcastDone && <p className="text-xs text-accent-green mt-1">Broadcast sent.</p>}
        {broadcastError && <p className="text-xs text-accent-red mt-1">{broadcastError}</p>}
      </div>
    </div>
  )
}
