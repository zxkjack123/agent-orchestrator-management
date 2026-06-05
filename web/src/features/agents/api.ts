import { api } from '@/lib/api-client'
import type { Agent, AddAgentForm, UpdateAgentForm, SpawnSessionForm, SpawnSessionResult, SetInstructionsResponse } from './types'

export const agentsApi = {
  list: (projectId: string) =>
    api.get<Agent[]>(`/api/v1/projects/${projectId}/agents`),

  add: (projectId: string, data: AddAgentForm) =>
    api.post<Agent>(`/api/v1/projects/${projectId}/agents`, data),

  update: (projectId: string, name: string, data: UpdateAgentForm) =>
    api.put<Agent>(`/api/v1/projects/${projectId}/agents/${name}`, data),

  remove: (projectId: string, name: string) =>
    api.delete<void>(`/api/v1/projects/${projectId}/agents/${name}`),

  spawnSession: (projectId: string, data: SpawnSessionForm) =>
    api.post<SpawnSessionResult>(`/api/v1/projects/${projectId}/sessions`, data),

  provision: (projectId: string, name: string) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/agents/${name}/provision`,
      {},
    ),

  getProfile: (projectId: string, name: string) =>
    api.get<{ profile: string }>(
      `/api/v1/projects/${projectId}/agents/${name}/profile`,
    ),

  getInstructions: (projectId: string, name: string) =>
    api.get<{ instructions: string }>(
      `/api/v1/projects/${projectId}/agents/${name}/instructions`,
    ),

  setInstructions: (projectId: string, name: string, instructions: string) =>
    api.put<SetInstructionsResponse>(
      `/api/v1/projects/${projectId}/agents/${name}/instructions`,
      { instructions },
    ),
}
