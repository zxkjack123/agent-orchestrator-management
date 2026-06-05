import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { Maximize2, Minimize2, Square, ScrollText, X } from 'lucide-react'
import { useTerminalWS } from './useTerminalWS'

type Props = {
  agentName: string
  paneId: string
  status?: string
  onStop?: () => void
}

export function TerminalPane({ agentName, paneId, status, onStop }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [terminal, setTerminal] = useState<Terminal | null>(null)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [showHistory, setShowHistory] = useState(false)
  const fitAddonRef = useRef<FitAddon | null>(null)

  // Boot xterm.js once the container div mounts.
  useEffect(() => {
    if (!containerRef.current) return

    const term = new Terminal({
      theme: {
        background: '#0f1117',
        foreground: '#c9d1d9',
        cursor: '#58a6ff',
        selectionBackground: '#264f78',
        black: '#0d1117',
        brightBlack: '#6e7681',
        red: '#f85149',
        brightRed: '#ff7b72',
        green: '#3fb950',
        brightGreen: '#56d364',
        yellow: '#d29922',
        brightYellow: '#e3b341',
        blue: '#58a6ff',
        brightBlue: '#79c0ff',
        magenta: '#bc8cff',
        brightMagenta: '#d2a8ff',
        cyan: '#39c5cf',
        brightCyan: '#56d4dd',
        white: '#8b949e',
        brightWhite: '#ecf2f8',
      },
      // Use monospace as primary so xterm.js can measure charHeight immediately
      // without waiting for JetBrains Mono / Fira Code to load.
      fontFamily: "monospace, 'JetBrains Mono', 'Fira Code'",
      fontSize: 10,
      lineHeight: 1.3,
      cursorBlink: true,
      scrollback: 1000,
    })

    const fitAddon = new FitAddon()
    fitAddonRef.current = fitAddon
    term.loadAddon(fitAddon)
    term.loadAddon(new WebLinksAddon())
    term.open(containerRef.current)

    // Delay fit + terminal exposure until after CSS grid layout is complete.
    // If we call fitAddon.fit() synchronously in useEffect the grid row heights
    // may not have been calculated yet, causing xterm.js to size to 90-200 rows
    // instead of the correct ~20 rows. With the wrong row count, tmux gets
    // resized to match, and scrollToBottom() ends up showing the blank bottom.
    let initialized = false
    const container = containerRef.current
    const initTerminal = () => {
      if (initialized || container.clientHeight === 0) return
      initialized = true
      fitAddon.fit()
      setTerminal(term)
    }

    // ResizeObserver fires after CSS layout — use it for initial sizing too.
    const observer = new ResizeObserver(() => {
      if (!initialized) initTerminal()
      else fitAddon.fit()
    })
    observer.observe(container)

    // Fallback: double-RAF in case ResizeObserver doesn't fire (e.g., zero-size on mount).
    requestAnimationFrame(() => requestAnimationFrame(initTerminal))

    // Re-fit at multiple checkpoints to survive late CSS settlements and
    // async resource loading (fonts, images) that change char metrics.
    const t1 = setTimeout(() => fitAddon.fit(), 100)
    const t2 = setTimeout(() => fitAddon.fit(), 500)
    const t3 = setTimeout(() => fitAddon.fit(), 1500)

    return () => {
      clearTimeout(t1)
      clearTimeout(t2)
      clearTimeout(t3)
      observer.disconnect()
      term.dispose()
    }
  }, [])

  // Connect WebSocket once both terminal and paneId are ready.
  useTerminalWS(paneId, terminal)

  const statusColor = {
    Working: 'bg-accent-green',
    Idle: 'bg-accent-yellow',
    WaitingApproval: 'bg-accent-purple',
    WaitingHandoff: 'bg-accent-purple',
    Booting: 'bg-accent',
    Stopped: 'bg-gray-600',
  }[status ?? ''] ?? 'bg-gray-600'

  return (
    <div
      className={[
        'bg-surface rounded-lg border border-surface-border overflow-hidden',
        isFullscreen ? 'fixed inset-4 z-50 shadow-2xl' : '',
      ].join(' ')}
      style={{ display: 'grid', gridTemplateRows: 'auto 1fr' }}
    >
      {/* Pane header */}
      <div className="flex items-center gap-2 px-3 py-1.5 bg-surface-raised border-b border-surface-border select-none">
        <span className={`w-2 h-2 rounded-full flex-shrink-0 ${statusColor}`} />
        <span className="text-xs text-gray-300 font-semibold flex-1 truncate">{agentName}</span>
        {status && (
          <span className="text-xs text-gray-600">{status}</span>
        )}

        <button
          onClick={() => setShowHistory(true)}
          className="text-gray-600 hover:text-gray-300 transition-colors"
          title="View scrollback history"
        >
          <ScrollText size={12} />
        </button>

        <button
          onClick={() => setIsFullscreen((v) => !v)}
          className="text-gray-600 hover:text-gray-300 transition-colors"
          title={isFullscreen ? 'Exit fullscreen' : 'Fullscreen'}
        >
          {isFullscreen ? <Minimize2 size={12} /> : <Maximize2 size={12} />}
        </button>

        {onStop && (
          <button
            onClick={onStop}
            className="text-gray-600 hover:text-accent-red transition-colors"
            title="Stop session"
          >
            <Square size={12} />
          </button>
        )}
      </div>

      {/* Terminal viewport — 1fr grid row gives FitAddon a concrete computed height */}
      <div
        ref={containerRef}
        className="overflow-hidden cursor-text"
        onClick={() => terminal?.focus()}
      />

      {showHistory && (
        <HistoryModal paneId={paneId} agentName={agentName} onClose={() => setShowHistory(false)} />
      )}
    </div>
  )
}

// ─── HistoryModal ─────────────────────────────────────────────────────────────

function HistoryModal({ paneId, agentName, onClose }: { paneId: string; agentName: string; onClose: () => void }) {
  const [text, setText] = useState<string | null>(null)
  const [error, setError] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    fetch(`/api/v1/terminal/${encodeURIComponent(paneId)}/history`)
      .then((r) => { if (!r.ok) throw new Error(); return r.text() })
      .then((t) => { setText(t) })
      .catch(() => setError(true))
  }, [paneId])

  // Scroll to bottom once text loads so user sees latest output first.
  useEffect(() => {
    if (text !== null) bottomRef.current?.scrollIntoView()
  }, [text])

  return (
    <div
      className="fixed inset-0 bg-black/80 z-[100] flex flex-col p-4"
      onClick={onClose}
    >
      <div
        className="flex-1 flex flex-col rounded-xl border border-surface-border overflow-hidden shadow-2xl min-h-0"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center gap-2 px-4 py-2 bg-surface-raised border-b border-surface-border flex-none select-none">
          <ScrollText size={13} className="text-accent flex-none" />
          <span className="text-xs text-gray-300 font-semibold flex-1">
            {agentName} — scrollback history
          </span>
          {text === null && !error && (
            <span className="text-xs text-gray-500">Loading…</span>
          )}
          {error && (
            <span className="text-xs text-accent-red">Failed to load</span>
          )}
          <button onClick={onClose} className="text-gray-600 hover:text-gray-300 transition-colors">
            <X size={14} />
          </button>
        </div>

        {/* Scrollable plain-text history */}
        <div className="flex-1 overflow-y-auto bg-[#0f1117] min-h-0">
          <pre
            className="text-[11px] leading-[1.5] text-gray-200 whitespace-pre-wrap break-words px-4 py-3"
            style={{ fontFamily: "monospace, 'JetBrains Mono', 'Fira Code'" }}
          >
            {text ?? (error ? 'Could not load history.' : '')}
          </pre>
          <div ref={bottomRef} />
        </div>
      </div>
    </div>
  )
}
