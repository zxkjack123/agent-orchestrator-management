package dto

// RoleDefinition is the wire representation of a role in agents.yaml.
type RoleDefinition struct {
	Name                  string   `json:"name"`
	Class                 string   `json:"class"`
	WorktreeMode          string   `json:"worktree_mode"`
	CheckpointExpectation string   `json:"checkpoint_expectation"`
	DefaultSessionMode    string   `json:"default_session_mode"`
	AgentsUsing           []string `json:"agents_using"`
	Description           string   `json:"description"`
}

// CreateRoleRequest is the body for POST /api/v1/projects/{id}/roles.
type CreateRoleRequest struct {
	Name                  string `json:"name"`
	Class                 string `json:"class"`
	WorktreeMode          string `json:"worktree_mode"`
	CheckpointExpectation string `json:"checkpoint_expectation"`
	DefaultSessionMode    string `json:"default_session_mode"`
	Description           string `json:"description"`
}

// UpdateRoleRequest is the body for PUT /api/v1/projects/{id}/roles/{name}.
type UpdateRoleRequest struct {
	Class                 *string `json:"class,omitempty"`
	WorktreeMode          *string `json:"worktree_mode,omitempty"`
	CheckpointExpectation *string `json:"checkpoint_expectation,omitempty"`
	DefaultSessionMode    *string `json:"default_session_mode,omitempty"`
	Description           *string `json:"description,omitempty"`
}

// ClassInfo is the wire representation of a class template summary.
type ClassInfo struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"` // "builtin" | "custom" | "builtin-overridden"
	RolesUsing  []string `json:"roles_using"`
	Description string   `json:"description"`
}

// ClassDetail is ClassInfo plus template content and edit permissions.
type ClassDetail struct {
	ClassInfo
	Content     string `json:"content"`
	IsProtected bool   `json:"is_protected"`
}

// SetClassRequest is the body for PUT /api/v1/projects/{id}/classes/{name}.
type SetClassRequest struct {
	Content string `json:"content"`
}

// ClassPreviewResponse is returned by GET .../classes/{name}/preview.
type ClassPreviewResponse struct {
	Rendered string `json:"rendered"`
}

// SystemTemplateResponse is returned by GET /api/v1/system-template.
type SystemTemplateResponse struct {
	Content string `json:"content"`
}
