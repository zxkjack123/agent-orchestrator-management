package session

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const defaultStatus = "Created"

// IDGenerator produces durable AOM session IDs.
type IDGenerator func() string

// CreateParams describes the minimum input needed to create a durable session record.
type CreateParams struct {
	ProjectID       string
	AgentID         string
	AgentName       string
	RoleName        string
	TaskID          string
	Runtime         string
	Model           string // optional; empty means provider default
	Status          string
	RepoPath        string
	WorktreePath    string
	TmuxSessionName string
	TmuxWindow      string
	TmuxPane        string
}

// Service owns session lifecycle persistence and state transitions.
type Service struct {
	repo  *Repository
	idGen IDGenerator
	now   func() time.Time
}

// NewService creates a session service backed by the provided database.
func NewService(db *sql.DB) *Service {
	return NewServiceWithIDGenerator(db, defaultIDGenerator())
}

// NewServiceWithIDGenerator creates a session service with an injected ID generator.
func NewServiceWithIDGenerator(db *sql.DB, idGen IDGenerator) *Service {
	if idGen == nil {
		idGen = defaultIDGenerator()
	}

	return &Service{
		repo:  NewRepository(db),
		idGen: idGen,
		now:   time.Now,
	}
}

// Create inserts a new durable session record.
func (s *Service) Create(params CreateParams) (*Record, error) {
	if strings.TrimSpace(params.ProjectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	if strings.TrimSpace(params.AgentName) == "" {
		return nil, fmt.Errorf("agent name is required")
	}
	if strings.TrimSpace(params.RoleName) == "" {
		return nil, fmt.Errorf("role name is required")
	}
	if strings.TrimSpace(params.Runtime) == "" {
		return nil, fmt.Errorf("runtime is required")
	}
	if strings.TrimSpace(params.RepoPath) == "" {
		return nil, fmt.Errorf("repo path is required")
	}

	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = defaultStatus
	}

	record := Record{
		ID:              s.idGen(),
		ProjectID:       strings.TrimSpace(params.ProjectID),
		AgentID:         strings.TrimSpace(params.AgentID),
		AgentName:       strings.TrimSpace(params.AgentName),
		RoleName:        strings.TrimSpace(params.RoleName),
		TaskID:          strings.TrimSpace(params.TaskID),
		Runtime:         strings.TrimSpace(params.Runtime),
		Model:           strings.TrimSpace(params.Model),
		Status:          status,
		RepoPath:        strings.TrimSpace(params.RepoPath),
		WorktreePath:    strings.TrimSpace(params.WorktreePath),
		TmuxSessionName: strings.TrimSpace(params.TmuxSessionName),
		TmuxWindow:      strings.TrimSpace(params.TmuxWindow),
		TmuxPane:        strings.TrimSpace(params.TmuxPane),
	}

	if err := s.repo.Upsert(record); err != nil {
		return nil, err
	}

	return s.repo.GetByID(record.ID)
}

// Save upserts an existing session record.
func (s *Service) Save(record Record) (*Record, error) {
	if strings.TrimSpace(record.ID) == "" {
		return nil, fmt.Errorf("session id is required")
	}
	if strings.TrimSpace(record.ProjectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	if strings.TrimSpace(record.AgentName) == "" {
		return nil, fmt.Errorf("agent name is required")
	}
	if strings.TrimSpace(record.RoleName) == "" {
		return nil, fmt.Errorf("role name is required")
	}
	if strings.TrimSpace(record.Runtime) == "" {
		return nil, fmt.Errorf("runtime is required")
	}
	if strings.TrimSpace(record.RepoPath) == "" {
		return nil, fmt.Errorf("repo path is required")
	}

	if err := s.repo.Upsert(record); err != nil {
		return nil, err
	}

	return s.repo.GetByID(record.ID)
}

// Get returns one durable session by ID.
func (s *Service) Get(id string) (*Record, error) {
	return s.repo.GetByID(id)
}

// ListByProject returns all durable sessions for one project.
func (s *Service) ListByProject(projectID string) ([]Record, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}

	return s.repo.ListByProjectID(strings.TrimSpace(projectID))
}

// ReconcileBinding updates one session to reflect whether its tmux pane is still live.
func (s *Service) ReconcileBinding(record Record, paneExists bool) (*Record, error) {
	if strings.TrimSpace(record.ID) == "" {
		return nil, fmt.Errorf("session id is required")
	}

	next := record
	changed := false

	if paneExists {
		now := s.now()
		next.LastSeenAt = &now
		changed = true
		if record.Status == "Detached" {
			next.Status = "Idle"
		}
	} else if shouldMarkDetached(record) {
		next.Status = "Detached"
		changed = true
	}

	if !changed {
		return &record, nil
	}

	return s.Save(next)
}

// Stop marks a session as intentionally stopped.
func (s *Service) Stop(record Record) (*Record, error) {
	if strings.TrimSpace(record.ID) == "" {
		return nil, fmt.Errorf("session id is required")
	}
	if !canStop(record.Status) {
		return nil, fmt.Errorf("session %q cannot transition from %s to Stopped", record.ID, record.Status)
	}

	record.Status = "Stopped"
	return s.Save(record)
}

// ActiveByAgent returns non-terminal sessions for a specific agent, used to detect
// duplicate spawns before a new session is created.
func (s *Service) ActiveByAgent(projectID, agentName string) ([]Record, error) {
	return s.repo.ActiveByAgent(projectID, agentName)
}

// IsVendorSessionIDActive returns true when the native CLI session ID is already
// registered to a live session in the project, preventing duplicate assignment
// when two sessions are spawned close together.
func (s *Service) IsVendorSessionIDActive(projectID, vendorSessionID string) (bool, error) {
	return s.repo.IsVendorSessionIDActive(projectID, vendorSessionID)
}

// LatestVendorSessionID returns the most recent native CLI session ID for the given
// task and agent, so AOM can resume the prior session rather than starting fresh.
func (s *Service) LatestVendorSessionID(taskID, agentName string) (string, error) {
	return s.repo.LatestVendorSessionID(taskID, agentName)
}

// SetVendorSessionID registers the native CLI session ID against an AOM session record.
// Operators call this after spawn once they know the agent's own session identifier.
func (s *Service) SetVendorSessionID(id, vendorSessionID string) (*Record, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("session id is required")
	}
	v := strings.TrimSpace(vendorSessionID)
	if v == "" {
		return nil, fmt.Errorf("vendor session id is required")
	}
	record, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("session %q not found", id)
	}
	record.VendorSessionID = v
	return s.Save(*record)
}

// Archive marks an inactive session as archived.
func (s *Service) Archive(record Record) (*Record, error) {
	if strings.TrimSpace(record.ID) == "" {
		return nil, fmt.Errorf("session id is required")
	}
	if !canArchive(record.Status) {
		return nil, fmt.Errorf("session %q cannot transition from %s to Archived", record.ID, record.Status)
	}

	record.Status = "Archived"
	return s.Save(record)
}

func shouldMarkDetached(record Record) bool {
	if strings.TrimSpace(record.TmuxPane) == "" || strings.TrimSpace(record.TmuxSessionName) == "" {
		return false
	}

	switch record.Status {
	case "Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked":
		return true
	default:
		return false
	}
}

func canStop(status string) bool {
	switch status {
	case "Idle", "WaitingHandoff", "Detached":
		return true
	default:
		return false
	}
}

func canArchive(status string) bool {
	switch status {
	case "Created", "Failed", "Stopped":
		return true
	default:
		return false
	}
}

func defaultIDGenerator() IDGenerator {
	return func() string {
		return "SESS-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
}
