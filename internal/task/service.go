package task

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/step"
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

// PriorityHigh, PriorityNormal, PriorityLow are the canonical priority levels.
const (
	PriorityHigh   = 10
	PriorityNormal = 0
	PriorityLow    = -10
)

// CreateParams describes the minimum input needed to create a task.
type CreateParams struct {
	ProjectID       string
	Title           string
	Description     string
	Mode            string
	Priority        int
	PreferredRole   string
	PreferredAgent  string
	InitialStepType string
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

// UpdateParams describes mutable task fields.
type UpdateParams struct {
	Mode           string
	Status         string
	Priority       string // "high", "normal", "low", or "" to leave unchanged
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
	stepType := strings.TrimSpace(params.InitialStepType)
	if stepType == "" {
		stepType = defaultStepType
	}
	initialStep := StepSeed{
		Type:      stepType,
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
		Description:    strings.TrimSpace(params.Description),
		Mode:           mode,
		Status:         defaultTaskStatus,
		Priority:       params.Priority,
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
	if params.Priority != "" {
		p, err := NormalizePriority(params.Priority)
		if err != nil {
			return nil, err
		}
		next.Priority = p
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

	if next.Status == "Done" && record.Status != "Done" {
		s.promoteUnblockedDependents(next.ID)
	}

	return s.repo.GetByID(next.ID)
}

// promoteUnblockedDependents transitions any Planned tasks whose every blocker is now
// Done or Archived to the Ready state. Errors are silently ignored so a Done transition
// never fails due to a dependent-promotion issue.
func (s *Service) promoteUnblockedDependents(doneTaskID string) {
	dependentIDs, err := s.repo.UnblocksIDs(doneTaskID)
	if err != nil {
		return
	}
	for _, depID := range dependentIDs {
		dep, err := s.repo.GetByID(depID)
		if err != nil || dep == nil || dep.Status != "Planned" {
			continue
		}
		blockerIDs, err := s.repo.BlockedByIDs(depID)
		if err != nil {
			continue
		}
		allDone := true
		for _, bID := range blockerIDs {
			b, err := s.repo.GetByID(bID)
			if err != nil || b == nil || (b.Status != "Done" && b.Status != "Archived") {
				allDone = false
				break
			}
		}
		if allDone {
			next := *dep
			next.Status = "Ready"
			_ = s.repo.Upsert(next)
		}
	}
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
	case "pendingapproval", "pending-approval":
		return "PendingApproval", nil
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
			"Ready":           true,
			"NeedsAttention":  true,
			"PendingApproval": true,
			"Archived":        true,
		},
		"Ready": {
			"InProgress":      true,
			"PendingApproval": true,
			"Archived":        true,
		},
		"InProgress": {
			"Blocked":         true,
			"NeedsAttention":  true,
			"Ready":           true,
			"PendingApproval": true,
			"Done":            true,
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
		"PendingApproval": {
			"Ready":   true,
			"Blocked": true,
		},
		"Done": {
			"Archived": true,
		},
	}

	if allowed[current][next] {
		return nil
	}

	validNext := make([]string, 0)
	for state := range allowed[current] {
		validNext = append(validNext, state)
	}
	sort.Strings(validNext)
	if len(validNext) == 0 {
		return fmt.Errorf("task transition %s -> %s is not allowed (no further transitions from %s)", current, next, current)
	}
	return fmt.Errorf("task transition %s -> %s is not allowed\nValid next states from %s: %s", current, next, current, strings.Join(validNext, ", "))
}

// ValidTransitions returns the allowed next states for the given task status,
// sorted alphabetically. Returns nil for terminal states with no transitions.
func ValidTransitions(current string) []string {
	allowed := map[string][]string{
		"Draft":          {"Archived", "Planned"},
		"Planned":        {"Archived", "NeedsAttention", "Ready"},
		"Ready":          {"Archived", "InProgress"},
		"InProgress":     {"Blocked", "Done", "NeedsAttention", "Ready"},
		"Blocked":        {"NeedsAttention", "Ready"},
		"NeedsAttention": {"Done", "InProgress", "Planned", "Ready"},
		"Done":           {"Archived"},
	}
	return allowed[current]
}

// NormalizePriority converts a human-readable priority label to its integer value.
func NormalizePriority(input string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "high":
		return PriorityHigh, nil
	case "normal", "":
		return PriorityNormal, nil
	case "low":
		return PriorityLow, nil
	default:
		return 0, fmt.Errorf("priority %q is not recognized (use high, normal, or low)", input)
	}
}

// PriorityLabel returns a human-readable label for a priority integer.
func PriorityLabel(p int) string {
	switch {
	case p >= PriorityHigh:
		return "high"
	case p <= PriorityLow:
		return "low"
	default:
		return "normal"
	}
}

// AddDependency records that dependentID is blocked by blockingID.
// Returns an error if either task does not exist or if adding the dependency
// would create a cycle.
func (s *Service) AddDependency(dependentID, blockingID string) error {
	dependentID = strings.TrimSpace(dependentID)
	blockingID = strings.TrimSpace(blockingID)

	if dependentID == blockingID {
		return fmt.Errorf("a task cannot depend on itself")
	}

	dep, err := s.repo.GetByID(dependentID)
	if err != nil {
		return err
	}
	if dep == nil {
		return fmt.Errorf("task %q not found", dependentID)
	}

	blk, err := s.repo.GetByID(blockingID)
	if err != nil {
		return err
	}
	if blk == nil {
		return fmt.Errorf("task %q not found", blockingID)
	}

	// Cycle detection: adding (dependent → blocking) creates a cycle if
	// blockingID can already reach dependentID through existing edges.
	if reachable, err := s.canReach(blockingID, dependentID); err != nil {
		return fmt.Errorf("cycle check: %w", err)
	} else if reachable {
		return fmt.Errorf("adding dependency would create a cycle: %s already depends (transitively) on %s", blockingID, dependentID)
	}

	return s.repo.AddDependency(dependentID, blockingID)
}

// RemoveDependency removes a blocking relationship between two tasks.
func (s *Service) RemoveDependency(dependentID, blockingID string) error {
	dependentID = strings.TrimSpace(dependentID)
	blockingID = strings.TrimSpace(blockingID)
	return s.repo.RemoveDependency(dependentID, blockingID)
}

// BlockedBy returns the tasks that block the given task.
func (s *Service) BlockedBy(taskID string) ([]Record, error) {
	ids, err := s.repo.BlockedByIDs(strings.TrimSpace(taskID))
	if err != nil {
		return nil, err
	}
	return s.fetchByIDs(ids)
}

// Unblocks returns the tasks that the given task is blocking.
func (s *Service) Unblocks(taskID string) ([]Record, error) {
	ids, err := s.repo.UnblocksIDs(strings.TrimSpace(taskID))
	if err != nil {
		return nil, err
	}
	return s.fetchByIDs(ids)
}

// ListUnblocked returns all tasks in a project that have no active blockers,
// ordered by priority (high first) then creation order.
func (s *Service) ListUnblocked(projectID string) ([]Record, error) {
	all, err := s.repo.ListByProjectID(strings.TrimSpace(projectID))
	if err != nil {
		return nil, err
	}

	var unblocked []Record
	for _, t := range all {
		if t.Status == "Done" || t.Status == "Archived" {
			continue
		}
		blockerIDs, err := s.repo.BlockedByIDs(t.ID)
		if err != nil {
			return nil, err
		}

		// A task is unblocked when it has no blockers, or all blockers are Done/Archived.
		blocked := false
		for _, bID := range blockerIDs {
			b, err := s.repo.GetByID(bID)
			if err != nil {
				return nil, err
			}
			if b != nil && b.Status != "Done" && b.Status != "Archived" {
				blocked = true
				break
			}
		}
		if !blocked {
			unblocked = append(unblocked, t)
		}
	}

	return unblocked, nil
}

// canReach returns true if startID can reach targetID through dependency edges.
func (s *Service) canReach(startID, targetID string) (bool, error) {
	edges, err := s.repo.AllDependencyEdges()
	if err != nil {
		return false, err
	}

	// Build adjacency: dep → blocking (direction of "is blocked by").
	adj := make(map[string][]string)
	for _, e := range edges {
		dep, blk := e[0], e[1]
		adj[dep] = append(adj[dep], blk)
	}

	visited := make(map[string]bool)
	queue := []string{startID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == targetID {
			return true, nil
		}
		if visited[cur] {
			continue
		}
		visited[cur] = true
		queue = append(queue, adj[cur]...)
	}

	return false, nil
}

func (s *Service) fetchByIDs(ids []string) ([]Record, error) {
	var records []Record
	for _, id := range ids {
		r, err := s.repo.GetByID(id)
		if err != nil {
			return nil, err
		}
		if r != nil {
			records = append(records, *r)
		}
	}
	return records, nil
}

func defaultTaskIDGenerator(prefix string) IDGenerator {
	return func() string {
		timestamp := time.Now().UnixNano()
		sequence := defaultIDSequence.Add(1)
		return prefix + "-" + strconv.FormatInt(timestamp, 10) + "-" + strconv.FormatInt(sequence, 10)
	}
}
