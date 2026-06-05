package dto

// Session is the wire representation of an AOM session record.
type Session struct {
	ID              string `json:"id"`
	AgentName       string `json:"agent_name"`
	RoleName        string `json:"role_name,omitempty"`
	Runtime         string `json:"runtime,omitempty"`
	Status          string `json:"status"`
	TaskID          string `json:"task_id,omitempty"`
	WorktreePath    string `json:"worktree_path,omitempty"`
	TmuxPane        string `json:"tmux_pane,omitempty"`
	TmuxSessionName string `json:"tmux_session_name,omitempty"`
	VendorSessionID string `json:"vendor_session_id,omitempty"`
	// Resumable is true when the session can be resumed: either the pane is still
	// alive or a native VendorSessionID is available for claude --resume / codex resume.
	Resumable bool   `json:"resumable"`
	CreatedAt string `json:"created_at"`
	Persistent bool   `json:"persistent,omitempty"`
}
