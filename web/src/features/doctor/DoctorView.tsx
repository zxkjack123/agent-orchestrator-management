import { useState } from 'react'
import { useProjectContext } from '@/app/ProjectContext'
import { api } from '@/lib/api-client'

type DoctorCheck = {
  result: 'pass' | 'warn' | 'fail' | 'info'
  message: string
}

type DoctorResult = {
  output: string
  checks: DoctorCheck[]
}

function Empty({ message }: { message: string }) {
  return (
    <div className="h-full flex items-center justify-center text-sm text-gray-600">
      {message}
    </div>
  )
}

function CheckRow({ check }: { check: DoctorCheck }) {
  const styles = {
    pass: { dot: 'bg-accent-green', text: 'text-gray-300', badge: 'text-accent-green' },
    warn: { dot: 'bg-accent-yellow', text: 'text-gray-300', badge: 'text-accent-yellow' },
    fail: { dot: 'bg-accent-red', text: 'text-gray-200', badge: 'text-accent-red' },
    info: { dot: 'bg-gray-500', text: 'text-gray-400', badge: 'text-gray-500' },
  }
  const s = styles[check.result] ?? styles.info
  return (
    <div className="flex items-start gap-3 px-3 py-2.5 border-b border-surface-border/30 last:border-0">
      <span className={`mt-1 w-2 h-2 rounded-full shrink-0 ${s.dot}`} />
      <span className={`text-xs ${s.text} flex-1`}>{check.message}</span>
      <span className={`text-xs font-mono uppercase shrink-0 ${s.badge}`}>{check.result}</span>
    </div>
  )
}

export function DoctorView() {
  const { selectedId } = useProjectContext()
  const [result, setResult] = useState<DoctorResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [lastRun, setLastRun] = useState<string | null>(null)

  if (!selectedId) return <Empty message="Select a project." />

  async function runDoctor() {
    if (!selectedId) return
    setLoading(true)
    try {
      const res = await api.post<DoctorResult>(
        `/api/v1/projects/${selectedId}/doctor`,
        {},
      )
      setResult(res)
      setLastRun(new Date().toLocaleTimeString())
    } catch (err) {
      setResult({ output: err instanceof Error ? err.message : 'Failed', checks: [] })
    } finally {
      setLoading(false)
    }
  }

  const passCount = result?.checks.filter((c) => c.result === 'pass').length ?? 0
  const warnCount = result?.checks.filter((c) => c.result === 'warn').length ?? 0
  const failCount = result?.checks.filter((c) => c.result === 'fail').length ?? 0

  return (
    <div className="h-full overflow-auto p-4 space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold text-gray-300">Health Check</h2>
          {lastRun && <p className="text-xs text-gray-600 mt-0.5">Last run: {lastRun}</p>}
        </div>
        <button
          onClick={runDoctor}
          disabled={loading}
          className="px-3 py-1.5 text-xs bg-accent text-surface font-medium rounded hover:bg-accent/80 disabled:opacity-50 transition-colors"
        >
          {loading ? 'Running…' : 'Run Doctor'}
        </button>
      </div>

      {result == null ? (
        <div className="flex flex-col items-center justify-center py-16 gap-3">
          <p className="text-xs text-gray-600">Run a health check to diagnose your project setup.</p>
          <button
            onClick={runDoctor}
            disabled={loading}
            className="px-4 py-2 text-xs bg-surface-raised border border-surface-border text-gray-400 rounded hover:text-gray-300 disabled:opacity-50 transition-colors"
          >
            {loading ? 'Running…' : 'Run Doctor'}
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          {result.checks.length > 0 && (
            <div className="flex items-center gap-4 text-xs">
              <span className="text-accent-green">{passCount} passed</span>
              {warnCount > 0 && <span className="text-accent-yellow">{warnCount} warnings</span>}
              {failCount > 0 && <span className="text-accent-red">{failCount} failed</span>}
            </div>
          )}

          {result.checks.length > 0 ? (
            <div className="bg-surface-raised border border-surface-border rounded-lg overflow-hidden">
              {result.checks.map((c, i) => (
                <CheckRow key={i} check={c} />
              ))}
            </div>
          ) : (
            <pre className="font-mono text-xs text-gray-300 bg-surface-raised border border-surface-border rounded-lg p-4 whitespace-pre-wrap">
              {result.output || '(no output)'}
            </pre>
          )}
        </div>
      )}
    </div>
  )
}
