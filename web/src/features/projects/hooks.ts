import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { projectsApi } from './api'

export const PROJECTS_KEY = ['projects'] as const

export function useProjects() {
  return useQuery({
    queryKey: PROJECTS_KEY,
    queryFn: projectsApi.list,
    refetchInterval: 10_000,
  })
}

export function useProjectAgents(projectId: string | null) {
  return useQuery({
    queryKey: ['projects', projectId, 'agents'],
    queryFn: () => projectsApi.agents(projectId!),
    enabled: !!projectId,
    refetchInterval: 5_000,
  })
}

export function useProjectStatus(projectId: string | null) {
  return useQuery({
    queryKey: ['projects', projectId, 'status'],
    queryFn: () => projectsApi.status(projectId!),
    enabled: !!projectId,
    refetchInterval: 5_000,
  })
}

export function useAddProject() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (path: string) => projectsApi.add(path),
    onSuccess: () => qc.invalidateQueries({ queryKey: PROJECTS_KEY }),
  })
}

export function useRemoveProject() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => projectsApi.remove(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: PROJECTS_KEY }),
  })
}
