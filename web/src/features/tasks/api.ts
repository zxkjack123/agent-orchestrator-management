import { api } from '@/lib/api-client'
import type { Task } from './types'

export type TaskSignal = 'task.completed' | 'handoff.prepared' | 'checkpoint.created' | 'step.completed'

export const tasksApi = {
  list: (projectId: string) =>
    api.get<Task[]>(`/api/v1/projects/${projectId}/tasks`),

  get: (projectId: string, taskId: string) =>
    api.get<Task>(`/api/v1/projects/${projectId}/tasks/${taskId}`),

  create: (
    projectId: string,
    title: string,
    opts?: { description?: string; mode?: string; agent?: string; role?: string },
  ) =>
    api.post<{ status: string; output: string }>(`/api/v1/projects/${projectId}/tasks`, {
      title,
      description: opts?.description ?? '',
      mode: opts?.mode ?? '',
      agent: opts?.agent ?? '',
      role: opts?.role ?? '',
    }),

  signal: (projectId: string, taskId: string, signal: TaskSignal) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/tasks/${taskId}/signal`,
      { signal },
    ),

  accept: (projectId: string, taskId: string, force = false) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/tasks/${taskId}/accept`,
      { force },
    ),

  close: (projectId: string, taskId: string) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/tasks/${taskId}/close`,
      {},
    ),

  cancel: (projectId: string, taskId: string) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/tasks/${taskId}/cancel`,
      {},
    ),
}
