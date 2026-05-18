package agent

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/config"
)

// Record is the persisted agent model for Milestone 1.
type Record struct {
	ID        string
	ProjectID string
	Name      string
	Runtime   string
	Role      string
	Enabled   bool
	Model     string // optional; empty means use the CLI's default model
}

// Repository persists agent state.
type Repository struct {
	db *sql.DB
}

// NewRepository creates an agent repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Sync upserts agents from config into the database.
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

	return nil
}

// Upsert inserts or updates an agent record.
func (r *Repository) Upsert(record Record) error {
	enabled := 0
	if record.Enabled {
		enabled = 1
	}

	_, err := r.db.Exec(`
INSERT INTO agents (id, project_id, name, runtime, role, enabled, model)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	project_id = excluded.project_id,
	name = excluded.name,
	runtime = excluded.runtime,
	role = excluded.role,
	enabled = excluded.enabled,
	model = excluded.model
`,
		record.ID,
		record.ProjectID,
		record.Name,
		record.Runtime,
		record.Role,
		enabled,
		record.Model,
	)
	if err != nil {
		return fmt.Errorf("upsert agent %q: %w", record.ID, err)
	}

	return nil
}

// ListByProjectID returns agents for a project ordered by name.
func (r *Repository) ListByProjectID(projectID string) ([]Record, error) {
	rows, err := r.db.Query(`
SELECT id, project_id, name, runtime, role, enabled, model
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
		if err := rows.Scan(&record.ID, &record.ProjectID, &record.Name, &record.Runtime, &record.Role, &enabled, &record.Model); err != nil {
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
