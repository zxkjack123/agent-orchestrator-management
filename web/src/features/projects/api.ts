import { api } from '@/lib/api-client'
import type { Agent, Project, ProjectStatus } from './types'

export const projectsApi = {
  list: () => api.get<Project[]>('/api/v1/projects'),
  add: (path: string) => api.post<Project>('/api/v1/projects', { path }),
  remove: (id: string) => api.delete<void>(`/api/v1/projects/${id}`),
  agents: (id: string) => api.get<Agent[]>(`/api/v1/projects/${id}/agents`),
  status: (id: string) => api.get<ProjectStatus>(`/api/v1/projects/${id}/status`),
  isolateSession: (id: string, agent: string, mode: string) =>
    api.post<{ status: string; output: string }>(`/api/v1/projects/${id}/sessions/isolate`, { agent, mode }),
}

export interface FsBrowseResult {
  path: string
  parent: string
  entries: { name: string; path: string }[]
}

export const fsApi = {
  browse: (path?: string): Promise<FsBrowseResult> => {
    const url = path ? `/api/v1/fs/browse?path=${encodeURIComponent(path)}` : '/api/v1/fs/browse'
    return api.get<FsBrowseResult>(url)
  },
  mkdir: (parent: string, name: string): Promise<{ path: string }> =>
    api.post('/api/v1/fs/mkdir', { parent, name }),
}

export const projectInitApi = {
  init: (path: string): Promise<{ status: string; output: string }> =>
    api.post('/api/v1/projects/init', { path }),
}
