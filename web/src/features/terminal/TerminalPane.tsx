import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { Maximize2, Minimize2, Square } from 'lucide-react'
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
      fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
      fontSize: 12,
      lineHeight: 1.4,
      cursorBlink: true,
      scrollback: 1000,
    })

    const fitAddon = new FitAddon()
    fitAddonRef.current = fitAddon
    term.loadAddon(fitAddon)
    term.loadAddon(new WebLinksAddon())
    term.open(containerRef.current)
    fitAddon.fit()
    setTerminal(term)

    const observer = new ResizeObserver(() => fitAddon.fit())
    observer.observe(containerRef.current)

    return () => {
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
        'flex flex-col bg-surface rounded-lg border border-surface-border overflow-hidden transition-all',
        isFullscreen ? 'fixed inset-4 z-50 shadow-2xl' : '',
      ].join(' ')}
    >
      {/* Pane header */}
      <div className="flex items-center gap-2 px-3 py-1.5 bg-surface-raised border-b border-surface-border select-none">
        <span className={`w-2 h-2 rounded-full flex-shrink-0 ${statusColor}`} />
        <span className="text-xs text-gray-300 font-semibold flex-1 truncate">{agentName}</span>
        {status && (
          <span className="text-xs text-gray-600">{status}</span>
        )}

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

      {/* Terminal viewport */}
      <div
        ref={containerRef}
        className="flex-1 p-1 cursor-text min-h-0"
        onClick={() => terminal?.focus()}
      />
    </div>
  )
}
