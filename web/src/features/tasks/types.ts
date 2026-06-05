export interface Task {
  id: string
  title: string
  description?: string
  status: string
  mode: string
  priority: number
  preferred_agent?: string
  preferred_role?: string
  created_at: string
  updated_at: string
}
