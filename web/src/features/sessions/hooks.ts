import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { sessionsApi } from './api'

export function useSessions(projectId: string | null, activeOnly = false) {
  const qc = useQueryClient()

  const query = useQuery({
    queryKey: ['projects', projectId, 'sessions', activeOnly],
    queryFn: () => sessionsApi.list(projectId!, activeOnly),
    enabled: !!projectId,
    refetchInterval: 5_000,
  })

  const stopMutation = useMutation({
    mutationFn: ({ sessionId }: { sessionId: string }) =>
      sessionsApi.stop(projectId!, sessionId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['projects', projectId] })
    },
  })

  function stop(agentName: string) {
    const session = query.data?.find(
      (s) => s.agent_name === agentName && ['Working', 'Idle', 'Booting'].includes(s.status),
    )
    if (session) stopMutation.mutate({ sessionId: session.id })
  }

  return { ...query, stop }
}
