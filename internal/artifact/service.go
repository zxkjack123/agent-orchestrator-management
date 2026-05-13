package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/session"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/step"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/task"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/worktree"
)

// Event is one canonical AOM task timeline event.
type Event struct {
	Type        string
	Actor       string
	StepID      string
	SessionID   string
	Summary     string
	StateEffect string
}

// SyncParams describes the task state needed to write continuity artifacts.
type SyncParams struct {
	Task                  task.Record
	Steps                 []step.Record
	ActiveSession         *session.Record
	Worktree              *worktree.Record
	CreatedBy             string
	UpdatedBy             string
	RecommendedNextAction string
}

// Service writes task-local operational memory artifacts.
type Service struct {
	repoPath   string
	stateDir   string
	now        func() time.Time
	eventIDGen func() string
}

// NewService creates an artifact service for one project root.
func NewService(repoPath, stateDir string) *Service {
	return &Service{
		repoPath:   repoPath,
		stateDir:   stateDir,
		now:        time.Now,
		eventIDGen: defaultEventIDGenerator,
	}
}

// SeedTaskArtifacts creates the initial task artifact set and appends seed events.
func (s *Service) SeedTaskArtifacts(params SyncParams) error {
	if err := s.writeArtifacts(params); err != nil {
		return err
	}

	if err := s.appendEvent(params, Event{
		Type:        "task.created",
		Actor:       defaultActor(params.CreatedBy),
		Summary:     fmt.Sprintf("Task created in %s mode", params.Task.Mode),
		StateEffect: fmt.Sprintf("Task %s", params.Task.Status),
	}); err != nil {
		return err
	}

	for _, item := range params.Steps {
		if err := s.appendEvent(params, Event{
			Type:        "step.proposed",
			Actor:       "aom",
			StepID:      item.ID,
			Summary:     fmt.Sprintf("Step seeded: %s", item.Title),
			StateEffect: fmt.Sprintf("Step %s", item.Status),
		}); err != nil {
			return err
		}
	}

	return nil
}

// RefreshTaskArtifacts rewrites the current task state artifacts without changing the timeline.
func (s *Service) RefreshTaskArtifacts(params SyncParams) error {
	return s.writeArtifacts(params)
}

// AppendEvent records one canonical task log event.
func (s *Service) AppendEvent(params SyncParams, event Event) error {
	return s.appendEvent(params, event)
}

// AppendEvents records multiple canonical task log events in order.
func (s *Service) AppendEvents(params SyncParams, events []Event) error {
	for _, event := range events {
		if err := s.appendEvent(params, event); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) writeArtifacts(params SyncParams) error {
	if strings.TrimSpace(params.Task.ID) == "" {
		return fmt.Errorf("task id is required")
	}
	if strings.TrimSpace(params.Task.Title) == "" {
		return fmt.Errorf("task title is required")
	}

	dir := s.taskDir(params)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create artifact dir %q: %w", dir, err)
	}

	files := map[string]string{
		"task.md":  s.renderTaskMarkdown(params),
		"state.md": s.renderStateMarkdown(params),
		"index.md": s.renderIndexMarkdown(params),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	if err := s.ensureLogFile(params); err != nil {
		return err
	}

	modeArtifacts := s.modeArtifacts(params)
	if err := s.removeUnusedModeArtifacts(dir, modeArtifacts); err != nil {
		return err
	}
	for name, content := range modeArtifacts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	return nil
}

func (s *Service) renderTaskMarkdown(params SyncParams) string {
	artifactRoot := fmt.Sprintf(".aom/%s/%s", s.stateDir, params.Task.ID)
	worktreePath := "not provisioned yet"
	worktreeBranch := "-"
	if params.Worktree != nil {
		worktreePath = params.Worktree.WorktreePath
		worktreeBranch = params.Worktree.BranchName
		if usesWorktreeArtifactRoot(params.Worktree) {
			artifactRoot = filepath.Join(params.Worktree.WorktreePath, ".agent")
		}
	}
	return fmt.Sprintf(`# Task

## Identity
- Task ID: %s
- Title: %s
- Task Mode: %s
- Status: %s
- Created By: %s
- Assigned Role: %s
- Assigned Agent: %s
- Worktree: %s
- Worktree Branch: %s
- Artifact Root: %s

## Goal
%s

## Scope
- Complete the steps tracked for this task

## Out of Scope
- Worktree isolation and provider-native runtime integration remain outside this slice

## Constraints
- Keep continuity state in AOM artifacts
- Follow the current task and step workflow state machine

## Success Criteria
- Planned steps are completed or explicitly resolved
- Task status reflects the final operator decision
- Relevant verification is captured before closure
`,
		params.Task.ID,
		params.Task.Title,
		params.Task.Mode,
		params.Task.Status,
		defaultActor(params.CreatedBy),
		emptyFallback(params.Task.PreferredRole),
		emptyFallback(params.Task.PreferredAgent),
		worktreePath,
		worktreeBranch,
		artifactRoot,
		params.Task.Title,
	)
}

func (s *Service) renderStateMarkdown(params SyncParams) string {
	activeStep := selectActiveStep(params.Steps)
	currentSessionID := "-"
	currentRuntime := "-"
	if params.ActiveSession != nil {
		currentSessionID = params.ActiveSession.ID
		currentRuntime = params.ActiveSession.Runtime
	}
	currentStep := "-"
	stepStatus := "-"
	if activeStep != nil {
		currentStep = fmt.Sprintf("%s %s", activeStep.ID, activeStep.Title)
		stepStatus = activeStep.Status
	}

	return fmt.Sprintf(`# Current State

## Status
- Task Status: %s
- Step Status: %s

## Ownership
- Current Owner: %s
- Current Runtime: %s
- Current Session: %s
- Current Step: %s

## Goal
%s

## Completed Work
%s

## Remaining Work
%s

## Touched Files
- None recorded yet

## Constraints
- Stay within the current task scope
- Preserve continuity through markdown artifacts

## Open Questions
- None recorded yet

## Next Action
%s

## Last Updated By
- Actor: %s
- Session: %s
`,
		params.Task.Status,
		stepStatus,
		currentOwner(params.Task, activeStep),
		currentRuntime,
		currentSessionID,
		currentStep,
		params.Task.Title,
		renderCompletedWork(params.Steps),
		renderRemainingWork(params.Steps),
		emptyFallback(params.RecommendedNextAction),
		defaultActor(params.UpdatedBy),
		currentSessionID,
	)
}

func (s *Service) renderIndexMarkdown(params SyncParams) string {
	activeStep := selectActiveStep(params.Steps)
	activeStepLine := "-"
	if activeStep != nil {
		activeStepLine = fmt.Sprintf("%s %s", activeStep.ID, activeStep.Title)
	}

	activeSession := "-"
	runtime := "-"
	recoveryStatus := "No active session"
	worktreeStatus := "NotProvisioned"
	if params.ActiveSession != nil {
		activeSession = params.ActiveSession.ID
		runtime = params.ActiveSession.Runtime
		recoveryStatus = "Live"
	}
	if params.Worktree != nil {
		worktreeStatus = params.Worktree.Status
	}
	checkpointID, checkpointAt := s.latestCheckpointInfo(params)

	return fmt.Sprintf(`# Task Index

## Identity
- Task ID: %s
- Title: %s
- Mode: %s
- Status: %s

## Active Control
- Active Step: %s
- Assigned Role: %s
- Assigned Agent: %s
- Active Session: %s
- Runtime: %s
- Worktree Status: %s
- Continuity Readiness: Good

## Artifacts
%s

## Checkpoint
- Latest Checkpoint: %s
- Last Checkpoint At: %s

## Attention
- Unresolved Review Items: 0
- Pending Approvals: 0
- Session Recovery Status: %s

## Recommended Next Action
%s
`,
		params.Task.ID,
		params.Task.Title,
		params.Task.Mode,
		params.Task.Status,
		activeStepLine,
		emptyFallback(params.Task.PreferredRole),
		emptyFallback(params.Task.PreferredAgent),
		activeSession,
		runtime,
		worktreeStatus,
		s.renderArtifactInventory(params),
		checkpointID,
		checkpointAt,
		recoveryStatus,
		emptyFallback(params.RecommendedNextAction),
	)
}

func (s *Service) renderArtifactInventory(params SyncParams) string {
	dir := s.taskDir(params)
	lines := []string{
		"- task.md: present",
		"- state.md: present",
		"- log.md: present",
		fmt.Sprintf("- handoff.md: %s", s.artifactPresence(dir, "handoff.md")),
		fmt.Sprintf("- review-notes.md: %s", s.artifactPresence(dir, "review-notes.md")),
	}

	switch params.Task.Mode {
	case "Requirements-first":
		lines = append(lines, "- requirements.md: present", "- design.md: n/a", "- tasks.md: present")
	case "Design-first":
		lines = append(lines, "- requirements.md: n/a", "- design.md: present", "- tasks.md: present")
	default:
		lines = append(lines, "- requirements.md: n/a", "- design.md: n/a", "- tasks.md: n/a")
	}

	return strings.Join(lines, "\n")
}

func (s *Service) artifactPresence(dir, name string) string {
	if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
		return "present"
	}
	return "absent"
}

func (s *Service) latestCheckpointInfo(params SyncParams) (string, string) {
	data, err := os.ReadFile(filepath.Join(s.taskDir(params), "log.md"))
	if err != nil {
		return "-", "-"
	}

	lines := strings.Split(string(data), "\n")
	latestID := "-"
	latestAt := "-"
	currentTimestamp := ""
	inCheckpointEvent := false
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "### "):
			inCheckpointEvent = strings.Contains(line, "| checkpoint.created")
			if !inCheckpointEvent {
				continue
			}
			parts := strings.SplitN(strings.TrimPrefix(line, "### "), " | ", 2)
			if len(parts) > 0 {
				currentTimestamp = strings.TrimSpace(parts[0])
			}
		case inCheckpointEvent && strings.Contains(line, "Checkpoint CHK-"):
			if idx := strings.Index(line, "CHK-"); idx >= 0 {
				checkpointID := strings.TrimSpace(line[idx:])
				if space := strings.IndexAny(checkpointID, " )"); space >= 0 {
					checkpointID = checkpointID[:space]
				}
				latestID = checkpointID
				if currentTimestamp != "" {
					latestAt = currentTimestamp
				}
			}
			inCheckpointEvent = false
		}
	}

	return latestID, latestAt
}

func (s *Service) modeArtifacts(params SyncParams) map[string]string {
	switch params.Task.Mode {
	case "Requirements-first":
		return map[string]string{
			"requirements.md": "# Requirements\n\n## Summary\n- Capture the accepted requirements for this task.\n",
			"tasks.md":        s.renderTasksMarkdown(params.Steps),
		}
	case "Design-first":
		return map[string]string{
			"design.md": "# Design\n\n## Summary\n- Capture the accepted design constraints for this task.\n",
			"tasks.md":  s.renderTasksMarkdown(params.Steps),
		}
	default:
		return nil
	}
}

func (s *Service) renderTasksMarkdown(steps []step.Record) string {
	var builder strings.Builder
	builder.WriteString("# Planned Tasks\n\n## Steps\n")
	if len(steps) == 0 {
		builder.WriteString("- No planned steps yet\n")
		return builder.String()
	}

	for _, item := range steps {
		builder.WriteString(fmt.Sprintf("- %s | %s | %s\n", item.ID, item.StepType, item.Title))
	}
	return builder.String()
}

func (s *Service) removeUnusedModeArtifacts(dir string, keep map[string]string) error {
	candidates := []string{"requirements.md", "design.md", "tasks.md"}
	for _, name := range candidates {
		if _, ok := keep[name]; ok {
			continue
		}
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	return nil
}

func (s *Service) ensureLogFile(params SyncParams) error {
	dir := s.taskDir(params)
	path := filepath.Join(dir, "log.md")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat log.md: %w", err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create artifact dir %q: %w", dir, err)
	}
	if err := os.WriteFile(path, []byte("# Task Log\n\n## Events\n"), 0o644); err != nil {
		return fmt.Errorf("write log.md: %w", err)
	}
	return nil
}

func (s *Service) appendEvent(params SyncParams, event Event) error {
	if err := s.ensureLogFile(params); err != nil {
		return err
	}

	taskID := params.Task.ID
	timestamp := s.now().Format(time.RFC3339)
	entry := fmt.Sprintf(
		"\n### %s | %s | %s\n- Actor: %s\n- Task: %s\n%s%s- Summary: %s\n- State Effect: %s\n",
		timestamp,
		s.eventIDGen(),
		event.Type,
		defaultActor(event.Actor),
		taskID,
		optionalLine("Step", event.StepID),
		optionalLine("Session", event.SessionID),
		event.Summary,
		event.StateEffect,
	)

	path := filepath.Join(s.taskDir(params), "log.md")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log.md for append: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(entry); err != nil {
		return fmt.Errorf("append log.md: %w", err)
	}

	return nil
}

func (s *Service) taskDir(params SyncParams) string {
	if usesWorktreeArtifactRoot(params.Worktree) {
		return filepath.Join(params.Worktree.WorktreePath, ".agent")
	}
	return filepath.Join(s.repoPath, ".aom", s.stateDir, params.Task.ID)
}

func usesWorktreeArtifactRoot(mapping *worktree.Record) bool {
	if mapping == nil || strings.TrimSpace(mapping.WorktreePath) == "" {
		return false
	}

	return mapping.Status == worktree.StatusReady || mapping.Status == worktree.StatusActive
}

func renderCompletedWork(steps []step.Record) string {
	completed := make([]string, 0)
	for _, item := range steps {
		if item.Status == "Completed" || item.Status == "Skipped" || item.Status == "Canceled" {
			completed = append(completed, fmt.Sprintf("- %s | %s", item.ID, item.Title))
		}
	}
	if len(completed) == 0 {
		return "- None recorded yet"
	}
	return strings.Join(completed, "\n")
}

func renderRemainingWork(steps []step.Record) string {
	remaining := make([]string, 0)
	for _, item := range steps {
		if item.Status == "Completed" || item.Status == "Skipped" || item.Status == "Canceled" {
			continue
		}
		remaining = append(remaining, fmt.Sprintf("- %s | %s | status=%s", item.ID, item.Title, item.Status))
	}
	if len(remaining) == 0 {
		return "- No remaining planned work"
	}
	return strings.Join(remaining, "\n")
}

func selectActiveStep(steps []step.Record) *step.Record {
	for _, status := range []string{"InProgress", "Ready", "Confirmed", "Proposed", "Blocked", "NeedsAttention"} {
		for _, item := range steps {
			if item.Status == status {
				stepCopy := item
				return &stepCopy
			}
		}
	}
	return nil
}

func currentOwner(taskRecord task.Record, activeStep *step.Record) string {
	if activeStep != nil {
		if strings.TrimSpace(activeStep.AgentName) != "" {
			return activeStep.AgentName
		}
		if strings.TrimSpace(activeStep.RoleName) != "" {
			return activeStep.RoleName
		}
	}
	if strings.TrimSpace(taskRecord.PreferredAgent) != "" {
		return taskRecord.PreferredAgent
	}
	if strings.TrimSpace(taskRecord.PreferredRole) != "" {
		return taskRecord.PreferredRole
	}
	return "-"
}

func optionalLine(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return fmt.Sprintf("- %s: %s\n", label, value)
}

func emptyFallback(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func defaultActor(value string) string {
	if strings.TrimSpace(value) == "" {
		return "aom"
	}
	return strings.TrimSpace(value)
}

func defaultEventIDGenerator() string {
	return "EVT-" + strconvFormatInt(time.Now().UnixNano())
}

func strconvFormatInt(value int64) string {
	return fmt.Sprintf("%d", value)
}
