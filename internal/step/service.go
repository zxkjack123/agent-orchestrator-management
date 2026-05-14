package step

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var defaultStepIDSequence atomic.Int64

// IDGenerator produces durable AOM step IDs.
type IDGenerator func() string

// CreateParams describes the minimum input needed to create one step.
type CreateParams struct {
	ProjectID    string
	TaskID       string
	StepType     string
	Title        string
	Status       string
	RoleName     string
	AgentName    string
	Dependencies []string
}

// UpdateParams describes mutable step fields in Milestone 3.
type UpdateParams struct {
	Status    string
	RoleName  string
	AgentName string
}

// Service owns step retrieval and update behavior for Milestone 3.
type Service struct {
	repo  *Repository
	idGen IDGenerator
}

// NewService creates a step service backed by the provided database.
func NewService(db *sql.DB) *Service {
	return NewServiceWithIDGenerator(db, defaultIDGenerator())
}

// NewServiceWithIDGenerator creates a step service with an injected ID generator.
func NewServiceWithIDGenerator(db *sql.DB, idGen IDGenerator) *Service {
	if idGen == nil {
		idGen = defaultIDGenerator()
	}

	return &Service{
		repo:  NewRepository(db),
		idGen: idGen,
	}
}

// Create inserts one explicit workflow step.
func (s *Service) Create(params CreateParams) (*Record, error) {
	projectID := strings.TrimSpace(params.ProjectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	taskID := strings.TrimSpace(params.TaskID)
	if taskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return nil, fmt.Errorf("step title is required")
	}

	stepType := strings.TrimSpace(params.StepType)
	if stepType == "" {
		stepType = "implementation"
	}

	status := "Proposed"
	if strings.TrimSpace(params.Status) != "" {
		normalizedStatus, err := normalizeStatus(params.Status)
		if err != nil {
			return nil, err
		}
		status = normalizedStatus
	}

	record := Record{
		ID:           s.idGen(),
		ProjectID:    projectID,
		TaskID:       taskID,
		StepType:     stepType,
		Title:        title,
		Status:       status,
		RoleName:     strings.TrimSpace(params.RoleName),
		AgentName:    strings.TrimSpace(params.AgentName),
		Dependencies: append([]string(nil), params.Dependencies...),
	}
	if record.Status == "Ready" && record.RoleName == "" && record.AgentName == "" {
		return nil, fmt.Errorf("step %q needs a role or agent before entering Ready", record.ID)
	}

	if err := s.repo.Upsert(record); err != nil {
		return nil, err
	}

	return s.repo.GetByID(record.ID)
}

// Get returns one step by ID.
func (s *Service) Get(id string) (*Record, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("step id is required")
	}

	return s.repo.GetByID(strings.TrimSpace(id))
}

// ListByTask returns all steps for one task.
func (s *Service) ListByTask(taskID string) ([]Record, error) {
	if strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("task id is required")
	}

	return s.repo.ListByTaskID(strings.TrimSpace(taskID))
}

// Update mutates step ownership or status with transition validation.
func (s *Service) Update(id string, params UpdateParams) (*Record, error) {
	record, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("step %q not found", strings.TrimSpace(id))
	}

	next := *record
	changed := false

	if params.RoleName != "" {
		next.RoleName = strings.TrimSpace(params.RoleName)
		changed = true
	}
	if params.AgentName != "" {
		next.AgentName = strings.TrimSpace(params.AgentName)
		changed = true
	}
	if params.Status != "" {
		status, err := normalizeStatus(params.Status)
		if err != nil {
			return nil, err
		}
		if err := validateTransition(record.Status, status); err != nil {
			return nil, err
		}
		next.Status = status
		changed = true
	}

	if !changed {
		return nil, fmt.Errorf("at least one step field must be updated")
	}
	if next.Status == "Ready" && strings.TrimSpace(next.RoleName) == "" && strings.TrimSpace(next.AgentName) == "" {
		return nil, fmt.Errorf("step %q needs a role or agent before entering Ready", next.ID)
	}

	if err := s.repo.Upsert(next); err != nil {
		return nil, err
	}

	return s.repo.GetByID(next.ID)
}

// AssignOwner updates step ownership explicitly, including clearing the assigned agent when needed.
func (s *Service) AssignOwner(id, roleName, agentName string) (*Record, error) {
	record, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("step %q not found", strings.TrimSpace(id))
	}

	next := *record
	next.RoleName = strings.TrimSpace(roleName)
	next.AgentName = strings.TrimSpace(agentName)

	if next.RoleName == record.RoleName && next.AgentName == record.AgentName {
		return record, nil
	}

	if next.Status == "Ready" && strings.TrimSpace(next.RoleName) == "" && strings.TrimSpace(next.AgentName) == "" {
		return nil, fmt.Errorf("step %q needs a role or agent before entering Ready", next.ID)
	}

	if err := s.repo.Upsert(next); err != nil {
		return nil, err
	}

	return s.repo.GetByID(next.ID)
}

func normalizeStatus(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "proposed":
		return "Proposed", nil
	case "confirmed":
		return "Confirmed", nil
	case "ready":
		return "Ready", nil
	case "inprogress", "in-progress":
		return "InProgress", nil
	case "blocked":
		return "Blocked", nil
	case "needsattention", "needs-attention":
		return "NeedsAttention", nil
	case "completed":
		return "Completed", nil
	case "skipped":
		return "Skipped", nil
	case "canceled", "cancelled":
		return "Canceled", nil
	default:
		return "", fmt.Errorf("step status %q is not recognized", input)
	}
}

func validateTransition(current, next string) error {
	if current == next {
		return nil
	}

	allowed := map[string]map[string]bool{
		"Proposed": {
			"Confirmed": true,
			"Skipped":   true,
			"Canceled":  true,
		},
		"Confirmed": {
			"Ready":    true,
			"Canceled": true,
		},
		"Ready": {
			"InProgress": true,
			"Skipped":    true,
			"Canceled":   true,
		},
		"InProgress": {
			"Blocked":        true,
			"NeedsAttention": true,
			"Completed":      true,
			"Ready":          true,
		},
		"Blocked": {
			"Ready":          true,
			"NeedsAttention": true,
		},
		"NeedsAttention": {
			"Ready":      true,
			"InProgress": true,
			"Canceled":   true,
		},
	}

	if allowed[current][next] {
		return nil
	}

	return fmt.Errorf("step transition %s -> %s is not allowed", current, next)
}

func defaultIDGenerator() IDGenerator {
	return func() string {
		timestamp := time.Now().UnixNano()
		sequence := defaultStepIDSequence.Add(1)
		return "STEP-" + strconv.FormatInt(timestamp, 10) + "-" + strconv.FormatInt(sequence, 10)
	}
}
