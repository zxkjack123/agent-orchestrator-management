package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/agent"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/artifact"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/project"
	aomruntime "github.com/lattapon-aek/agents-orchestrator-management-private/internal/runtime"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/session"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/step"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/task"
)

type reviewParams struct {
	taskID          string
	agentName       string
	launchMode      aomruntime.LaunchMode
	allowEmptyBranch bool
}

func (r Runner) executeReview(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required (or use 'close <task-id>')")
	}

	if args[0] == "close" {
		return r.executeReviewClose(args[1:])
	}

	params := reviewParams{
		taskID:     strings.TrimSpace(args[0]),
		launchMode: aomruntime.LaunchModePlaceholder,
	}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			params.agentName = strings.TrimSpace(args[i])
		case "--mock":
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModeMock); err != nil {
				return err
			}
		case "--real":
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModeReal); err != nil {
				return err
			}
		case "--allow-empty-branch":
			params.allowEmptyBranch = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, params.taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", params.taskID)
	}

	reviewerAgent, err := r.resolveReviewerAgent(result, params.agentName)
	if err != nil {
		return err
	}

	reviewStep, err := r.ensureReviewStep(result, view, reviewerAgent)
	if err != nil {
		return err
	}
	if err := r.prepareReviewState(result, view.Task.ID, reviewStep.ID); err != nil {
		return err
	}
	view, err = r.loadTaskView(result, params.taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found after review step preparation", params.taskID)
	}

	service := artifact.NewService(result.Project.RepoPath, result.StateDir)
	syncParams := artifact.SyncParams{
		Task:                  view.Task,
		Steps:                 view.Steps,
		Worktree:              view.Worktree,
		CreatedBy:             "operator",
		UpdatedBy:             "operator",
		ReviewOwnerHint:       view.ReviewOwnerHint,
		ReviewOwnerAmbiguous:  view.ReviewOwnerAmbiguous,
		RecommendedNextAction: recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous),
	}
	if err := service.EnsureReviewNotesTemplate(syncParams, reviewerAgent.Name, ""); err != nil {
		return err
	}
	if changed, unresolvedCount, err := r.reconcileReviewFindings(result, view.Task.ID, reviewStep.ID); err != nil {
		return err
	} else if changed {
		view, err = r.loadTaskView(result, params.taskID)
		if err != nil {
			return err
		}
		if view == nil {
			return fmt.Errorf("task %q not found after review findings reconciliation", params.taskID)
		}
		fmt.Fprintf(r.stdout, "Review findings detected: %d\n", unresolvedCount)
	}
	if err := r.syncTaskArtifacts(result, view.Task.ID, artifact.Event{
		Type:        "review.prepared",
		Actor:       "operator",
		StepID:      reviewStep.ID,
		Summary:     fmt.Sprintf("Review context prepared for %s", reviewerAgent.Name),
		StateEffect: "Review Ready",
	}, false); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Review prepared")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", view.Task.ID)
	fmt.Fprintf(r.stdout, "Review step: %s\n", reviewStep.ID)
	fmt.Fprintf(r.stdout, "Reviewer agent: %s\n", reviewerAgent.Name)
	fmt.Fprintf(r.stdout, "Review notes: %s\n", filepath.Join(taskArtifactRoot(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree), "review-notes.md"))
	fmt.Fprintf(r.stdout, "Unresolved review items: %d\n", artifact.CountUnresolvedReviewItems(filepath.Join(taskArtifactRoot(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree), "review-notes.md")))

	if !r.app.Tmux.Availability().Available {
		fmt.Fprintln(r.stdout, "Next action: tmux is unavailable here; inspect review-notes.md and spawn the reviewer later from a supported environment")
		if err := r.refreshTaskArtifacts(result, view.Task.ID, nil); err != nil {
			return err
		}
		return nil
	}

	// Guard: block reviewer spawn if the implementation branch has no commits.
	// Reviewing an empty branch produces misleading or empty reports and wastes
	// the reviewer's context window. Pass --allow-empty-branch to bypass.
	if !params.allowEmptyBranch && view.Worktree != nil {
		branch := view.Worktree.BranchName
		defaultBranch := result.Project.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = "main"
		}
		commitsOut, commitsErr := exec.Command("git", "-C", result.Project.RepoPath,
			"log", "--oneline", defaultBranch+".."+branch, "--").Output()
		if commitsErr == nil && strings.TrimSpace(string(commitsOut)) == "" {
			return fmt.Errorf(
				"reviewer spawn blocked: branch %q has no commits ahead of %q — implementation is not ready for review.\n"+
					"Wait for the builder to commit work, then re-run: aom review %s\n"+
					"Pass --allow-empty-branch to bypass (e.g. reviewing a pure-documentation task)",
				branch, defaultBranch, params.taskID,
			)
		}
	}

	sessionRecord, reused, err := r.resolveReviewSession(result, view.Task.ID, reviewStep.ID, reviewerAgent, params.launchMode)
	if err != nil {
		return err
	}
	if err := r.activateReviewState(result, view.Task.ID, reviewStep.ID); err != nil {
		return err
	}
	view, err = r.loadTaskView(result, params.taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found after activating review state", params.taskID)
	}

	if err := service.EnsureReviewNotesTemplate(artifact.SyncParams{
		Task:                  view.Task,
		Steps:                 view.Steps,
		ActiveSession:         sessionRecord,
		Worktree:              view.Worktree,
		CreatedBy:             "operator",
		UpdatedBy:             "operator",
		ReviewOwnerHint:       view.ReviewOwnerHint,
		ReviewOwnerAmbiguous:  view.ReviewOwnerAmbiguous,
		RecommendedNextAction: recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous),
	}, reviewerAgent.Name, sessionRecord.ID); err != nil {
		return err
	}
	if err := r.refreshTaskArtifacts(result, view.Task.ID, sessionRecord); err != nil {
		return err
	}
	if reused {
		fmt.Fprintf(r.stdout, "Session reused: %s\n", sessionRecord.ID)
	} else {
		fmt.Fprintf(r.stdout, "Session spawned: %s\n", sessionRecord.ID)
	}

	return nil
}

// executeReviewClose marks the active review step Completed, transitions the
// task back to InProgress, and records a review.closed event. If the review
// notes contain an unambiguous owner hint, it is applied to the task.
func (r Runner) executeReviewClose(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}
	taskID := strings.TrimSpace(args[0])

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	// Find the active review step.
	var reviewStep *step.Record
	for i := range view.Steps {
		s := &view.Steps[i]
		if s.StepType == "review" && (s.Status == "InProgress" || s.Status == "Ready" || s.Status == "NeedsAttention") {
			reviewStep = s
			break
		}
	}
	if reviewStep == nil {
		return fmt.Errorf("no active review step found for task %q", taskID)
	}

	// Complete the review step. The step machine requires InProgress before Completed,
	// so advance through InProgress if the step hasn't entered it yet.
	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	if reviewStep.Status == "Ready" || reviewStep.Status == "NeedsAttention" {
		if _, err := stepService.Update(reviewStep.ID, step.UpdateParams{Status: "in-progress"}); err != nil {
			return fmt.Errorf("advance review step to in-progress: %w", err)
		}
	}
	if _, err := stepService.Update(reviewStep.ID, step.UpdateParams{Status: "completed"}); err != nil {
		return fmt.Errorf("complete review step: %w", err)
	}

	// Transition task back to InProgress.
	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	updateParams := task.UpdateParams{Status: "in-progress"}
	if !view.ReviewOwnerAmbiguous && strings.TrimSpace(view.ReviewOwnerHint) != "" {
		updateParams.PreferredAgent = view.ReviewOwnerHint
	}
	if _, err := taskService.Update(taskID, updateParams); err != nil {
		return fmt.Errorf("transition task to in-progress: %w", err)
	}

	// Record the review.closed event in the artifact log.
	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        "review.closed",
		Actor:       "operator",
		StepID:      reviewStep.ID,
		Summary:     fmt.Sprintf("Review step %s closed; task returned to in-progress", reviewStep.ID),
		StateEffect: "Review Closed",
	}, false); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Review closed")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", taskID)
	fmt.Fprintf(r.stdout, "Review step: %s\n", reviewStep.ID)
	fmt.Fprintf(r.stdout, "Task status: in-progress\n")
	if updateParams.PreferredAgent != "" {
		fmt.Fprintf(r.stdout, "Preferred agent: %s\n", updateParams.PreferredAgent)
	}
	return nil
}


func (r Runner) resolveReviewerAgent(result *project.OpenResult, name string) (*agent.Record, error) {
	if result == nil {
		return nil, fmt.Errorf("project context is required")
	}
	if strings.TrimSpace(name) != "" {
		agentRecord, err := findAgent(result.Agents, name)
		if err != nil {
			return nil, err
		}
		if agentRecord.Role != "reviewer" {
			return nil, fmt.Errorf("agent %q does not use reviewer role", name)
		}
		return agentRecord, nil
	}

	for _, item := range result.Agents {
		if item.Enabled && item.Role == "reviewer" {
			agentCopy := item
			return &agentCopy, nil
		}
	}

	return nil, fmt.Errorf("no enabled reviewer agent is configured for this project")
}

func (r Runner) reconcileReviewFindings(result *project.OpenResult, taskID, reviewStepID string) (bool, int, error) {
	if result == nil {
		return false, 0, fmt.Errorf("project context is required")
	}

	reviewNotesPath := filepath.Join(taskArtifactRoot(result.Project.RepoPath, result.StateDir, taskID, nil), "review-notes.md")
	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return false, 0, err
	}
	if view == nil {
		return false, 0, fmt.Errorf("task %q not found", taskID)
	}
	reviewNotesPath = filepath.Join(taskArtifactRoot(result.Project.RepoPath, result.StateDir, taskID, view.Worktree), "review-notes.md")
	unresolvedCount := artifact.CountUnresolvedReviewItems(reviewNotesPath)
	if unresolvedCount == 0 {
		return false, 0, nil
	}
	suggestedOwner := artifact.SuggestedReviewOwner(reviewNotesPath)
	suggestedAgent := resolveRoleHintAgent(result.Agents, suggestedOwner)

	changed := false

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return false, unresolvedCount, err
	}
	defer taskDB.Close()

	taskRecord, err := taskService.Get(taskID)
	if err != nil {
		return false, unresolvedCount, err
	}
	if taskRecord == nil {
		return false, unresolvedCount, fmt.Errorf("task %q not found", taskID)
	}
	if suggestedOwner != "" && (!strings.EqualFold(taskRecord.PreferredRole, suggestedOwner) || !strings.EqualFold(strings.TrimSpace(taskRecord.PreferredAgent), strings.TrimSpace(suggestedAgent))) {
		if _, err := taskService.AssignOwner(taskID, suggestedOwner, suggestedAgent); err != nil {
			return false, unresolvedCount, err
		}
		changed = true
	}
	if taskRecord.Status == "InProgress" {
		if _, err := taskService.Update(taskID, task.UpdateParams{Status: "needs-attention"}); err != nil {
			return false, unresolvedCount, err
		}
		changed = true
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return false, unresolvedCount, err
	}
	defer stepDB.Close()

	stepRecord, err := stepService.Get(reviewStepID)
	if err != nil {
		return false, unresolvedCount, err
	}
	if stepRecord == nil {
		return false, unresolvedCount, fmt.Errorf("step %q not found", reviewStepID)
	}
	if suggestedOwner != "" {
		if followupStep := selectReviewFollowupStep(view.Steps, reviewStepID); followupStep != nil {
			if !strings.EqualFold(followupStep.RoleName, suggestedOwner) || !strings.EqualFold(strings.TrimSpace(followupStep.AgentName), strings.TrimSpace(suggestedAgent)) {
				if _, err := stepService.AssignOwner(followupStep.ID, suggestedOwner, suggestedAgent); err != nil {
					return false, unresolvedCount, err
				}
				changed = true
			}
		}
	}
	if stepRecord.Status == "InProgress" || stepRecord.Status == "Blocked" {
		if _, err := stepService.Update(reviewStepID, step.UpdateParams{Status: "needs-attention"}); err != nil {
			return false, unresolvedCount, err
		}
		changed = true
	}

	if !changed {
		return false, unresolvedCount, nil
	}

	view, err = r.loadTaskView(result, taskID)
	if err != nil {
		return false, unresolvedCount, err
	}
	if view == nil {
		return false, unresolvedCount, fmt.Errorf("task %q not found after review findings reconciliation", taskID)
	}
	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        "review.findings_detected",
		Actor:       "operator",
		StepID:      reviewStepID,
		Summary:     reviewFindingsSummary(unresolvedCount, suggestedOwner, suggestedAgent),
		StateEffect: reviewFindingsStateEffect(suggestedOwner, suggestedAgent),
	}, false); err != nil {
		return false, unresolvedCount, err
	}

	return true, unresolvedCount, nil
}

func reviewFindingsSummary(unresolvedCount int, suggestedOwner, suggestedAgent string) string {
	if strings.TrimSpace(suggestedOwner) != "" {
		return fmt.Sprintf("Review findings detected with %d unresolved item(s); preferred follow-up owner is %s", unresolvedCount, reviewOwnerHintDisplay(buildReviewOwnerHint(suggestedOwner, suggestedAgent), false))
	}
	return fmt.Sprintf("Review findings detected with %d unresolved item(s)", unresolvedCount)
}

func reviewFindingsStateEffect(suggestedOwner, suggestedAgent string) string {
	if strings.TrimSpace(suggestedOwner) != "" {
		return fmt.Sprintf("Task and review step moved to NeedsAttention; preferred owner and follow-up step hint reset to %s", reviewOwnerHintDisplay(buildReviewOwnerHint(suggestedOwner, suggestedAgent), false))
	}
	return "Task and review step moved to NeedsAttention"
}

func selectReviewFollowupStep(steps []step.Record, reviewStepID string) *step.Record {
	for i := len(steps) - 1; i >= 0; i-- {
		item := steps[i]
		if item.ID == reviewStepID || item.StepType == "review" {
			continue
		}
		switch item.Status {
		case "Canceled", "Skipped":
			continue
		default:
			stepCopy := item
			return &stepCopy
		}
	}
	return nil
}

func (r Runner) prepareReviewState(result *project.OpenResult, taskID, stepID string) error {
	if result == nil {
		return fmt.Errorf("project context is required")
	}
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(stepID) == "" {
		return fmt.Errorf("review state requires task and step identifiers")
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	taskRecord, err := taskService.Get(taskID)
	if err != nil {
		return err
	}
	if taskRecord == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if taskRecord.Status == "Planned" {
		if _, err := taskService.Update(taskID, task.UpdateParams{Status: "ready"}); err != nil {
			return err
		}
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	stepRecord, err := stepService.Get(stepID)
	if err != nil {
		return err
	}
	if stepRecord == nil {
		return fmt.Errorf("step %q not found", stepID)
	}
	if stepRecord.Status == "Confirmed" || stepRecord.Status == "Blocked" || stepRecord.Status == "NeedsAttention" {
		if _, err := stepService.Update(stepID, step.UpdateParams{Status: "ready"}); err != nil {
			return err
		}
	}

	return nil
}

func (r Runner) activateReviewState(result *project.OpenResult, taskID, stepID string) error {
	if result == nil {
		return fmt.Errorf("project context is required")
	}
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(stepID) == "" {
		return fmt.Errorf("review activation requires task and step identifiers")
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	taskRecord, err := taskService.Get(taskID)
	if err != nil {
		return err
	}
	if taskRecord == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if taskRecord.Status == "Ready" {
		if _, err := taskService.Update(taskID, task.UpdateParams{Status: "in-progress"}); err != nil {
			return err
		}
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	stepRecord, err := stepService.Get(stepID)
	if err != nil {
		return err
	}
	if stepRecord == nil {
		return fmt.Errorf("step %q not found", stepID)
	}
	if stepRecord.Status == "Ready" {
		if _, err := stepService.Update(stepID, step.UpdateParams{Status: "in-progress"}); err != nil {
			return err
		}
	}

	return nil
}

func (r Runner) ensureReviewStep(result *project.OpenResult, view *taskView, reviewerAgent *agent.Record) (*step.Record, error) {
	if result == nil || view == nil || reviewerAgent == nil {
		return nil, fmt.Errorf("review step context is required")
	}

	if existing := findReusableReviewStep(view.Steps); existing != nil {
		stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
		if err != nil {
			return nil, err
		}
		defer stepDB.Close()

		updated, err := stepService.AssignOwner(existing.ID, reviewerAgent.Role, reviewerAgent.Name)
		if err != nil {
			return nil, err
		}
		if updated.Status == "Confirmed" || updated.Status == "Blocked" || updated.Status == "NeedsAttention" {
			return stepService.Update(updated.ID, step.UpdateParams{Status: "ready"})
		}
		return updated, nil
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer stepDB.Close()

	return stepService.Create(step.CreateParams{
		ProjectID:    view.Task.ProjectID,
		TaskID:       view.Task.ID,
		StepType:     "review",
		Title:        "Review " + view.Task.Title,
		Status:       "ready",
		RoleName:     reviewerAgent.Role,
		AgentName:    reviewerAgent.Name,
		Dependencies: reviewStepDependencies(view.Steps),
	})
}

func (r Runner) resolveReviewSession(
	result *project.OpenResult,
	taskID string,
	stepID string,
	reviewerAgent *agent.Record,
	launchMode aomruntime.LaunchMode,
) (*session.Record, bool, error) {
	if reusable, err := r.findReusableReviewSession(result, taskID, reviewerAgent.Name); err != nil {
		return nil, false, err
	} else if reusable != nil {
		return reusable, true, nil
	}

	record, err := r.executeResolvedSessionSpawn(result, reviewerAgent, sessionSpawnParams{
		agentName:  reviewerAgent.Name,
		taskID:     taskID,
		stepID:     stepID,
		launchMode: launchMode,
	})
	if err != nil {
		return nil, false, err
	}
	return record, false, nil
}

func (r Runner) findReusableReviewSession(result *project.OpenResult, taskID, agentName string) (*session.Record, error) {
	if result == nil {
		return nil, fmt.Errorf("project context is required")
	}
	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return nil, err
	}
	for _, item := range sessions {
		if item.TaskID != taskID || item.AgentName != agentName {
			continue
		}
		if strings.TrimSpace(item.TmuxSessionName) == "" || strings.TrimSpace(item.TmuxPane) == "" {
			continue
		}
		switch item.Status {
		case "Idle", "WaitingHandoff":
			recordCopy := item
			return &recordCopy, nil
		}
	}
	return nil, nil
}

func findReusableReviewStep(steps []step.Record) *step.Record {
	for i := len(steps) - 1; i >= 0; i-- {
		item := steps[i]
		if item.StepType != "review" {
			continue
		}
		switch item.Status {
		case "Ready", "InProgress", "Confirmed", "Blocked", "NeedsAttention":
			stepCopy := item
			return &stepCopy
		}
	}
	return nil
}

func reviewStepDependencies(steps []step.Record) []string {
	for i := len(steps) - 1; i >= 0; i-- {
		item := steps[i]
		if item.StepType == "review" {
			continue
		}
		switch item.Status {
		case "Completed", "InProgress", "Ready", "Confirmed", "Blocked", "NeedsAttention":
			return []string{item.ID}
		}
	}
	return nil
}

