import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { rolesApi } from './api'
import type { CreateRoleForm } from './types'

export function useRoles(projectId: string | null) {
  const qc = useQueryClient()

  const query = useQuery({
    queryKey: ['projects', projectId, 'roles'],
    queryFn: () => rolesApi.listRoles(projectId!),
    enabled: !!projectId,
    refetchInterval: 10_000,
  })

  const createMutation = useMutation({
    mutationFn: (data: CreateRoleForm) => rolesApi.createRole(projectId!, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['projects', projectId, 'roles'] }),
  })

  const updateMutation = useMutation({
    mutationFn: ({ name, data }: { name: string; data: Partial<CreateRoleForm> }) =>
      rolesApi.updateRole(projectId!, name, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['projects', projectId, 'roles'] }),
  })

  const deleteMutation = useMutation({
    mutationFn: (name: string) => rolesApi.deleteRole(projectId!, name),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['projects', projectId, 'roles'] }),
  })

  return {
    ...query,
    createRole: createMutation.mutateAsync,
    updateRole: (name: string, data: Partial<CreateRoleForm>) =>
      updateMutation.mutateAsync({ name, data }),
    deleteRole: deleteMutation.mutateAsync,
  }
}

export function useClasses(projectId: string | null) {
  const qc = useQueryClient()

  const query = useQuery({
    queryKey: ['projects', projectId, 'classes'],
    queryFn: () => rolesApi.listClasses(projectId!),
    enabled: !!projectId,
  })

  const setMutation = useMutation({
    mutationFn: ({ name, content }: { name: string; content: string }) =>
      rolesApi.setClass(projectId!, name, content),
    onSuccess: (_, { name }) => {
      qc.invalidateQueries({ queryKey: ['projects', projectId, 'classes'] })
      qc.invalidateQueries({ queryKey: ['projects', projectId, 'classes', name] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (name: string) => rolesApi.deleteClass(projectId!, name),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['projects', projectId, 'classes'] }),
  })

  return {
    ...query,
    setClass: (name: string, content: string) => setMutation.mutateAsync({ name, content }),
    deleteClass: deleteMutation.mutateAsync,
  }
}

export function useClassDetail(projectId: string | null, className: string | null) {
  return useQuery({
    queryKey: ['projects', projectId, 'classes', className],
    queryFn: () => rolesApi.getClass(projectId!, className!),
    enabled: !!projectId && !!className,
  })
}

export function useSystemTemplate() {
  return useQuery({
    queryKey: ['system-template'],
    queryFn: () => rolesApi.getSystemTemplate(),
    staleTime: Infinity, // never changes — embedded in binary
  })
}
