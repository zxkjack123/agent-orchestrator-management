export type Session = {
  id: string
  agent_name: string
  role_name?: string
  runtime?: string
  status: string
  task_id?: string
  worktree_path?: string
  tmux_pane?: string
  tmux_session_name?: string
  vendor_session_id?: string
  resumable: boolean
  created_at: string
  persistent?: boolean
}
