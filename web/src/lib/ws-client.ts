const WS_BASE = import.meta.env.VITE_WS_BASE ?? ''

export type WSMessage = {
  type: 'output' | 'event' | 'error'
  data: string
}

export type WSOptions = {
  onMessage: (msg: WSMessage) => void
  onOpen?: () => void
  onClose?: () => void
  // How long to wait before reconnecting after an unexpected close (ms).
  reconnectDelay?: number
}

// openWS opens a WebSocket connection and returns a cleanup function.
// It auto-reconnects unless cleanup() is called first.
export function openWS(path: string, opts: WSOptions): () => void {
  let ws: WebSocket | null = null
  let stopped = false
  const delay = opts.reconnectDelay ?? 3000

  function connect() {
    if (stopped) return
    const url = `${WS_BASE}${path}`.replace(/^http/, 'ws')
    ws = new WebSocket(url)

    ws.onopen = () => opts.onOpen?.()
    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data as string) as WSMessage
        opts.onMessage(msg)
      } catch {
        // ignore malformed frames
      }
    }
    ws.onclose = () => {
      opts.onClose?.()
      if (!stopped) setTimeout(connect, delay)
    }
    ws.onerror = () => ws?.close()
  }

  connect()

  return () => {
    stopped = true
    ws?.close()
  }
}

// sendInput sends a keyboard input message to an open terminal WebSocket.
export function sendInput(ws: WebSocket | null, data: string) {
  if (ws?.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: 'input', data }))
  }
}
