package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/artifact"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/step"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/task"
)

func (r Runner) executeTaskCreate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task title is required")
	}

	params := taskCreateParams{title: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--mode":
			i++
			if i >= len(args) {
				return fmt.Errorf("--mode requires a value")
			}
			params.mode = args[i]
		case "--role":
			i++
			if i >= len(args) {
				return fmt.Errorf("--role requires a value")
			}
			params.role = args[i]
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			params.agent = args[i]
		case "--priority":
			i++
			if i >= len(args) {
				return fmt.Errorf("--priority requires a value")
			}
			params.priority = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	if err := r.validateTaskProvisioning(result); err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	priority, err := task.NormalizePriority(params.priority)
	if err != nil {
		return err
	}

	createResult, err := taskService.Create(task.CreateParams{
		ProjectID:      result.Project.ID,
		Title:          params.title,
		Mode:           params.mode,
		Priority:       priority,
		PreferredRole:  params.role,
		PreferredAgent: params.agent,
	})
	if err != nil {
		return err
	}

	if _, err := r.ensurePlannedWorktree(result, createResult.Task); err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, createResult.Task.ID, artifact.Event{
		Type:        "task.created",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task created in %s mode", createResult.Task.Mode),
		StateEffect: fmt.Sprintf("Task %s", createResult.Task.Status),
	}, true); err != nil {
		return err
	}

	_ = r.refreshProjectBoard(result)

	recommendedNext := "confirm the initial step and move the task to Ready"
	if createResult.Task.PreferredRole != "" || createResult.Task.PreferredAgent != "" {
		recommendedNext = "confirm the initial step owner and move the task to Ready"
	}

	fmt.Fprintln(r.stdout, "Task created")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", createResult.Task.ID)
	fmt.Fprintf(r.stdout, "Title: %s\n", createResult.Task.Title)
	fmt.Fprintf(r.stdout, "Mode: %s\n", createResult.Task.Mode)
	fmt.Fprintf(r.stdout, "Status: %s\n", createResult.Task.Status)
	fmt.Fprintf(r.stdout, "Initial steps: %d\n", len(createResult.Steps))
	fmt.Fprintf(r.stdout, "Recommended next step: %s\n", recommendedNext)

	return nil
}

type taskCreateParams struct {
	title    string
	mode     string
	role     string
	agent    string
	priority string
}

func (r Runner) executeTaskShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("task show does not accept extra positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", strings.TrimSpace(args[0]))
	}

	fmt.Fprintln(r.stdout, "Task")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "ID: %s\n", view.Task.ID)
	fmt.Fprintf(r.stdout, "Title: %s\n", view.Task.Title)
	fmt.Fprintf(r.stdout, "Mode: %s\n", view.Task.Mode)
	fmt.Fprintf(r.stdout, "Status: %s\n", view.Task.Status)
	fmt.Fprintf(r.stdout, "Preferred role: %s\n", emptyFallback(view.Task.PreferredRole))
	fmt.Fprintf(r.stdout, "Preferred agent: %s\n", emptyFallback(view.Task.PreferredAgent))
	if view.Worktree != nil {
		fmt.Fprintf(r.stdout, "Worktree status: %s\n", view.Worktree.Status)
		fmt.Fprintf(r.stdout, "Worktree branch: %s\n", view.Worktree.BranchName)
		fmt.Fprintf(r.stdout, "Worktree path: %s\n", view.Worktree.WorktreePath)
		if hint := worktreeHint(view.Task.ID, view.Worktree, view.WorktreeDrift); hint != "" {
			fmt.Fprintf(r.stdout, "Worktree hint: %s\n", hint)
		}
	}
	fmt.Fprintf(r.stdout, "Artifact root: %s\n", taskArtifactRoot(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree))
	fmt.Fprintf(r.stdout, "Task log: %s\n", taskArtifactLogPath(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree))
	fmt.Fprintf(r.stdout, "Unresolved review items: %d\n", view.UnresolvedReviewItems)
	fmt.Fprintf(r.stdout, "Review owner hint: %s\n", reviewOwnerHintDisplay(view.ReviewOwnerHint, view.ReviewOwnerAmbiguous))
	fmt.Fprintf(r.stdout, "Steps: %d\n", len(view.Steps))
	fmt.Fprintf(r.stdout, "Recommended next action: %s\n", recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous))

	return nil
}

func (r Runner) executeTaskUpdate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}

	params := taskUpdateParams{id: strings.TrimSpace(args[0])}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--mode":
			i++
			if i >= len(args) {
				return fmt.Errorf("--mode requires a value")
			}
			params.mode = args[i]
		case "--status":
			i++
			if i >= len(args) {
				return fmt.Errorf("--status requires a value")
			}
			params.status = args[i]
		case "--role":
			i++
			if i >= len(args) {
				return fmt.Errorf("--role requires a value")
			}
			params.role = args[i]
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			params.agent = args[i]
		case "--priority":
			i++
			if i >= len(args) {
				return fmt.Errorf("--priority requires a value")
			}
			params.priority = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	record, err := taskService.Update(params.id, task.UpdateParams{
		Mode:           params.mode,
		Status:         params.status,
		Priority:       params.priority,
		PreferredRole:  params.role,
		PreferredAgent: params.agent,
	})
	if err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, record.ID, artifact.Event{
		Type:        mapTaskEventType(params.status, params.mode),
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task updated to mode=%s status=%s", record.Mode, record.Status),
		StateEffect: fmt.Sprintf("Task %s", record.Status),
	}, false); err != nil {
		return err
	}

	_ = r.refreshProjectBoard(result)

	fmt.Fprintln(r.stdout, "Task updated")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", record.ID)
	fmt.Fprintf(r.stdout, "Mode: %s\n", record.Mode)
	fmt.Fprintf(r.stdout, "Status: %s\n", record.Status)
	fmt.Fprintf(r.stdout, "Priority: %s\n", task.PriorityLabel(record.Priority))
	fmt.Fprintf(r.stdout, "Preferred role: %s\n", emptyFallback(record.PreferredRole))
	fmt.Fprintf(r.stdout, "Preferred agent: %s\n", emptyFallback(record.PreferredAgent))

	return nil
}

type taskUpdateParams struct {
	id       string
	mode     string
	status   string
	role     string
	agent    string
	priority string
}

func (r Runner) executeTaskClose(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("task close does not accept extra positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	taskID := strings.TrimSpace(args[0])

	// Check current task status before attempting close so we can give an actionable error.
	current, err := taskService.Get(taskID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if current.Status != "InProgress" && current.Status != "NeedsAttention" {
		hint := fmt.Sprintf("run: aom task update %s --status in-progress", taskID)
		return fmt.Errorf("task close requires status InProgress or NeedsAttention (current: %s)\n  hint: %s", current.Status, hint)
	}

	// Warn about incomplete steps before closing so the operator can make an informed choice.
	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	steps, err := stepService.ListByTask(taskID)
	if err != nil {
		return err
	}
	autoSkippedSteps, err := autoSkipPlaceholderIntegrationSteps(stepService, steps)
	if err != nil {
		return err
	}
	if len(autoSkippedSteps) > 0 {
		fmt.Fprintf(r.stdout, "Auto-skipped %d placeholder integration step(s):\n", len(autoSkippedSteps))
		for _, s := range autoSkippedSteps {
			fmt.Fprintf(r.stdout, "  - %s (%s)\n", s.ID, s.Status)
		}
		fmt.Fprintln(r.stdout)

		steps, err = stepService.ListByTask(taskID)
		if err != nil {
			return err
		}
	}
	var incompleteSteps []string
	for _, s := range steps {
		if s.Status != "Completed" && s.Status != "Skipped" && s.Status != "Canceled" {
			incompleteSteps = append(incompleteSteps, fmt.Sprintf("%s (%s)", s.ID, s.Status))
		}
	}
	if len(incompleteSteps) > 0 {
		fmt.Fprintf(r.stdout, "Warning: %d step(s) are not yet in a terminal state:\n", len(incompleteSteps))
		for _, s := range incompleteSteps {
			fmt.Fprintf(r.stdout, "  - %s\n", s)
		}
		fmt.Fprintf(r.stdout, "  (closing task anyway; use 'aom step update <step-id> --status completed' to tidy up)\n\n")
	}

	// Warn if the task worktree has uncommitted changes or no commits ahead of the default branch.
	view, viewErr := r.loadTaskView(result, taskID)
	if viewErr == nil && view != nil && view.Worktree != nil {
		wtPath := view.Worktree.WorktreePath
		branch := view.Worktree.BranchName
		defaultBranch := result.Project.DefaultBranch

		// Uncommitted changes (tracked files modified or staged).
		statusOut, statusErr := exec.Command("git", "-C", wtPath, "status", "--porcelain").Output()
		if statusErr == nil && strings.TrimSpace(string(statusOut)) != "" {
			fmt.Fprintf(r.stdout, "Warning: worktree has uncommitted changes — run 'git commit' in %s before merging.\n\n", wtPath)
		}

		// No commits on the task branch yet.
		commitsOut, commitsErr := exec.Command("git", "-C", result.Project.RepoPath,
			"log", "--oneline", defaultBranch+".."+branch).Output()
		if commitsErr == nil && strings.TrimSpace(string(commitsOut)) == "" {
			fmt.Fprintf(r.stdout, "Warning: branch %q has no commits ahead of %q — agent work will not appear in git history after merge.\n\n", branch, defaultBranch)
		}
	}

	record, err := taskService.Close(taskID)
	if err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, record.ID, artifact.Event{
		Type:        "task.closed",
		Actor:       "operator",
		Summary:     "Task closed explicitly by operator",
		StateEffect: fmt.Sprintf("Task %s", record.Status),
	}, false); err != nil {
		return err
	}

	// Emit task.unblocked events for any dependents whose blockers are all Done.
	dependents, depErr := taskService.Unblocks(taskID)
	if depErr == nil {
		for _, dep := range dependents {
			blockers, bErr := taskService.BlockedBy(dep.ID)
			if bErr != nil {
				continue
			}
			allDone := true
			for _, b := range blockers {
				if b.Status != "Done" && b.Status != "Archived" {
					allDone = false
					break
				}
			}
			if allDone {
				_ = r.syncTaskArtifacts(result, dep.ID, artifact.Event{
					Type:        "task.unblocked",
					Actor:       "aom",
					Summary:     fmt.Sprintf("All blockers resolved — %s is now unblocked", dep.ID),
					StateEffect: "unblocked",
				}, false)
				_ = appendChannelMessage(result.Project.RepoPath, "aom",
					fmt.Sprintf("%s (%s) is now unblocked — all dependencies are done", dep.ID, dep.Title),
					time.Now())
				fmt.Fprintf(r.stdout, "Unblocked: %s (%s)\n", dep.ID, dep.Title)
			}
		}
	}

	_ = r.refreshProjectBoard(result)

	fmt.Fprintln(r.stdout, "Task closed")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", record.ID)
	fmt.Fprintf(r.stdout, "Status: %s\n", record.Status)

	return nil
}

// executeTaskAccept accepts a completed agent task in one shot:
// walks all non-terminal steps to Completed, then transitions the task to Done.
func (r Runner) executeTaskAccept(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("task accept takes exactly one argument")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	taskID := strings.TrimSpace(args[0])

	current, err := taskService.Get(taskID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if current.Status == "Done" || current.Status == "Archived" {
		return fmt.Errorf("task %q is already %s", taskID, current.Status)
	}

	// Walk all non-terminal steps to Completed.
	steps, err := stepService.ListByTask(taskID)
	if err != nil {
		return err
	}
	terminalStatuses := map[string]bool{"Completed": true, "Skipped": true, "Canceled": true}
	var completedStepIDs []string
	for _, s := range steps {
		if terminalStatuses[s.Status] {
			continue
		}
		path := stepWalkPath(s.Status, "Completed")
		for _, status := range path {
			if _, err := stepService.Update(s.ID, step.UpdateParams{Status: status}); err != nil {
				return fmt.Errorf("step %s: %w", s.ID, err)
			}
		}
		completedStepIDs = append(completedStepIDs, s.ID)
	}

	// Walk task state to Done through required intermediates.
	taskWalk := taskWalkToDone(current.Status)
	var taskRecord *task.Record
	for _, status := range taskWalk {
		taskRecord, err = taskService.Update(taskID, task.UpdateParams{Status: status})
		if err != nil {
			return fmt.Errorf("task transition to %s: %w", status, err)
		}
	}
	if taskRecord == nil {
		taskRecord = current
	}

	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        "task.closed",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task accepted: %d step(s) completed, task closed", len(completedStepIDs)),
		StateEffect: fmt.Sprintf("Task %s", taskRecord.Status),
	}, false); err != nil {
		return err
	}

	_ = r.refreshProjectBoard(result)

	fmt.Fprintln(r.stdout, "Task accepted")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", taskRecord.ID)
	fmt.Fprintf(r.stdout, "Status: %s\n", taskRecord.Status)
	if len(completedStepIDs) > 0 {
		fmt.Fprintf(r.stdout, "Steps completed: %d\n", len(completedStepIDs))
	}

	return nil
}

// executeTaskReady transitions a Planned task to Ready in one shot:
// confirms all Proposed steps, then moves the task to Ready.
func (r Runner) executeTaskReady(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("task ready takes exactly one argument")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	taskID := strings.TrimSpace(args[0])

	current, err := taskService.Get(taskID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if current.Status == "Ready" {
		fmt.Fprintf(r.stdout, "Task %s is already Ready\n", taskID)
		return nil
	}
	if current.Status != "Planned" {
		return fmt.Errorf("task %q is %s; task ready only works on Planned tasks", taskID, current.Status)
	}

	// Confirm all Proposed steps.
	steps, err := stepService.ListByTask(taskID)
	if err != nil {
		return err
	}
	var confirmedCount int
	for _, s := range steps {
		if s.Status != "Proposed" {
			continue
		}
		if _, err := stepService.Update(s.ID, step.UpdateParams{Status: "Confirmed"}); err != nil {
			return fmt.Errorf("confirm step %s: %w", s.ID, err)
		}
		confirmedCount++
	}

	// Transition task Planned → Ready.
	taskRecord, err := taskService.Update(taskID, task.UpdateParams{Status: "Ready"})
	if err != nil {
		return fmt.Errorf("task transition to Ready: %w", err)
	}

	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        "task.readied",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task readied: %d step(s) confirmed", confirmedCount),
		StateEffect: "Task Ready",
	}, false); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Task ready")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task:   %s\n", taskRecord.ID)
	fmt.Fprintf(r.stdout, "Status: %s\n", taskRecord.Status)
	if confirmedCount > 0 {
		fmt.Fprintf(r.stdout, "Steps confirmed: %d\n", confirmedCount)
	}
	fmt.Fprintf(r.stdout, "\nNext: aom session spawn --task %s --agent <name>\n", taskID)

	return nil
}

// taskWalkToDone returns the ordered status transitions needed to reach Done
// from the given current status, inclusive of Done itself.
func taskWalkToDone(current string) []string {
	linearPath := []string{"Planned", "Ready", "InProgress", "Done"}
	ci := -1
	for i, s := range linearPath {
		if s == current {
			ci = i
			break
		}
	}
	if ci >= 0 && ci < len(linearPath)-1 {
		return linearPath[ci+1:]
	}
	// NeedsAttention/Blocked can go directly to Done.
	return []string{"Done"}
}

func (r Runner) executeTaskReanalyze(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
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

	nextAction := recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous)

	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        "reanalysis.completed",
		Actor:       "aom",
		Summary:     fmt.Sprintf("Artifacts re-synchronized from current system state; recommended next action: %s", nextAction),
		StateEffect: fmt.Sprintf("Task %s", view.Task.Status),
	}, false); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Task re-analyzed")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", view.Task.ID)
	fmt.Fprintf(r.stdout, "Title: %s\n", view.Task.Title)
	fmt.Fprintf(r.stdout, "Status: %s\n", view.Task.Status)
	if view.Worktree != nil {
		if view.WorktreeDrift != "" {
			fmt.Fprintf(r.stdout, "Worktree: %s (%s)\n", view.Worktree.Status, view.WorktreeDrift)
		} else {
			fmt.Fprintf(r.stdout, "Worktree: %s\n", view.Worktree.Status)
		}
	}
	if view.UnresolvedReviewItems > 0 {
		fmt.Fprintf(r.stdout, "Unresolved review items: %d\n", view.UnresolvedReviewItems)
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Recommended next action: %s\n", nextAction)
	return nil
}

func (r Runner) executeTaskLink(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	dependentID := strings.TrimSpace(args[0])
	var blockingID string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--blocked-by", "--depends-on":
			i++
			if i >= len(args) {
				return fmt.Errorf("--blocked-by requires a value")
			}
			blockingID = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q (use --blocked-by <blocker-task-id>)", args[i])
		}
	}

	if blockingID == "" {
		return fmt.Errorf("--blocked-by <task-id> is required (alias: --depends-on)")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if err := taskService.AddDependency(dependentID, blockingID); err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, dependentID, artifact.Event{
		Type:        "task.linked",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task %s is now blocked by %s", dependentID, blockingID),
		StateEffect: "dependency added",
	}, false); err != nil {
		return err
	}

	_ = r.refreshProjectBoard(result)

	fmt.Fprintf(r.stdout, "Linked: %s is blocked by %s\n", dependentID, blockingID)
	return nil
}

func (r Runner) executeTaskUnlink(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	dependentID := strings.TrimSpace(args[0])
	var blockingID string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--blocked-by":
			i++
			if i >= len(args) {
				return fmt.Errorf("--blocked-by requires a value")
			}
			blockingID = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q (use --blocked-by <blocker-task-id>)", args[i])
		}
	}

	if blockingID == "" {
		return fmt.Errorf("--blocked-by <task-id> is required")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if err := taskService.RemoveDependency(dependentID, blockingID); err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, dependentID, artifact.Event{
		Type:        "task.unlinked",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task %s is no longer blocked by %s", dependentID, blockingID),
		StateEffect: "dependency removed",
	}, false); err != nil {
		return err
	}

	_ = r.refreshProjectBoard(result)

	fmt.Fprintf(r.stdout, "Unlinked: %s is no longer blocked by %s\n", dependentID, blockingID)
	return nil
}

func (r Runner) executeTaskRequest(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task title is required")
	}

	// Support optional task-id prefix: aom task request [<task-id>] "<title>"
	// If args[0] looks like a TASK-ID and args[1] exists, treat args[1] as title.
	title := strings.TrimSpace(args[0])
	startIdx := 1
	if strings.HasPrefix(title, "TASK-") && len(args) > 1 && !strings.HasPrefix(args[1], "--") {
		title = strings.TrimSpace(args[1])
		startIdx = 2
	}

	var fromSession, priorityFlag, agentFlag string

	for i := startIdx; i < len(args); i++ {
		switch args[i] {
		case "--from-session":
			i++
			if i >= len(args) {
				return fmt.Errorf("--from-session requires a value")
			}
			fromSession = strings.TrimSpace(args[i])
		case "--priority":
			i++
			if i >= len(args) {
				return fmt.Errorf("--priority requires a value")
			}
			priorityFlag = strings.TrimSpace(args[i])
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			agentFlag = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if priorityFlag == "" {
		priorityFlag = "normal"
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	reqID := generateRequestID(time.Now())

	requestedBy := "operator"
	if fromSession != "" {
		requestedBy = fromSession
	} else if agentFlag != "" {
		requestedBy = agentFlag
	}

	rec := RequestRecord{
		ID:          reqID,
		Title:       title,
		RequestedBy: requestedBy,
		Priority:    priorityFlag,
		Status:      "pending",
	}

	if err := writeRequestArtifact(result.Project.RepoPath, rec); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Request filed\n\n")
	fmt.Fprintf(r.stdout, "Request: %s\n", reqID)
	fmt.Fprintf(r.stdout, "Title:   %s\n", title)
	fmt.Fprintf(r.stdout, "Status:  pending\n")
	fmt.Fprintf(r.stdout, "Next:    aom task list-requests\n")
	return nil
}

func (r Runner) executeTaskListRequests(args []string) error {
	_ = args

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	records, err := readPendingRequests(result.Project.RepoPath)
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Pending task requests")
	fmt.Fprintln(r.stdout, "")

	if len(records) == 0 {
		fmt.Fprintln(r.stdout, "No pending requests.")
		return nil
	}

	for _, rec := range records {
		fmt.Fprintf(r.stdout, "  %s  [%s]  %s  from=%s\n",
			rec.ID, rec.Priority, rec.Title, emptyFallback(rec.RequestedBy))
	}
	fmt.Fprintf(r.stdout, "\nApprove: aom task approve-request <id>\n")
	return nil
}

func (r Runner) executeTaskApproveRequest(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("request id is required")
	}

	reqID := strings.TrimSpace(args[0])

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	all, err := readAllRequests(result.Project.RepoPath)
	if err != nil {
		return err
	}

	var rec *RequestRecord
	for i := range all {
		if all[i].ID == reqID {
			rec = &all[i]
			break
		}
	}
	if rec == nil {
		return fmt.Errorf("request %q not found", reqID)
	}
	if rec.Status != "pending" {
		return fmt.Errorf("request %q is already %s", reqID, rec.Status)
	}

	if err := r.validateTaskProvisioning(result); err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	priority, err := task.NormalizePriority(rec.Priority)
	if err != nil {
		priority = task.PriorityNormal
	}

	createResult, err := taskService.Create(task.CreateParams{
		ProjectID: result.Project.ID,
		Title:     rec.Title,
		Priority:  priority,
	})
	if err != nil {
		return err
	}

	if _, err := r.ensurePlannedWorktree(result, createResult.Task); err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, createResult.Task.ID, artifact.Event{
		Type:        "task.created",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task created from approved request %s", reqID),
		StateEffect: fmt.Sprintf("Task %s", createResult.Task.Status),
	}, true); err != nil {
		return err
	}

	// Mark request approved.
	rec.Status = "approved"
	if err := writeRequestArtifact(result.Project.RepoPath, *rec); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Request approved — new task created\n\n")
	fmt.Fprintf(r.stdout, "Request: %s\n", reqID)
	fmt.Fprintf(r.stdout, "Task:    %s\n", createResult.Task.ID)
	fmt.Fprintf(r.stdout, "Title:   %s\n", createResult.Task.Title)
	fmt.Fprintf(r.stdout, "\nNext: aom task show %s\n", createResult.Task.ID)
	return nil
}

func (r Runner) executeTaskRejectRequest(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("request id is required")
	}

	reqID := strings.TrimSpace(args[0])
	var reason string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--reason":
			i++
			if i >= len(args) {
				return fmt.Errorf("--reason requires a value")
			}
			reason = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	all, err := readAllRequests(result.Project.RepoPath)
	if err != nil {
		return err
	}

	var rec *RequestRecord
	for i := range all {
		if all[i].ID == reqID {
			rec = &all[i]
			break
		}
	}
	if rec == nil {
		return fmt.Errorf("request %q not found", reqID)
	}
	if rec.Status != "pending" {
		return fmt.Errorf("request %q is already %s", reqID, rec.Status)
	}

	rec.Status = "rejected"
	rec.Reason = reason
	if err := writeRequestArtifact(result.Project.RepoPath, *rec); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Request rejected\n\n")
	fmt.Fprintf(r.stdout, "Request: %s\n", reqID)
	if reason != "" {
		fmt.Fprintf(r.stdout, "Reason:  %s\n", reason)
	}
	return nil
}

func (r Runner) executeTaskRecordResult(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	taskID := strings.TrimSpace(args[0])
	var passed, failed bool
	var summary, ciURL, note string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--passed":
			passed = true
		case "--failed":
			failed = true
		case "--summary":
			i++
			if i >= len(args) {
				return fmt.Errorf("--summary requires a value")
			}
			summary = strings.TrimSpace(args[i])
		case "--url":
			i++
			if i >= len(args) {
				return fmt.Errorf("--url requires a value")
			}
			ciURL = strings.TrimSpace(args[i])
		case "--note":
			i++
			if i >= len(args) {
				return fmt.Errorf("--note requires a value")
			}
			note = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if !passed && !failed {
		return fmt.Errorf("--passed or --failed is required")
	}
	if passed && failed {
		return fmt.Errorf("--passed and --failed are mutually exclusive")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	eventType := "test.passed"
	eventSummary := "CI tests passed"
	if failed {
		eventType = "test.failed"
		eventSummary = "CI tests failed"
	}
	if summary != "" {
		eventSummary = eventSummary + ": " + summary
	}
	if ciURL != "" {
		eventSummary = eventSummary + " (" + ciURL + ")"
	}
	if note != "" {
		eventSummary = eventSummary + " [note: " + note + "]"
	}

	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        eventType,
		Actor:       "ci",
		Summary:     eventSummary,
		StateEffect: eventType,
	}, false); err != nil {
		return err
	}

	// Move task to NeedsAttention on failure.
	if failed {
		taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
		if err != nil {
			return err
		}
		defer sqlDB.Close()

		rec, err := taskService.Get(taskID)
		if err != nil {
			return err
		}
		if rec != nil && (rec.Status == "InProgress" || rec.Status == "Ready") {
			if _, err := taskService.Update(taskID, task.UpdateParams{Status: "NeedsAttention"}); err != nil {
				// Non-fatal: log event already appended.
				fmt.Fprintf(r.stderr, "warning: could not transition task to NeedsAttention: %v\n", err)
			}
		}
	}

	status := "passed"
	if failed {
		status = "failed"
	}
	fmt.Fprintf(r.stdout, "Result recorded: %s — %s\n", status, eventSummary)
	return nil
}

func (r Runner) executeTaskList(args []string) error {
	var statusFilter, format string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--status":
			i++
			if i >= len(args) {
				return fmt.Errorf("--status requires a value")
			}
			statusFilter = strings.ToLower(strings.TrimSpace(args[i]))
		case "--format":
			i++
			if i >= len(args) {
				return fmt.Errorf("--format requires a value (json)")
			}
			format = strings.ToLower(strings.TrimSpace(args[i]))
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	all, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	tasks := all[:0]
	for _, t := range all {
		if statusFilter == "" || strings.ToLower(t.Status) == statusFilter {
			tasks = append(tasks, t)
		}
	}

	if len(tasks) == 0 {
		if statusFilter != "" {
			fmt.Fprintf(r.stdout, "No tasks with status %q.\n", statusFilter)
		} else {
			fmt.Fprintln(r.stdout, "No tasks found.")
		}
		return nil
	}

	if format == "json" {
		type taskJSON struct {
			ID             string   `json:"id"`
			Title          string   `json:"title"`
			Status         string   `json:"status"`
			Priority       string   `json:"priority"`
			PreferredRole  string   `json:"preferred_role,omitempty"`
			PreferredAgent string   `json:"preferred_agent,omitempty"`
			BlockedBy      []string `json:"blocked_by,omitempty"`
		}
		out := make([]taskJSON, 0, len(tasks))
		for _, t := range tasks {
			blockers, _ := taskService.BlockedBy(t.ID)
			var blockerIDs []string
			for _, b := range blockers {
				blockerIDs = append(blockerIDs, b.ID)
			}
			out = append(out, taskJSON{
				ID:             t.ID,
				Title:          t.Title,
				Status:         t.Status,
				Priority:       task.PriorityLabel(t.Priority),
				PreferredRole:  t.PreferredRole,
				PreferredAgent: t.PreferredAgent,
				BlockedBy:      blockerIDs,
			})
		}
		enc := json.NewEncoder(r.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	fmt.Fprintf(r.stdout, "%-20s  %-16s  %-8s  %-14s  %-16s  %s\n",
		"TASK", "STATUS", "PRIORITY", "ROLE", "AGENT", "TITLE")
	fmt.Fprintf(r.stdout, "%s\n", strings.Repeat("-", 100))

	for _, t := range tasks {
		blockerIDs, _ := taskService.BlockedBy(t.ID)
		blockedLabel := ""
		if len(blockerIDs) > 0 {
			ids := make([]string, 0, len(blockerIDs))
			for _, b := range blockerIDs {
				ids = append(ids, b.ID)
			}
			blockedLabel = " [blocked by: " + strings.Join(ids, ",") + "]"
		}
		agent := t.PreferredAgent
		if agent == "" {
			agent = "-"
		}
		role := t.PreferredRole
		if role == "" {
			role = "-"
		}
		fmt.Fprintf(r.stdout, "%-20s  %-16s  %-8s  %-14s  %-16s  %s%s\n",
			t.ID, t.Status, task.PriorityLabel(t.Priority), role, agent, t.Title, blockedLabel)
	}

	return nil
}

func (r Runner) executeTaskClaim(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	taskID := strings.TrimSpace(args[0])
	agentName := os.Getenv("AOM_ACTOR")

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			agentName = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if agentName == "" {
		return fmt.Errorf("agent name is required (--agent <name> or set AOM_ACTOR env var)")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	roleName := ""
	for _, a := range result.Agents {
		if a.Name == agentName {
			roleName = a.Role
			break
		}
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	updated, err := taskService.AssignOwner(taskID, roleName, agentName)
	if err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:    "task.claimed",
		Actor:   agentName,
		Summary: fmt.Sprintf("Task claimed by %s", agentName),
	}, false); err != nil {
		return err
	}

	_ = r.refreshProjectBoard(result)

	fmt.Fprintf(r.stdout, "Task claimed\n\n")
	fmt.Fprintf(r.stdout, "Task:  %s\n", updated.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", agentName)
	if roleName != "" {
		fmt.Fprintf(r.stdout, "Role:  %s\n", roleName)
	}
	return nil
}
