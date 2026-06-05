export interface Agent {
  name: string
  role: string
  runtime: string
  enabled: boolean
  model?: string
  tmux_pane?: string
  status?: string
  workspace_path?: string // non-empty = workspace (free-roam) mode
}

export interface AddAgentForm {
  name: string
  role: string
  runtime: string
  model?: string
  enabled: boolean
}

export interface UpdateAgentForm {
  model?: string
  enabled?: boolean
}

export interface SpawnSessionForm {
  agent: string
  task_id?: string
  mode: string
  persistent?: boolean
}

export interface SpawnSessionResult {
  status: string
  output: string
}

export interface SetInstructionsResponse {
  status: string
  active_session: boolean
}
