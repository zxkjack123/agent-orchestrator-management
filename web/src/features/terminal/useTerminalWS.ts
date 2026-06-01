import { useEffect } from 'react'
import type { Terminal } from '@xterm/xterm'

const WS_BASE = import.meta.env.VITE_WS_BASE ?? ''

// useTerminalWS wires a WebSocket tmux stream to an xterm.js Terminal.
// Server sends full-screen snapshots; client sends keystrokes back.
export function useTerminalWS(paneId: string | undefined, terminal: Terminal | null) {
  useEffect(() => {
    if (!paneId || !terminal) return

    const term = terminal
    let stopped = false
    let ws: WebSocket | null = null

    function connect() {
      if (stopped) return
      const url = `${WS_BASE}/ws/terminal/${paneId}`.replace(/^http/, 'ws')
      ws = new WebSocket(url)

      ws.onmessage = (e) => {
        try {
          const msg = JSON.parse(e.data as string) as { type: string; data: string }
          if (msg.type === 'output') {
            // Full screen snapshot: reset then write so display matches tmux exactly.
            term.reset()
            term.write(msg.data)
          } else if (msg.type === 'error') {
            term.writeln(`\r\n\x1b[31m[aom] ${msg.data}\x1b[0m`)
          }
        } catch {
          // ignore malformed frames
        }
      }

      ws.onclose = () => {
        if (!stopped) {
          term.writeln('\r\n\x1b[33m[aom] disconnected — reconnecting…\x1b[0m')
          setTimeout(connect, 3000)
        }
      }

      ws.onerror = () => ws?.close()
    }

    connect()

    // Forward keystrokes to the tmux pane.
    const disposeKey = term.onData((data) => {
      if (ws?.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }))
      }
    })

    return () => {
      stopped = true
      ws?.close()
      disposeKey.dispose()
    }
  }, [paneId, terminal])
}
