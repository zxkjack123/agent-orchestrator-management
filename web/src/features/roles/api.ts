import { api } from '@/lib/api-client'
import type { RoleDefinition, ClassDetail, ClassInfo, CreateRoleForm } from './types'

export const rolesApi = {
  listRoles: (projectId: string) =>
    api.get<RoleDefinition[]>(`/api/v1/projects/${projectId}/roles`),

  createRole: (projectId: string, data: CreateRoleForm) =>
    api.post<RoleDefinition>(`/api/v1/projects/${projectId}/roles`, data),

  getRole: (projectId: string, name: string) =>
    api.get<RoleDefinition>(`/api/v1/projects/${projectId}/roles/${name}`),

  updateRole: (
    projectId: string,
    name: string,
    data: Partial<Omit<RoleDefinition, 'name' | 'agents_using'>>,
  ) => api.put<RoleDefinition>(`/api/v1/projects/${projectId}/roles/${name}`, data),

  deleteRole: (projectId: string, name: string) =>
    api.delete<void>(`/api/v1/projects/${projectId}/roles/${name}`),

  previewRole: (projectId: string, name: string, runtime = 'claude') =>
    api.get<{ rendered: string }>(
      `/api/v1/projects/${projectId}/roles/${name}/preview?runtime=${runtime}`,
    ),

  listClasses: (projectId: string) =>
    api.get<ClassInfo[]>(`/api/v1/projects/${projectId}/classes`),

  getClass: (projectId: string, name: string) =>
    api.get<ClassDetail>(`/api/v1/projects/${projectId}/classes/${name}`),

  setClass: (projectId: string, name: string, content: string) =>
    api.put<ClassDetail>(`/api/v1/projects/${projectId}/classes/${name}`, { content }),

  deleteClass: (projectId: string, name: string) =>
    api.delete<void>(`/api/v1/projects/${projectId}/classes/${name}`),

  previewClass: (
    projectId: string,
    name: string,
    opts: { runtime?: string; role?: string; agent?: string } = {},
  ) => {
    const params = new URLSearchParams()
    if (opts.runtime) params.set('runtime', opts.runtime)
    if (opts.role) params.set('role', opts.role)
    if (opts.agent) params.set('agent', opts.agent)
    return api.get<{ rendered: string }>(
      `/api/v1/projects/${projectId}/classes/${name}/preview?${params}`,
    )
  },

  getSystemTemplate: () => api.get<{ content: string }>('/api/v1/system-template'),
}
