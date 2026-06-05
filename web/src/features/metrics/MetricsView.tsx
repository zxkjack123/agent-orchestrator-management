import { useState } from 'react'
import { useProjectContext } from '@/app/ProjectContext'
import { api } from '@/lib/api-client'

function Empty({ message }: { message: string }) {
  return (
    <div className="h-full flex items-center justify-center text-sm text-gray-600">
      {message}
    </div>
  )
}

export function MetricsView() {
  const { selectedId } = useProjectContext()
  const [output, setOutput] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [lastRun, setLastRun] = useState<string | null>(null)

  if (!selectedId) return <Empty message="Select a project." />

  async function fetchMetrics() {
    if (!selectedId) return
    setLoading(true)
    setError(null)
    try {
      const res = await api.get<{ status: string; output: string }>(
        `/api/v1/projects/${selectedId}/metrics`,
      )
      setOutput(res.output)
      setLastRun(new Date().toLocaleTimeString())
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="h-full overflow-auto p-4 space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold text-gray-300">Metrics</h2>
          {lastRun && <p className="text-xs text-gray-600 mt-0.5">Last run: {lastRun}</p>}
        </div>
        <button
          onClick={fetchMetrics}
          disabled={loading}
          className="px-3 py-1.5 text-xs bg-accent text-surface font-medium rounded hover:bg-accent/80 disabled:opacity-50 transition-colors"
        >
          {loading ? 'Running…' : 'Refresh'}
        </button>
      </div>

      {error && (
        <div className="bg-accent-red/10 border border-accent-red/20 rounded px-3 py-2 text-xs text-accent-red">
          {error}
        </div>
      )}

      {output == null ? (
        <div className="flex flex-col items-center justify-center py-16 gap-3">
          <p className="text-xs text-gray-600">Click Refresh to load velocity metrics.</p>
          <button
            onClick={fetchMetrics}
            disabled={loading}
            className="px-4 py-2 text-xs bg-surface-raised border border-surface-border text-gray-400 rounded hover:text-gray-300 disabled:opacity-50 transition-colors"
          >
            {loading ? 'Loading…' : 'Load Metrics'}
          </button>
        </div>
      ) : (
        <pre className="font-mono text-xs text-gray-300 bg-surface-raised border border-surface-border rounded-lg p-4 whitespace-pre-wrap">
          {output || '(no output)'}
        </pre>
      )}
    </div>
  )
}
