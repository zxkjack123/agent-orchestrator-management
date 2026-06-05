import { api } from '@/lib/api-client'
import type { Session } from './types'

export const sessionsApi = {
  list: (projectId: string, activeOnly = false) =>
    api.get<Session[]>(
      `/api/v1/projects/${projectId}/sessions${activeOnly ? '?active=true' : ''}`,
    ),
  get: (projectId: string, sessionId: string) =>
    api.get<Session>(`/api/v1/projects/${projectId}/sessions/${sessionId}`),
  stop: (projectId: string, sessionId: string) =>
    api.delete<void>(`/api/v1/projects/${projectId}/sessions/${sessionId}`),
  archive: (projectId: string, sessionId: string) =>
    api.post<Session>(`/api/v1/projects/${projectId}/sessions/${sessionId}/archive`, {}),
  send: (projectId: string, sessionId: string, message: string, from?: string) =>
    api.post<void>(`/api/v1/projects/${projectId}/sessions/${sessionId}/send`, {
      message,
      ...(from ? { from } : {}),
    }),
  spawn: (projectId: string, agent: string, mode: 'real' | 'mock', taskId?: string, persistent = false) =>
    api.post<{ status: string; output: string }>(`/api/v1/projects/${projectId}/sessions`, {
      agent,
      mode,
      ...(taskId ? { task_id: taskId } : {}),
      ...(persistent ? { persistent: true } : {}),
    }),
  resume: (projectId: string, sessionId: string) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/sessions/${sessionId}/resume`,
      {},
    ),

  approve: (projectId: string, sessionId: string) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/sessions/${sessionId}/approve`,
      {},
    ),

  deny: (projectId: string, sessionId: string) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/sessions/${sessionId}/deny`,
      {},
    ),

  recover: (projectId: string, sessionId: string) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/sessions/${sessionId}/recover`,
      {},
    ),
}
