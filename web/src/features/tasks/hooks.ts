import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { tasksApi, type TaskSignal } from './api'

export function useTasks(projectId: string | null) {
  return useQuery({
    queryKey: ['projects', projectId, 'tasks'],
    queryFn: () => tasksApi.list(projectId!),
    enabled: !!projectId,
    refetchInterval: 10_000,
  })
}

export function useTask(projectId: string | null, taskId: string | null) {
  return useQuery({
    queryKey: ['projects', projectId, 'tasks', taskId],
    queryFn: () => tasksApi.get(projectId!, taskId!),
    enabled: !!projectId && !!taskId,
    refetchInterval: 10_000,
  })
}

export function useTaskActions(projectId: string | null) {
  const qc = useQueryClient()
  const invalidate = () => qc.invalidateQueries({ queryKey: ['projects', projectId] })

  const signalMutation = useMutation({
    mutationFn: ({ taskId, signal }: { taskId: string; signal: TaskSignal }) =>
      tasksApi.signal(projectId!, taskId, signal),
    onSuccess: invalidate,
  })

  const acceptMutation = useMutation({
    mutationFn: ({ taskId, force }: { taskId: string; force?: boolean }) =>
      tasksApi.accept(projectId!, taskId, force),
    onSuccess: invalidate,
  })

  const createMutation = useMutation({
    mutationFn: ({
      title,
      description,
      mode,
      agent,
      role,
    }: {
      title: string
      description?: string
      mode?: string
      agent?: string
      role?: string
    }) => tasksApi.create(projectId!, title, { description, mode, agent, role }),
    onSuccess: invalidate,
  })

  const closeMutation = useMutation({
    mutationFn: ({ taskId }: { taskId: string }) => tasksApi.close(projectId!, taskId),
    onSuccess: invalidate,
  })

  const cancelMutation = useMutation({
    mutationFn: ({ taskId }: { taskId: string }) => tasksApi.cancel(projectId!, taskId),
    onSuccess: invalidate,
  })

  return { signalMutation, acceptMutation, createMutation, closeMutation, cancelMutation }
}
