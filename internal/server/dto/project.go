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
	Name            string `json:"name"`
	Role            string `json:"role"`
	Runtime         string `json:"runtime"`
	Enabled         bool   `json:"enabled"`
	Model           string `json:"model,omitempty"`
	TmuxPane        string `json:"tmux_pane,omitempty"`
	Status          string `json:"status,omitempty"`
	Persistent      bool   `json:"persistent,omitempty"`
	IsSharedSession bool   `json:"is_shared_session,omitempty"`
	WorkspacePath   string `json:"workspace_path,omitempty"`
}

// AddAgentRequest is the body for POST /api/v1/projects/{id}/agents.
type AddAgentRequest struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Runtime string `json:"runtime"`
	Model   string `json:"model,omitempty"`
	Enabled bool   `json:"enabled"`
}

// UpdateAgentRequest is the body for PUT /api/v1/projects/{id}/agents/{name}.
type UpdateAgentRequest struct {
	Model   *string `json:"model,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

// SpawnSessionRequest is the body for POST /api/v1/projects/{id}/sessions.
type SpawnSessionRequest struct {
	Agent      string `json:"agent"`
	TaskID     string `json:"task_id,omitempty"`
	Mode       string `json:"mode"`       // "real" | "mock"
	Persistent bool   `json:"persistent,omitempty"`
}

// SpawnSessionResponse is returned after a successful spawn.
type SpawnSessionResponse struct {
	Status string `json:"status"`
	Output string `json:"output"`
}

// SendMessageRequest is the body for POST /api/v1/projects/{id}/sessions/{sid}/send.
type SendMessageRequest struct {
	Message string `json:"message"`
	From    string `json:"from,omitempty"` // defaults to "operator"
}

// Task is the wire representation of a task record.
type Task struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	Status         string `json:"status"`
	Mode           string `json:"mode"`
	Priority       int    `json:"priority"`
	PreferredAgent string `json:"preferred_agent,omitempty"`
	PreferredRole  string `json:"preferred_role,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// SetInstructionsResponse is returned by PUT .../agents/{name}/instructions.
type SetInstructionsResponse struct {
	Status        string `json:"status"`
	ActiveSession bool   `json:"active_session"`
}

// ProjectStatus is the summary payload for GET /api/v1/projects/:id/status.
type ProjectStatus struct {
	ProjectID   string  `json:"project_id"`
	ProjectName string  `json:"project_name"`
	ProjectPath string  `json:"project_path"`
	Agents      []Agent `json:"agents"`
	ActiveCount int     `json:"active_count"`
	IdleCount   int     `json:"idle_count"`
}
