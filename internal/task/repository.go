package task

import (
	"database/sql"
	"fmt"
	"time"
)

// Record is the persisted workflow task model.
type Record struct {
	ID             string
	ProjectID      string
	Title          string
	Description    string
	Mode           string
	Status         string
	Priority       int
	PreferredRole  string
	PreferredAgent string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Repository persists durable task state.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a task repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Upsert inserts or updates a task record.
func (r *Repository) Upsert(record Record) error {
	_, err := r.db.Exec(`
INSERT INTO tasks (
	id,
	project_id,
	title,
	description,
	mode,
	status,
	priority,
	preferred_role,
	preferred_agent
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	project_id = excluded.project_id,
	title = excluded.title,
	description = excluded.description,
	mode = excluded.mode,
	status = excluded.status,
	priority = excluded.priority,
	preferred_role = excluded.preferred_role,
	preferred_agent = excluded.preferred_agent,
	updated_at = CURRENT_TIMESTAMP
`,
		record.ID,
		record.ProjectID,
		record.Title,
		record.Description,
		record.Mode,
		record.Status,
		record.Priority,
		record.PreferredRole,
		record.PreferredAgent,
	)
	if err != nil {
		return fmt.Errorf("upsert task %q: %w", record.ID, err)
	}

	return nil
}

// GetByID returns one task record by durable task ID.
func (r *Repository) GetByID(id string) (*Record, error) {
	row := r.db.QueryRow(`
SELECT
	id,
	project_id,
	title,
	description,
	mode,
	status,
	priority,
	preferred_role,
	preferred_agent,
	created_at,
	updated_at
FROM tasks
WHERE id = ?
`,
		id,
	)

	record, err := scanRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get task %q: %w", id, err)
	}

	return record, nil
}

// CountByProjectID returns the durable task count for one project.
func (r *Repository) CountByProjectID(projectID string) (int, error) {
	var count int
	if err := r.db.QueryRow(`SELECT COUNT(1) FROM tasks WHERE project_id = ?`, projectID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count tasks for project %q: %w", projectID, err)
	}

	return count, nil
}

// ListByProjectID returns task records for one project ordered by priority (desc) then update time.
func (r *Repository) ListByProjectID(projectID string) ([]Record, error) {
	rows, err := r.db.Query(`
SELECT
	id,
	project_id,
	title,
	description,
	mode,
	status,
	priority,
	preferred_role,
	preferred_agent,
	created_at,
	updated_at
FROM tasks
WHERE project_id = ?
ORDER BY priority DESC, updated_at DESC, created_at DESC, id DESC
`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tasks for project %q: %w", projectID, err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task row: %w", err)
		}
		records = append(records, *record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task rows: %w", err)
	}

	return records, nil
}

// AddDependency records that dependentTaskID is blocked by blockingTaskID.
func (r *Repository) AddDependency(dependentTaskID, blockingTaskID string) error {
	_, err := r.db.Exec(`
INSERT INTO task_dependencies (dependent_task_id, blocking_task_id)
VALUES (?, ?)
ON CONFLICT DO NOTHING
`,
		dependentTaskID, blockingTaskID,
	)
	if err != nil {
		return fmt.Errorf("add dependency %q blocks %q: %w", blockingTaskID, dependentTaskID, err)
	}

	return nil
}

// RemoveDependency removes the blocking relationship between two tasks.
func (r *Repository) RemoveDependency(dependentTaskID, blockingTaskID string) error {
	_, err := r.db.Exec(`
DELETE FROM task_dependencies
WHERE dependent_task_id = ? AND blocking_task_id = ?
`,
		dependentTaskID, blockingTaskID,
	)
	if err != nil {
		return fmt.Errorf("remove dependency %q blocks %q: %w", blockingTaskID, dependentTaskID, err)
	}

	return nil
}

// BlockedByIDs returns the IDs of tasks that block the given task.
func (r *Repository) BlockedByIDs(taskID string) ([]string, error) {
	rows, err := r.db.Query(`
SELECT blocking_task_id FROM task_dependencies
WHERE dependent_task_id = ?
`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("list blockers for task %q: %w", taskID, err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan blocker id: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// UnblocksIDs returns the IDs of tasks that are blocked by the given task.
func (r *Repository) UnblocksIDs(taskID string) ([]string, error) {
	rows, err := r.db.Query(`
SELECT dependent_task_id FROM task_dependencies
WHERE blocking_task_id = ?
`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("list dependents for task %q: %w", taskID, err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan dependent id: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// AllDependencyEdges returns all (dependent, blocking) pairs for cycle detection.
func (r *Repository) AllDependencyEdges() ([][2]string, error) {
	rows, err := r.db.Query(`SELECT dependent_task_id, blocking_task_id FROM task_dependencies`)
	if err != nil {
		return nil, fmt.Errorf("list all dependency edges: %w", err)
	}
	defer rows.Close()

	var edges [][2]string
	for rows.Next() {
		var dep, blk string
		if err := rows.Scan(&dep, &blk); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		edges = append(edges, [2]string{dep, blk})
	}

	return edges, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(scanner rowScanner) (*Record, error) {
	var record Record
	if err := scanner.Scan(
		&record.ID,
		&record.ProjectID,
		&record.Title,
		&record.Description,
		&record.Mode,
		&record.Status,
		&record.Priority,
		&record.PreferredRole,
		&record.PreferredAgent,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &record, nil
}
