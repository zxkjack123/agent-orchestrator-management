package task

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/step"
)

var defaultIDSequence atomic.Int64

const (
	defaultMode       = "Direct"
	defaultTaskStatus = "Planned"
	defaultStepType   = "implementation"
	defaultStepStatus = "Proposed"
)

// IDGenerator produces durable AOM task and step IDs.
type IDGenerator func() string

// CreateParams describes the minimum input needed to create a task.
type CreateParams struct {
	ProjectID      string
	Title          string
	Mode           string
	PreferredRole  string
	PreferredAgent string
}

// StepSeed describes one step that should be created with a new task.
type StepSeed struct {
	Type         string
	Title        string
	RoleName     string
	AgentName    string
	Dependencies []string
}

// CreateResult returns the created task plus its initial steps.
type CreateResult struct {
	Task  Record
	Steps []step.Record
}

// UpdateParams describes mutable task fields in Milestone 3.
type UpdateParams struct {
	Mode           string
	Status         string
	PreferredRole  string
	PreferredAgent string
}

// Service owns task creation and retrieval behavior for Milestone 3.
type Service struct {
	repo      *Repository
	stepRepo  *step.Repository
	taskIDGen IDGenerator
	stepIDGen IDGenerator
}

// NewService creates a task service backed by the provided database.
func NewService(db *sql.DB) *Service {
	return NewServiceWithGenerators(db, defaultTaskIDGenerator("TASK"), defaultTaskIDGenerator("STEP"))
}

// NewServiceWithGenerators creates a task service with injected generators.
func NewServiceWithGenerators(db *sql.DB, taskIDGen, stepIDGen IDGenerator) *Service {
	if taskIDGen == nil {
		taskIDGen = defaultTaskIDGenerator("TASK")
	}
	if stepIDGen == nil {
		stepIDGen = defaultTaskIDGenerator("STEP")
	}

	return &Service{
		repo:      NewRepository(db),
		stepRepo:  step.NewRepository(db),
		taskIDGen: taskIDGen,
		stepIDGen: stepIDGen,
	}
}

// Create inserts a new task record and one initial step proposal.
func (s *Service) Create(params CreateParams) (*CreateResult, error) {
	initialStep := StepSeed{
		Type:      defaultStepType,
		Title:     strings.TrimSpace(params.Title),
		RoleName:  strings.TrimSpace(params.PreferredRole),
		AgentName: strings.TrimSpace(params.PreferredAgent),
	}

	return s.createTask(params, []StepSeed{initialStep})
}

// CreateFromPlan inserts a new task record and the proposed planned steps.
func (s *Service) CreateFromPlan(params CreateParams, seeds []StepSeed) (*CreateResult, error) {
	if len(seeds) == 0 {
		return nil, fmt.Errorf("at least one planned step is required")
	}

	return s.createTask(params, seeds)
}

func (s *Service) createTask(params CreateParams, seeds []StepSeed) (*CreateResult, error) {
	projectID := strings.TrimSpace(params.ProjectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}

	title := strings.TrimSpace(params.Title)
	if title == "" {
		return nil, fmt.Errorf("task title is required")
	}

	mode, err := normalizeMode(params.Mode)
	if err != nil {
		return nil, err
	}

	taskRecord := Record{
		ID:             s.taskIDGen(),
		ProjectID:      projectID,
		Title:          title,
		Mode:           mode,
		Status:         defaultTaskStatus,
		PreferredRole:  strings.TrimSpace(params.PreferredRole),
		PreferredAgent: strings.TrimSpace(params.PreferredAgent),
	}

	if err := s.repo.Upsert(taskRecord); err != nil {
		return nil, err
	}

	createdStepIDs := make([]string, 0, len(seeds))
	for i, seed := range seeds {
		stepType := strings.TrimSpace(seed.Type)
		if stepType == "" {
			stepType = defaultStepType
		}
		stepTitle := strings.TrimSpace(seed.Title)
		if stepTitle == "" {
			stepTitle = title
		}

		dependencies := append([]string(nil), seed.Dependencies...)
		if len(dependencies) == 0 && i > 0 {
			dependencies = []string{createdStepIDs[i-1]}
		}

		stepID := s.stepIDGen()
		initialStep := step.Record{
			ID:           stepID,
			ProjectID:    projectID,
			TaskID:       taskRecord.ID,
			StepType:     stepType,
			Title:        stepTitle,
			Status:       defaultStepStatus,
			RoleName:     strings.TrimSpace(seed.RoleName),
			AgentName:    strings.TrimSpace(seed.AgentName),
			Dependencies: dependencies,
		}
		if err := s.stepRepo.Upsert(initialStep); err != nil {
			return nil, err
		}
		createdStepIDs = append(createdStepIDs, stepID)
	}

	createdTask, err := s.repo.GetByID(taskRecord.ID)
	if err != nil {
		return nil, err
	}
	steps, err := s.stepRepo.ListByTaskID(taskRecord.ID)
	if err != nil {
		return nil, err
	}

	return &CreateResult{
		Task:  *createdTask,
		Steps: steps,
	}, nil
}

// Get returns one task by ID.
func (s *Service) Get(id string) (*Record, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("task id is required")
	}

	return s.repo.GetByID(strings.TrimSpace(id))
}

// CountByProject returns the durable task count for one project.
func (s *Service) CountByProject(projectID string) (int, error) {
	if strings.TrimSpace(projectID) == "" {
		return 0, fmt.Errorf("project id is required")
	}

	return s.repo.CountByProjectID(strings.TrimSpace(projectID))
}

// ListByProject returns all tasks for one project ordered by most recent activity.
func (s *Service) ListByProject(projectID string) ([]Record, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}

	return s.repo.ListByProjectID(strings.TrimSpace(projectID))
}

// Update mutates task metadata with transition validation.
func (s *Service) Update(id string, params UpdateParams) (*Record, error) {
	record, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("task %q not found", strings.TrimSpace(id))
	}

	next := *record
	changed := false

	if params.Mode != "" {
		mode, err := normalizeMode(params.Mode)
		if err != nil {
			return nil, err
		}
		next.Mode = mode
		changed = true
	}
	if params.PreferredRole != "" {
		next.PreferredRole = strings.TrimSpace(params.PreferredRole)
		changed = true
	}
	if params.PreferredAgent != "" {
		next.PreferredAgent = strings.TrimSpace(params.PreferredAgent)
		changed = true
	}
	if params.Status != "" {
		status, err := normalizeTaskStatus(params.Status)
		if err != nil {
			return nil, err
		}
		if err := validateTaskTransition(record.Status, status); err != nil {
			return nil, err
		}
		next.Status = status
		changed = true
	}

	if !changed {
		return nil, fmt.Errorf("at least one task field must be updated")
	}

	if err := s.repo.Upsert(next); err != nil {
		return nil, err
	}

	return s.repo.GetByID(next.ID)
}

// Close explicitly marks a task Done when allowed by the state machine.
func (s *Service) Close(id string) (*Record, error) {
	return s.Update(id, UpdateParams{Status: "Done"})
}

// AssignOwner updates task ownership explicitly, including clearing the preferred agent when needed.
func (s *Service) AssignOwner(id, roleName, agentName string) (*Record, error) {
	record, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("task %q not found", strings.TrimSpace(id))
	}

	next := *record
	next.PreferredRole = strings.TrimSpace(roleName)
	next.PreferredAgent = strings.TrimSpace(agentName)

	if next.PreferredRole == record.PreferredRole && next.PreferredAgent == record.PreferredAgent {
		return record, nil
	}

	if err := s.repo.Upsert(next); err != nil {
		return nil, err
	}

	return s.repo.GetByID(next.ID)
}

func normalizeMode(input string) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return defaultMode, nil
	}

	switch strings.ToLower(value) {
	case "direct":
		return "Direct", nil
	case "bugfix":
		return "Bugfix", nil
	case "requirements-first":
		return "Requirements-first", nil
	case "design-first":
		return "Design-first", nil
	default:
		return "", fmt.Errorf("task mode %q is not recognized", input)
	}
}

func normalizeTaskStatus(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "draft":
		return "Draft", nil
	case "planned":
		return "Planned", nil
	case "ready":
		return "Ready", nil
	case "inprogress", "in-progress":
		return "InProgress", nil
	case "blocked":
		return "Blocked", nil
	case "needsattention", "needs-attention":
		return "NeedsAttention", nil
	case "done":
		return "Done", nil
	case "archived":
		return "Archived", nil
	default:
		return "", fmt.Errorf("task status %q is not recognized", input)
	}
}

func validateTaskTransition(current, next string) error {
	if current == next {
		return nil
	}

	allowed := map[string]map[string]bool{
		"Draft": {
			"Planned":  true,
			"Archived": true,
		},
		"Planned": {
			"Ready":          true,
			"NeedsAttention": true,
		},
		"Ready": {
			"InProgress": true,
			"Archived":   true,
		},
		"InProgress": {
			"Blocked":        true,
			"NeedsAttention": true,
			"Ready":          true,
			"Done":           true,
		},
		"Blocked": {
			"Ready":          true,
			"NeedsAttention": true,
		},
		"NeedsAttention": {
			"Planned":    true,
			"Ready":      true,
			"InProgress": true,
			"Done":       true,
		},
		"Done": {
			"Archived": true,
		},
	}

	if allowed[current][next] {
		return nil
	}

	return fmt.Errorf("task transition %s -> %s is not allowed", current, next)
}

func defaultTaskIDGenerator(prefix string) IDGenerator {
	return func() string {
		timestamp := time.Now().UnixNano()
		sequence := defaultIDSequence.Add(1)
		return prefix + "-" + strconv.FormatInt(timestamp, 10) + "-" + strconv.FormatInt(sequence, 10)
	}
}
