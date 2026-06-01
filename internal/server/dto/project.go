package dto

// Project is the wire representation of a registered AOM project.
type Project struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	AddedAt string `json:"added_at"`
}

// AddProjectRequest is the body for POST /api/v1/projects.
type AddProjectRequest struct {
	Path string `json:"path"`
}

// Agent is the wire representation of an agent within a project.
type Agent struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Runtime  string `json:"runtime"`
	Enabled  bool   `json:"enabled"`
	TmuxPane string `json:"tmux_pane,omitempty"`
	Status   string `json:"status,omitempty"`
}

// ProjectStatus is the summary payload for GET /api/v1/projects/:id/status.
type ProjectStatus struct {
	ProjectID   string  `json:"project_id"`
	ProjectName string  `json:"project_name"`
	Agents      []Agent `json:"agents"`
	ActiveCount int     `json:"active_count"`
	IdleCount   int     `json:"idle_count"`
}
