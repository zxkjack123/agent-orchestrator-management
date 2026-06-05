import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { sessionsApi } from './api'

export function useSessions(projectId: string | null, activeOnly = false) {
  const qc = useQueryClient()
  const invalidate = () => qc.invalidateQueries({ queryKey: ['projects', projectId] })

  const query = useQuery({
    queryKey: ['projects', projectId, 'sessions', activeOnly],
    queryFn: () => sessionsApi.list(projectId!, activeOnly),
    enabled: !!projectId,
    refetchInterval: 5_000,
  })

  const stopMutation = useMutation({
    mutationFn: ({ sessionId }: { sessionId: string }) =>
      sessionsApi.stop(projectId!, sessionId),
    onSuccess: invalidate,
  })

  const archiveMutation = useMutation({
    mutationFn: ({ sessionId }: { sessionId: string }) =>
      sessionsApi.archive(projectId!, sessionId),
    onSuccess: invalidate,
  })

  const sendMutation = useMutation({
    mutationFn: ({ sessionId, message }: { sessionId: string; message: string }) =>
      sessionsApi.send(projectId!, sessionId, message),
  })

  const spawnMutation = useMutation({
    mutationFn: ({ agent, mode, taskId, persistent }: { agent: string; mode: 'real' | 'mock'; taskId?: string; persistent?: boolean }) =>
      sessionsApi.spawn(projectId!, agent, mode, taskId, persistent),
    onSuccess: invalidate,
  })

  const resumeMutation = useMutation({
    mutationFn: ({ sessionId }: { sessionId: string }) =>
      sessionsApi.resume(projectId!, sessionId),
    onSuccess: invalidate,
  })

  const approveMutation = useMutation({
    mutationFn: ({ sessionId }: { sessionId: string }) =>
      sessionsApi.approve(projectId!, sessionId),
    onSuccess: invalidate,
  })

  const denyMutation = useMutation({
    mutationFn: ({ sessionId }: { sessionId: string }) =>
      sessionsApi.deny(projectId!, sessionId),
    onSuccess: invalidate,
  })

  const recoverMutation = useMutation({
    mutationFn: ({ sessionId }: { sessionId: string }) =>
      sessionsApi.recover(projectId!, sessionId),
    onSuccess: invalidate,
  })

  function stop(agentName: string) {
    const session = query.data?.find(
      (s) => s.agent_name === agentName && ['Working', 'Idle', 'Booting'].includes(s.status),
    )
    if (session) stopMutation.mutate({ sessionId: session.id })
  }

  function archive(sessionId: string) {
    archiveMutation.mutate({ sessionId })
  }

  function sendMessage(sessionId: string, message: string): Promise<void> {
    return sendMutation.mutateAsync({ sessionId, message })
  }

  function spawn(agent: string, mode: 'real' | 'mock', taskId?: string, persistent = false): Promise<{ status: string; output: string }> {
    return spawnMutation.mutateAsync({ agent, mode, taskId, persistent })
  }

  function resume(sessionId: string): Promise<{ status: string; output: string }> {
    return resumeMutation.mutateAsync({ sessionId })
  }

  function approve(sessionId: string): Promise<{ status: string; output: string }> {
    return approveMutation.mutateAsync({ sessionId })
  }

  function deny(sessionId: string): Promise<{ status: string; output: string }> {
    return denyMutation.mutateAsync({ sessionId })
  }

  function recover(sessionId: string): Promise<{ status: string; output: string }> {
    return recoverMutation.mutateAsync({ sessionId })
  }

  return {
    ...query,
    stop,
    archive,
    sendMessage,
    spawn,
    resume,
    approve,
    deny,
    recover,
    spawnMutation,
    resumeMutation,
    approveMutation,
    denyMutation,
    recoverMutation,
  }
}
