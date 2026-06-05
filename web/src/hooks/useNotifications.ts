import { useEffect, useRef } from 'react'

type WatchItem = {
  id: string
  label: string
  status: string
}

// Triggers a browser notification when new WaitingApproval sessions or
// NeedsAttention/WaitingHandoff tasks appear that weren't seen before.
export function useNotifications(
  sessions: WatchItem[],
  tasks: WatchItem[],
  projectName?: string,
) {
  const knownIds = useRef<Set<string>>(new Set())
  const permissionRequested = useRef(false)

  useEffect(() => {
    if (typeof Notification === 'undefined') return
    if (!permissionRequested.current && Notification.permission === 'default') {
      permissionRequested.current = true
      Notification.requestPermission()
    }
  }, [])

  useEffect(() => {
    if (typeof Notification === 'undefined') return
    if (Notification.permission !== 'granted') return

    const alertSessions = sessions.filter((s) => s.status === 'WaitingApproval')
    const alertTasks = tasks.filter((t) =>
      ['NeedsAttention', 'WaitingHandoff'].includes(t.status),
    )

    for (const s of alertSessions) {
      if (!knownIds.current.has(s.id)) {
        knownIds.current.add(s.id)
        new Notification(`[${projectName ?? 'AOM'}] Approval needed`, {
          body: `${s.label} is waiting for your approval`,
          tag: s.id,
        })
      }
    }

    for (const t of alertTasks) {
      if (!knownIds.current.has(t.id)) {
        knownIds.current.add(t.id)
        new Notification(`[${projectName ?? 'AOM'}] Task needs attention`, {
          body: `${t.label}: ${t.status}`,
          tag: t.id,
        })
      }
    }
  }, [sessions, tasks, projectName])
}
