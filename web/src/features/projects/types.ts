export type Project = {
  id: string
  name: string
  path: string
  added_at: string
}

export type Agent = {
  name: string
  role: string
  runtime: string
  enabled: boolean
  tmux_pane?: string
  status?: string
  is_shared_session?: boolean
}

export type ProjectStatus = {
  project_id: string
  project_name: string
  agents: Agent[]
  active_count: number
  idle_count: number
}
