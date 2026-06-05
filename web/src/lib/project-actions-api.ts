import { api } from './api-client'

export const projectActionsApi = {
  channel: (projectId: string, message: string) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/channel`,
      { message },
    ),

  broadcast: (projectId: string, message: string) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/broadcast`,
      { message },
    ),

  pauseAll: (projectId: string) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/pause-all`,
      {},
    ),

  resumeAll: (projectId: string) =>
    api.post<{ status: string; output: string }>(
      `/api/v1/projects/${projectId}/resume-all`,
      {},
    ),
}
