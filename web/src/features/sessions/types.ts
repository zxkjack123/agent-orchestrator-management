export type Session = {
  id: string
  agent_name: string
  status: string
  task_id?: string
  tmux_pane?: string
  created_at: string
  persistent?: boolean
}
