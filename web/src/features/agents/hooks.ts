import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { agentsApi } from './api'
import type { AddAgentForm, UpdateAgentForm, SpawnSessionForm } from './types'

export function useAgents(projectId: string | null) {
  const qc = useQueryClient()

  const query = useQuery({
    queryKey: ['projects', projectId, 'agents'],
    queryFn: () => agentsApi.list(projectId!),
    enabled: !!projectId,
    refetchInterval: 5_000,
  })

  const addMutation = useMutation({
    mutationFn: (data: AddAgentForm) => agentsApi.add(projectId!, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['projects', projectId, 'agents'] })
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ name, data }: { name: string; data: UpdateAgentForm }) =>
      agentsApi.update(projectId!, name, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['projects', projectId, 'agents'] })
    },
  })

  const removeMutation = useMutation({
    mutationFn: (name: string) => agentsApi.remove(projectId!, name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['projects', projectId, 'agents'] })
    },
  })

  const spawnMutation = useMutation({
    mutationFn: (data: SpawnSessionForm) => agentsApi.spawnSession(projectId!, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['projects', projectId, 'sessions'] })
    },
  })

  const provisionMutation = useMutation({
    mutationFn: (name: string) => agentsApi.provision(projectId!, name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['projects', projectId, 'agents'] })
    },
  })

  return {
    ...query,
    addAgent: addMutation.mutateAsync,
    updateAgent: (name: string, data: UpdateAgentForm) =>
      updateMutation.mutateAsync({ name, data }),
    removeAgent: removeMutation.mutateAsync,
    spawnSession: spawnMutation.mutateAsync,
    provisionAgent: provisionMutation.mutateAsync,
    isSpawning: spawnMutation.isPending,
    spawnResult: spawnMutation.data,
    spawnError: spawnMutation.error,
  }
}

export function useAgentProfile(projectId: string | null, agentName: string | null) {
  return useQuery({
    queryKey: ['projects', projectId, 'agents', agentName, 'profile'],
    queryFn: () => agentsApi.getProfile(projectId!, agentName!),
    enabled: !!projectId && !!agentName,
  })
}

export function useAgentInstructions(projectId: string | null, agentName: string | null) {
  const qc = useQueryClient()

  const query = useQuery({
    queryKey: ['projects', projectId, 'agents', agentName, 'instructions'],
    queryFn: () => agentsApi.getInstructions(projectId!, agentName!),
    enabled: !!projectId && !!agentName,
  })

  const setMutation = useMutation({
    mutationFn: (instructions: string) =>
      agentsApi.setInstructions(projectId!, agentName!, instructions),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: ['projects', projectId, 'agents', agentName, 'instructions'],
      })
    },
  })

  return {
    instructions: query.data?.instructions ?? '',
    isLoading: query.isLoading,
    error: query.error,
    setInstructions: setMutation.mutateAsync,
    isSaving: setMutation.isPending,
    saveError: setMutation.error,
    saveSuccess: setMutation.isSuccess,
    saveResult: setMutation.data,
  }
}
