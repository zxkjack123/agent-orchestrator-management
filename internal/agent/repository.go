package agent

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/config"
)

// Record is the persisted agent model for Milestone 1.
type Record struct {
	ID            string
	ProjectID     string
	Name          string
	Runtime       string
	Role          string
	Enabled       bool
	Model         string // optional; empty means use the CLI's default model
	WorkspacePath string // optional; empty means use per-task worktrees
}

// Repository persists agent state.
type Repository struct {
	db *sql.DB
}

// NewRepository creates an agent repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Sync upserts agents from config into the database and removes any agents
// that are no longer present in agents.yaml. Config is the authoritative source:
// agents deleted from the YAML file are pruned from the DB on the next open/init.
func (r *Repository) Sync(projectID string, cfg config.AgentsFile) error {
	names := make([]string, 0, len(cfg.Agents))
	for name := range cfg.Agents {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		agentCfg := cfg.Agents[name]
		record := Record{
			ID:        projectID + ":" + name,
			ProjectID: projectID,
			Name:      name,
			Runtime:   agentCfg.Runtime,
			Role:      agentCfg.Role,
			Enabled:   agentCfg.Enabled,
			Model:     agentCfg.Model,
		}
		if err := r.Upsert(record); err != nil {
			return err
		}
	}

	return r.pruneRemovedAgents(projectID, names)
}

// pruneRemovedAgents deletes DB rows for agents that are no longer in the config.
// knownNames is the sorted slice of agent names currently in agents.yaml.
func (r *Repository) pruneRemovedAgents(projectID string, knownNames []string) error {
	if len(knownNames) == 0 {
		_, err := r.db.Exec(`DELETE FROM agents WHERE project_id = ?`, projectID)
		return err
	}
	placeholders := strings.Repeat("?,", len(knownNames))
	placeholders = placeholders[:len(placeholders)-1] // trim trailing comma
	args := make([]interface{}, 0, 1+len(knownNames))
	args = append(args, projectID)
	for _, n := range knownNames {
		args = append(args, n)
	}
	query := fmt.Sprintf(
		`DELETE FROM agents WHERE project_id = ? AND name NOT IN (%s)`,
		placeholders,
	)
	_, err := r.db.Exec(query, args...)
	return err
}

// Upsert inserts or updates an agent record.
func (r *Repository) Upsert(record Record) error {
	enabled := 0
	if record.Enabled {
		enabled = 1
	}

	// workspace_path is runtime state set by "aom agent provision".
	// It must NOT be overwritten during config sync (Sync passes workspace_path="").
	// Preserve the existing DB value whenever the incoming value is empty.
	_, err := r.db.Exec(`
INSERT INTO agents (id, project_id, name, runtime, role, enabled, model, workspace_path)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	project_id = excluded.project_id,
	name = excluded.name,
	runtime = excluded.runtime,
	role = excluded.role,
	enabled = excluded.enabled,
	model = excluded.model,
	workspace_path = CASE
		WHEN excluded.workspace_path != '' THEN excluded.workspace_path
		ELSE agents.workspace_path
	END
`,
		record.ID,
		record.ProjectID,
		record.Name,
		record.Runtime,
		record.Role,
		enabled,
		record.Model,
		record.WorkspacePath,
	)
	if err != nil {
		return fmt.Errorf("upsert agent %q: %w", record.ID, err)
	}

	return nil
}

// ListByProjectID returns agents for a project ordered by name.
func (r *Repository) ListByProjectID(projectID string) ([]Record, error) {
	rows, err := r.db.Query(`
SELECT id, project_id, name, runtime, role, enabled, model, workspace_path
FROM agents
WHERE project_id = ?
ORDER BY name
`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list agents for project %q: %w", projectID, err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		var enabled int
		if err := rows.Scan(&record.ID, &record.ProjectID, &record.Name, &record.Runtime, &record.Role, &enabled, &record.Model, &record.WorkspacePath); err != nil {
			return nil, fmt.Errorf("scan agent row: %w", err)
		}
		record.Enabled = enabled != 0
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent rows: %w", err)
	}

	return records, nil
}

// SetWorkspacePath updates the workspace_path for one agent.
func (r *Repository) SetWorkspacePath(projectID, agentName, path string) error {
	_, err := r.db.Exec(
		`UPDATE agents SET workspace_path = ? WHERE project_id = ? AND name = ?`,
		path, projectID, agentName,
	)
	if err != nil {
		return fmt.Errorf("set workspace path for agent %q: %w", agentName, err)
	}
	return nil
}
