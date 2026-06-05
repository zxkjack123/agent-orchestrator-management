import { useEffect } from 'react'
import type { Terminal } from '@xterm/xterm'

const WS_BASE = import.meta.env.VITE_WS_BASE ?? ''

// useTerminalWS connects an xterm.js Terminal to a tmux pane via PTY-backed WebSocket.
//
// Protocol:
//   Server → Browser (text JSON):  { type: "ready" }  — send dimensions now
//                                   { type: "error", data: "..." }  — pane not found / fatal
//   Server → Browser (binary):     Raw PTY bytes — written directly to xterm.js
//   Browser → Server (binary):     Raw keystroke bytes from xterm.js onData
//   Browser → Server (text JSON):  { type: "resize", cols, rows }  — terminal resize
export function useTerminalWS(paneId: string | undefined, terminal: Terminal | null) {
  useEffect(() => {
    if (!paneId || !terminal) return

    const term = terminal
    let stopped = false
    let ws: WebSocket | null = null
    const encoder = new TextEncoder()

    function connect() {
      if (stopped) return
      const url = `${WS_BASE}/ws/terminal/${encodeURIComponent(paneId!)}`.replace(/^http/, 'ws')
      ws = new WebSocket(url)
      ws.binaryType = 'arraybuffer' // receive binary frames as ArrayBuffer

      ws.onmessage = (e) => {
        if (e.data instanceof ArrayBuffer) {
          // Raw PTY output — pass straight to xterm.js.
          // tmux rendered this at the browser's exact dimensions, so no wrapping issues.
          term.write(new Uint8Array(e.data))
        } else {
          // JSON control message
          try {
            const msg = JSON.parse(e.data as string) as { type: string; data?: string }
            if (msg.type === 'ready') {
              // Server is ready — send our FitAddon-sized dimensions.
              if (ws?.readyState === WebSocket.OPEN && term.cols > 1 && term.rows > 1) {
                ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
              }
            } else if (msg.type === 'error') {
              term.writeln(`\r\n\x1b[31m[aom] ${msg.data}\x1b[0m`)
              if (msg.data?.includes('pane not found')) {
                // Pane is gone — no point retrying until the agent is respawned.
                stopped = true
                term.writeln(`\r\x1b[33m[aom] start a session: aom session spawn <agent>\x1b[0m`)
              }
              // Other errors (e.g. transient tmux failures) → close and let onclose retry.
              ws?.close()
            }
          } catch {
            // ignore malformed frames
          }
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

    // Forward keystrokes as raw binary bytes — no JSON wrapping overhead.
    const disposeKey = term.onData((data) => {
      if (ws?.readyState === WebSocket.OPEN) {
        ws.send(encoder.encode(data).buffer)
      }
    })

    // Forward terminal resize events so the PTY is resized and tmux redraws.
    const disposeResize = term.onResize(({ cols, rows }) => {
      if (ws?.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols, rows }))
      }
    })

    return () => {
      stopped = true
      ws?.close()
      disposeKey.dispose()
      disposeResize.dispose()
    }
  }, [paneId, terminal])
}
