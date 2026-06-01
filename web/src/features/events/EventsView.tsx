import { useEffect, useRef, useState } from 'react'
import { useProjectContext } from '@/app/ProjectContext'
import { openWS } from '@/lib/ws-client'

export function EventsView() {
  const { selectedId } = useProjectContext()
  const [lines, setLines] = useState<string[]>([])
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!selectedId) return
    setLines([])

    const cleanup = openWS(`/ws/events/${selectedId}`, {
      onMessage: (msg) => {
        if (msg.type === 'event') {
          setLines((prev) => [...prev.slice(-500), msg.data])
        }
      },
    })
    return cleanup
  }, [selectedId])

  // Auto-scroll to bottom on new lines.
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines])

  if (!selectedId) {
    return <Empty message="Select a project." />
  }

  return (
    <div className="h-full overflow-auto p-4 font-mono text-xs">
      <h2 className="text-sm font-semibold text-gray-300 mb-3">Events (live)</h2>
      {lines.length === 0 ? (
        <p className="text-gray-600">Waiting for events…</p>
      ) : (
        lines.map((line, i) => (
          <div key={i} className="text-gray-400 py-0.5 border-b border-surface-border/30">
            {line}
          </div>
        ))
      )}
      <div ref={bottomRef} />
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
