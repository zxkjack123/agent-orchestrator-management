package session

import (
	"database/sql"
	"fmt"
	"time"
)

// Record is the persisted live session model for Milestone 2.
type Record struct {
	ID              string
	ProjectID       string
	AgentID         string
	AgentName       string
	RoleName        string
	TaskID          string
	Runtime         string
	Status          string
	RepoPath        string
	WorktreePath    string
	TmuxSessionName string
	TmuxWindow      string
	TmuxPane        string
	VendorSessionID string
	LastSeenAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Repository persists durable session state.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a session repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Upsert inserts or updates a session record.
func (r *Repository) Upsert(record Record) error {
	_, err := r.db.Exec(`
INSERT INTO sessions (
	id,
	project_id,
	agent_id,
	agent_name,
	role_name,
	task_id,
	runtime,
	status,
	repo_path,
	worktree_path,
	tmux_session_name,
	tmux_window,
	tmux_pane,
	vendor_session_id,
	last_seen_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	project_id = excluded.project_id,
	agent_id = excluded.agent_id,
	agent_name = excluded.agent_name,
	role_name = excluded.role_name,
	task_id = excluded.task_id,
	runtime = excluded.runtime,
	status = excluded.status,
	repo_path = excluded.repo_path,
	worktree_path = excluded.worktree_path,
	tmux_session_name = excluded.tmux_session_name,
	tmux_window = excluded.tmux_window,
	tmux_pane = excluded.tmux_pane,
	vendor_session_id = excluded.vendor_session_id,
	last_seen_at = excluded.last_seen_at,
	updated_at = CURRENT_TIMESTAMP
`,
		record.ID,
		record.ProjectID,
		nullableString(record.AgentID),
		record.AgentName,
		record.RoleName,
		nullableString(record.TaskID),
		record.Runtime,
		record.Status,
		record.RepoPath,
		record.WorktreePath,
		record.TmuxSessionName,
		record.TmuxWindow,
		record.TmuxPane,
		record.VendorSessionID,
		nullableTime(record.LastSeenAt),
	)
	if err != nil {
		return fmt.Errorf("upsert session %q: %w", record.ID, err)
	}

	return nil
}

// GetByID returns one session record by its durable session ID.
func (r *Repository) GetByID(id string) (*Record, error) {
	row := r.db.QueryRow(`
SELECT
	id,
	project_id,
	COALESCE(agent_id, ''),
	agent_name,
	role_name,
	COALESCE(task_id, ''),
	runtime,
	status,
	repo_path,
	worktree_path,
	tmux_session_name,
	tmux_window,
	tmux_pane,
	vendor_session_id,
	last_seen_at,
	created_at,
	updated_at
FROM sessions
WHERE id = ?
`,
		id,
	)

	record, err := scanRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get session %q: %w", id, err)
	}

	return record, nil
}

// ListByProjectID returns session records for one project ordered by creation time.
func (r *Repository) ListByProjectID(projectID string) ([]Record, error) {
	rows, err := r.db.Query(`
SELECT
	id,
	project_id,
	COALESCE(agent_id, ''),
	agent_name,
	role_name,
	COALESCE(task_id, ''),
	runtime,
	status,
	repo_path,
	worktree_path,
	tmux_session_name,
	tmux_window,
	tmux_pane,
	vendor_session_id,
	last_seen_at,
	created_at,
	updated_at
FROM sessions
WHERE project_id = ?
ORDER BY created_at, id
`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions for project %q: %w", projectID, err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan session row: %w", err)
		}
		records = append(records, *record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session rows: %w", err)
	}

	return records, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(scanner rowScanner) (*Record, error) {
	var record Record
	var lastSeen sql.NullTime
	if err := scanner.Scan(
		&record.ID,
		&record.ProjectID,
		&record.AgentID,
		&record.AgentName,
		&record.RoleName,
		&record.TaskID,
		&record.Runtime,
		&record.Status,
		&record.RepoPath,
		&record.WorktreePath,
		&record.TmuxSessionName,
		&record.TmuxWindow,
		&record.TmuxPane,
		&record.VendorSessionID,
		&lastSeen,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if lastSeen.Valid {
		record.LastSeenAt = &lastSeen.Time
	}

	return &record, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}

	return *value
}
