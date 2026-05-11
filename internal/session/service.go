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
	Runtime         string
	Status          string
	RepoPath        string
	WorktreePath    string
	TmuxSessionName string
	TmuxWindow      string
	TmuxPane        string
}

// Service owns session creation and listing behavior for Milestone 2.
type Service struct {
	repo  *Repository
	idGen IDGenerator
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
		Runtime:         strings.TrimSpace(params.Runtime),
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

func defaultIDGenerator() IDGenerator {
	return func() string {
		return "SESS-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
}
