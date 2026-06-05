package dto

// Session is the wire representation of an AOM session record.
type Session struct {
	ID         string `json:"id"`
	AgentName  string `json:"agent_name"`
	Status     string `json:"status"`
	TaskID     string `json:"task_id,omitempty"`
	TmuxPane   string `json:"tmux_pane,omitempty"`
	CreatedAt  string `json:"created_at"`
	Persistent bool   `json:"persistent,omitempty"`
}
