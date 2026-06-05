export interface RoleDefinition {
  name: string
  class: string
  worktree_mode: string
  checkpoint_expectation: string
  default_session_mode: string
  agents_using: string[]
  description: string
}

export interface ClassInfo {
  name: string
  source: 'builtin' | 'custom' | 'builtin-overridden'
  roles_using: string[]
  description: string
}

export interface ClassDetail extends ClassInfo {
  content: string
  is_protected: boolean
}

export interface CreateRoleForm {
  name: string
  class: string
  worktree_mode: string
  checkpoint_expectation: string
  default_session_mode: string
  description: string
}
