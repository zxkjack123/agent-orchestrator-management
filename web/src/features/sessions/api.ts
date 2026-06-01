import { api } from '@/lib/api-client'
import type { Session } from './types'

export const sessionsApi = {
  list: (projectId: string, activeOnly = false) =>
    api.get<Session[]>(
      `/api/v1/projects/${projectId}/sessions${activeOnly ? '?active=true' : ''}`,
    ),
  stop: (projectId: string, sessionId: string) =>
    api.delete<void>(`/api/v1/projects/${projectId}/sessions/${sessionId}`),
}
