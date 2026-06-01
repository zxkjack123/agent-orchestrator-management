import { api } from '@/lib/api-client'
import type { Agent, Project, ProjectStatus } from './types'

export const projectsApi = {
  list: () => api.get<Project[]>('/api/v1/projects'),
  add: (path: string) => api.post<Project>('/api/v1/projects', { path }),
  remove: (id: string) => api.delete<void>(`/api/v1/projects/${id}`),
  agents: (id: string) => api.get<Agent[]>(`/api/v1/projects/${id}/agents`),
  status: (id: string) => api.get<ProjectStatus>(`/api/v1/projects/${id}/status`),
}
