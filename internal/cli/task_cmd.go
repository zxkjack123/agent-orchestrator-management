package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/artifact"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/step"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/task"
)

// handoffTemplateSentinels is the set of placeholder strings that AOM seeds into
// new handoff.md files. Any handoff.md that still contains one of these strings
// is considered unfilled and will fail the verify check or block promotion.
var handoffTemplateSentinels = []string{
	"Fill this in when the work is ready for transfer",
	"Fill in what was completed in this session",
	"Fill in what still needs to happen next",
	"Record touched files before signaling",
}

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
		case "--step-type":
			i++
			if i >= len(args) {
				return fmt.Errorf("--step-type requires a value")
			}
			params.stepType = args[i]
		case "--description":
			i++
			if i >= len(args) {
				return fmt.Errorf("--description requires a value")
			}
			params.description = args[i]
		case "--invariant":
			i++
			if i >= len(args) {
				return fmt.Errorf("--invariant requires a value")
			}
			params.invariants = append(params.invariants, args[i])
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
		ProjectID:       result.Project.ID,
		Title:           params.title,
		Description:     params.description,
		Mode:            params.mode,
		Priority:        priority,
		PreferredRole:   params.role,
		PreferredAgent:  params.agent,
		InitialStepType: params.stepType,
	})
	if err != nil {
		return err
	}

	// Skip per-task worktree provisioning when the assigned agent has a permanent
	// workspace. Workspace agents never need to cd to a new worktree — their tasks
	// land as artifacts inside workspace/.agent/tasks/<taskID>/.
	assignedAgent := findAgentByName(result.Agents, params.agent)
	agentHasWorkspace := assignedAgent != nil && strings.TrimSpace(assignedAgent.WorkspacePath) != ""
	if !agentHasWorkspace {
		if _, err := r.ensurePlannedWorktree(result, createResult.Task); err != nil {
			return err
		}
	}

	if len(params.invariants) > 0 {
		invPath := invariantsPath(result.Project.RepoPath, result.StateDir, createResult.Task.ID)
		if err := os.MkdirAll(filepath.Dir(invPath), 0o755); err != nil {
			return fmt.Errorf("create invariants dir: %w", err)
		}
		if err := os.WriteFile(invPath, []byte(strings.Join(params.invariants, "\n")+"\n"), 0o644); err != nil {
			return fmt.Errorf("write invariants: %w", err)
		}
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
	if createResult.Task.Description != "" {
		fmt.Fprintf(r.stdout, "Description: %s\n", createResult.Task.Description)
	}
	fmt.Fprintf(r.stdout, "Mode: %s\n", createResult.Task.Mode)
	fmt.Fprintf(r.stdout, "Status: %s\n", createResult.Task.Status)
	fmt.Fprintf(r.stdout, "Initial steps: %d\n", len(createResult.Steps))
	fmt.Fprintf(r.stdout, "Recommended next step: %s\n", recommendedNext)

	return nil
}

type taskCreateParams struct {
	title       string
	description string
	mode        string
	role        string
	agent       string
	priority    string
	stepType    string
	invariants  []string
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
	if transitions := task.ValidTransitions(view.Task.Status); len(transitions) > 0 {
		fmt.Fprintf(r.stdout, "Valid transitions: -> %s\n", strings.Join(transitions, ", "))
	}
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
	if invs := loadTaskInvariants(result, view.Task.ID); len(invs) > 0 {
		fmt.Fprintf(r.stdout, "Invariants: %s\n", strings.Join(invs, ", "))
	}

	// Commit guard: warn if agent signalled task.completed but branch has no commits.
	if view.Task.Status != "Done" && view.Task.Status != "Archived" {
		if view.Worktree != nil {
			logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree)
			if hasTaskCompletedEvent(logPath) {
				branch := view.Worktree.BranchName
				defaultBranch := result.Project.DefaultBranch
				commitsOut, commitsErr := exec.Command("git", "-C", result.Project.RepoPath,
					"log", "--oneline", defaultBranch+".."+branch).Output()
				if commitsErr == nil && strings.TrimSpace(string(commitsOut)) == "" {
					fmt.Fprintf(r.stdout, "\n⚠  task.completed logged but no commits found on branch %q ahead of %q\n", branch, defaultBranch)
					fmt.Fprintf(r.stdout, "   Agent may not have committed work. Run: aom task verify %s\n", view.Task.ID)
				}
			}
		} else if agentRec := findAgentByName(result.Agents, view.Task.PreferredAgent); agentRec != nil &&
			strings.TrimSpace(agentRec.WorkspacePath) != "" &&
			strings.TrimSpace(view.Task.PreferredAgent) != "" {
			// Workspace agent: check workspace log and agents/<name> branch.
			wsLogPath := filepath.Join(strings.TrimSpace(agentRec.WorkspacePath), ".agent", "log.md")
			if hasTaskCompletedEvent(wsLogPath) {
				branch := "agents/" + strings.TrimSpace(view.Task.PreferredAgent)
				taskID := view.Task.ID
				defaultBranch := result.Project.DefaultBranch
				// Check any commits exist on the branch.
				commitsOut, commitsErr := exec.Command("git", "-C", result.Project.RepoPath,
					"log", "--oneline", defaultBranch+".."+branch).Output()
				if commitsErr == nil && strings.TrimSpace(string(commitsOut)) == "" {
					fmt.Fprintf(r.stdout, "\n⚠  task.completed logged but no commits found on branch %q ahead of %q\n", branch, defaultBranch)
					fmt.Fprintf(r.stdout, "   Agent may not have committed work. Run: aom task verify %s\n", taskID)
				} else {
					// Commits exist — check for [TASK-xxx] tag specifically.
					taggedOut, taggedErr := exec.Command("git", "-C", result.Project.RepoPath,
						"log", "--oneline", "--fixed-strings", "--grep=["+taskID+"]",
						defaultBranch+".."+branch).Output()
					if taggedErr == nil && strings.TrimSpace(string(taggedOut)) == "" {
						fmt.Fprintf(r.stdout, "\n⚠  task.completed logged but no commits tagged [%s] on branch %q\n", taskID, branch)
						fmt.Fprintf(r.stdout, "   Commits exist but will be excluded from merge — prefix commits with [%s]\n", taskID)
						fmt.Fprintf(r.stdout, "   Run: aom task verify %s\n", taskID)
					}
				}
			}
		}
	}

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
		walkPath := taskWalkToInProgress(current.Status)
		if len(walkPath) == 0 {
			return fmt.Errorf("task %q (status: %s) cannot be auto-advanced to InProgress for close", taskID, current.Status)
		}
		for _, status := range walkPath {
			if _, err := taskService.Update(taskID, task.UpdateParams{Status: status}); err != nil {
				return fmt.Errorf("auto-advance to InProgress: %w", err)
			}
		}
		fmt.Fprintf(r.stderr, "Note: auto-advanced task from %s → InProgress before close\n", current.Status)
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
	} else if viewErr == nil && view != nil && view.Worktree == nil {
		// Workspace agent: no per-task worktree — check the agents/<name> branch instead.
		agentName := strings.TrimSpace(view.Task.PreferredAgent)
		if agentRecord := findAgentByName(result.Agents, agentName); agentRecord != nil &&
			strings.TrimSpace(agentRecord.WorkspacePath) != "" && agentName != "" {

			wsPath := strings.TrimSpace(agentRecord.WorkspacePath)
			branch := "agents/" + agentName
			defaultBranch := result.Project.DefaultBranch

			// Uncommitted changes in the workspace.
			statusOut, statusErr := exec.Command("git", "-C", wsPath, "status", "--porcelain").Output()
			if statusErr == nil && strings.TrimSpace(string(statusOut)) != "" {
				fmt.Fprintf(r.stdout, "Warning: workspace has uncommitted changes — run 'git commit' in %s before merging.\n\n", wsPath)
			}

			// No commits tagged [TASK-taskID] on the workspace branch yet.
			commitsOut, commitsErr := exec.Command("git", "-C", result.Project.RepoPath,
				"log", "--oneline", "--fixed-strings", "--grep=["+taskID+"]",
				defaultBranch+".."+branch).Output()
			if commitsErr == nil && strings.TrimSpace(string(commitsOut)) == "" {
				fmt.Fprintf(r.stdout, "Warning: no commits tagged [%s] on branch %q — "+
					"agent must prefix commits with [%s] so AOM can identify them at merge time.\n\n",
					taskID, branch, taskID)
			}
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

	go runHook(result.Project.RepoPath, "on-task-done", record.ID, record.Title, record.Status)

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
// Flags: --force   skip completion-readiness checks and accept unconditionally.
func (r Runner) executeTaskAccept(args []string) error {
	var taskID string
	force := false
	autoMode := false
	autoInterval := 15 * time.Second
	autoTimeout := 30 * time.Minute

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force":
			force = true
		case "--auto":
			autoMode = true
		case "--interval":
			i++
			if i >= len(args) {
				return fmt.Errorf("--interval requires a value (e.g. 15s, 1m)")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--interval: %w", err)
			}
			autoInterval = d
		case "--timeout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--timeout requires a value (e.g. 30m, 2h)")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--timeout: %w", err)
			}
			autoTimeout = d
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown flag %q", args[i])
			}
			if taskID != "" {
				return fmt.Errorf("task accept takes exactly one task identifier")
			}
			taskID = strings.TrimSpace(args[i])
		}
	}
	if taskID == "" {
		return fmt.Errorf("task identifier is required")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// --auto mode: poll verify checks every interval until all pass, then accept.
	if autoMode {
		agentHint := taskID
		fmt.Fprintf(r.stdout, "Auto-accept: watching %s — polling every %s, timeout %s\n", taskID, autoInterval, autoTimeout)
		fmt.Fprintln(r.stdout, "Press Ctrl+C to cancel.")
		fmt.Fprintln(r.stdout, "")
		deadline := time.Now().Add(autoTimeout)
		iteration := 1
		for {
			view, viewErr := r.loadTaskView(result, taskID)
			if viewErr == nil && view != nil {
				if strings.TrimSpace(view.Task.PreferredAgent) != "" {
					agentHint = view.Task.PreferredAgent
				}
				checks := r.runTaskVerifyChecks(result, view)
				allOK := true
				fmt.Fprintf(r.stdout, "─── #%d  %s ─────────────────────────────\n", iteration, time.Now().Format("15:04:05"))
				for _, c := range checks {
					icon := "ok"
					if !c.ok {
						icon = "FAIL"
						allOK = false
					}
					if c.note != "" {
						fmt.Fprintf(r.stdout, "  [%s]  %s — %s\n", icon, c.name, c.note)
					} else {
						fmt.Fprintf(r.stdout, "  [%s]  %s\n", icon, c.name)
					}
				}
				if allOK {
					fmt.Fprintln(r.stdout, "\nAll checks passed — proceeding to accept...")
					fmt.Fprintln(r.stdout, "")
					break
				}
			}
			if time.Now().After(deadline) {
				fmt.Fprintf(r.stdout, "\n⚠  Auto-accept timed out after %s\n\n", autoTimeout)
				fmt.Fprintln(r.stdout, "Diagnose:")
				fmt.Fprintf(r.stdout, "  aom capture %s --diff\n", agentHint)
				fmt.Fprintf(r.stdout, "  aom task verify %s\n", taskID)
				fmt.Fprintln(r.stdout, "  aom session recover <session-id>")
				fmt.Fprintln(r.stdout, "")
				fmt.Fprintln(r.stdout, "Accept anyway (bypass checks):")
				fmt.Fprintf(r.stdout, "  aom task accept --force %s\n", taskID)
				return fmt.Errorf("auto-accept: timeout after %s", autoTimeout)
			}
			fmt.Fprintf(r.stdout, "\n  (next check in %s...)\n", autoInterval)
			time.Sleep(autoInterval)
			iteration++
		}
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

	// Verify completion readiness unless --force is given.
	// This prevents accepting tasks where the agent wrote task.completed but
	// forgot to commit, fill handoff.md, or otherwise finish properly.
	if !force {
		view, viewErr := r.loadTaskView(result, taskID)
		if viewErr == nil && view != nil {
			checks := r.runTaskVerifyChecks(result, view)
			var failed []verifyCheck
			for _, c := range checks {
				if !c.ok {
					failed = append(failed, c)
				}
			}
			if len(failed) > 0 {
				fmt.Fprintln(r.stdout, "Task accept blocked — completion checks failed:")
				fmt.Fprintln(r.stdout, "")
				for _, c := range failed {
					if c.note != "" {
						fmt.Fprintf(r.stdout, "  [FAIL]  %s — %s\n", c.name, c.note)
					} else {
						fmt.Fprintf(r.stdout, "  [FAIL]  %s\n", c.name)
					}
				}
				fmt.Fprintln(r.stdout, "")
				fmt.Fprintln(r.stdout, "Re-run with --force to accept anyway, or wait for the agent to complete its work.")
				return fmt.Errorf("task accept blocked: %d completion check(s) failed", len(failed))
			}
		}
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

	go runHook(result.Project.RepoPath, "on-task-done", taskRecord.ID, taskRecord.Title, taskRecord.Status)

	_ = r.refreshProjectBoard(result)

	// Auto-stop any Idle sessions bound to this task so that orphaned background
	// terminals (e.g. codex background terminal children) are cleaned up immediately
	// without requiring a separate "aom session stop" command.
	stoppedSessions := r.autoStopIdleSessionsForTask(result, taskID)

	fmt.Fprintln(r.stdout, "Task accepted")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", taskRecord.ID)
	fmt.Fprintf(r.stdout, "Status: %s\n", taskRecord.Status)
	if len(completedStepIDs) > 0 {
		fmt.Fprintf(r.stdout, "Steps completed: %d\n", len(completedStepIDs))
	}
	for _, sid := range stoppedSessions {
		fmt.Fprintf(r.stdout, "Session stopped: %s (background processes cleaned up)\n", sid)
	}

	return nil
}

// autoStopIdleSessionsForTask finds Idle sessions bound to taskID and stops them,
// killing any descendant processes (codex background terminals, caffeinate, etc.).
// Returns the IDs of sessions that were stopped. Errors are best-effort: a cleanup
// failure does not fail the task accept.
func (r Runner) autoStopIdleSessionsForTask(result *project.OpenResult, taskID string) []string {
	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return nil
	}
	defer sqlDB.Close()

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return nil
	}

	var stopped []string
	for _, s := range sessions {
		if s.TaskID != taskID {
			continue
		}
		if s.Status != "Idle" && s.Status != "WaitingHandoff" {
			continue
		}
		if _, _, err := r.stopSessionRecord(result, s, false); err == nil {
			stopped = append(stopped, s.ID)
		}
	}
	return stopped
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

	go runHook(result.Project.RepoPath, "on-task-ready", taskRecord.ID, taskRecord.Title, taskRecord.Status)

	_ = r.refreshProjectBoard(result)

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

// taskWalkToInProgress returns the status transitions needed to reach InProgress
// from the given current status. Returns nil for statuses that cannot advance.
func taskWalkToInProgress(current string) []string {
	switch current {
	case "Planned":
		return []string{"Ready", "InProgress"}
	case "Ready":
		return []string{"InProgress"}
	default:
		return nil
	}
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

	// Load agent record to check for workspace mode.
	agentRecord := findAgentByName(result.Agents, agentName)

	roleName := ""
	if agentRecord != nil {
		roleName = agentRecord.Role
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

	// Workspace mode: write current-task.md into the agent workspace.
	// Worktree provisioning is skipped — the agent works from its permanent workspace.
	if agentRecord != nil && strings.TrimSpace(agentRecord.WorkspacePath) != "" {
		writeCurrentTaskFile(agentRecord.WorkspacePath, updated.ID, updated.Title)
		fmt.Fprintf(r.stdout, "Workspace: %s (agent workspace mode — no worktree provisioned)\n", agentRecord.WorkspacePath)
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

// hasTaskCompletedEvent returns true if log.md contains a task.completed event,
// indicating an agent signalled completion regardless of the DB task status.
func hasTaskCompletedEvent(logPath string) bool {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return false
	}
	s := string(data)
	// Accept both the canonical "task.completed" signal (from aom task signal) and
	// "task.closed" (the event written by aom task close / aom task accept).
	// codex agents often call "aom task close" which produces task.closed; treating
	// it as equivalent avoids a spurious verify failure for that runtime.
	return strings.Contains(s, "task.completed") || strings.Contains(s, "task.closed")
}

// verifyCheck is one completion-readiness result from runTaskVerifyChecks.
type verifyCheck struct {
	name string
	ok   bool
	note string
}

// runTaskVerifyChecks evaluates completion readiness for the given task view:
// commits on branch, state.md updated, handoff.md filled, task.completed in log,
// and any registered invariants. Returns the full list of checks (never nil for a
// task that has a worktree; may be short for workspace agents with no branch).
func (r Runner) runTaskVerifyChecks(result *project.OpenResult, view *taskView) []verifyCheck {
	var checks []verifyCheck

	// Resolve workspace path for the assigned agent (may be empty for traditional agents).
	agentRecord := findAgentByName(result.Agents, view.Task.PreferredAgent)
	workspacePath := ""
	if agentRecord != nil {
		workspacePath = strings.TrimSpace(agentRecord.WorkspacePath)
	}

	// Check 1: commits on branch ahead of default branch.
	// Traditional agents: check per-task worktree branch.
	// Workspace agents:   check agents/<name> permanent branch.
	if view.Worktree != nil {
		branch := view.Worktree.BranchName
		defaultBranch := result.Project.DefaultBranch
		commitsOut, commitsErr := exec.Command("git", "-C", result.Project.RepoPath,
			"log", "--oneline", defaultBranch+".."+branch).Output()
		hasCommits := commitsErr == nil && strings.TrimSpace(string(commitsOut)) != ""
		note := ""
		if !hasCommits {
			note = fmt.Sprintf("no commits on %q ahead of %q", branch, defaultBranch)
		}
		checks = append(checks, verifyCheck{"commits on branch", hasCommits, note})
	} else if workspacePath != "" && strings.TrimSpace(view.Task.PreferredAgent) != "" {
		branch := "agents/" + strings.TrimSpace(view.Task.PreferredAgent)
		taskID := view.Task.ID
		defaultBranch := result.Project.DefaultBranch

		// Check 1a: any commits on the workspace branch at all.
		commitsOut, commitsErr := exec.Command("git", "-C", result.Project.RepoPath,
			"log", "--oneline", defaultBranch+".."+branch).Output()
		hasCommits := commitsErr == nil && strings.TrimSpace(string(commitsOut)) != ""
		note := ""
		if !hasCommits {
			note = fmt.Sprintf("no commits on workspace branch %q ahead of %q", branch, defaultBranch)
		}
		checks = append(checks, verifyCheck{"commits on branch", hasCommits, note})

		// Check 1b: at least one commit is tagged [TASK-xxx] for this task.
		// Workspace agents share one branch across tasks — the tag is how AOM
		// identifies which commits belong to this task at merge time (aom merge commit).
		// A branch with commits but no tagged commits will produce an empty merge set.
		taggedOut, taggedErr := exec.Command("git", "-C", result.Project.RepoPath,
			"log", "--oneline", "--fixed-strings", "--grep=["+taskID+"]",
			defaultBranch+".."+branch).Output()
		hasTagged := taggedErr == nil && strings.TrimSpace(string(taggedOut)) != ""
		tagNote := ""
		if !hasTagged {
			tagNote = fmt.Sprintf(
				"no commit tagged [%s] on branch %q — prefix commits with [%s] so aom merge commit can find them",
				taskID, branch, taskID,
			)
		}
		checks = append(checks, verifyCheck{"[" + taskID + "] tagged commit", hasTagged, tagNote})
	}

	// Check 2: state.md has completed work entries.
	// For workspace agents prefer workspace/.agent/state.md (agent's live copy) over the
	// task-artifact state.md which is only refreshed by AOM CLI calls.
	artifactRoot := taskArtifactRoot(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree)
	statePath := filepath.Join(artifactRoot, "state.md")
	if workspacePath != "" {
		wsStatePath := filepath.Join(workspacePath, ".agent", "state.md")
		if _, err := os.Stat(wsStatePath); err == nil {
			statePath = wsStatePath
		}
	}
	stateData, stateErr := os.ReadFile(statePath)
	stateOK := stateErr == nil && !strings.Contains(string(stateData), "- None recorded yet")
	stateNote := ""
	if !stateOK {
		if stateErr != nil {
			stateNote = "state.md not found"
		} else {
			stateNote = "state.md still shows 'None recorded yet'"
		}
	}
	checks = append(checks, verifyCheck{"state.md updated", stateOK, stateNote})

	// Check 3: handoff.md is present, non-trivial, and not template placeholder text.
	// For workspace agents also accept the workspace copy (.agent/handoff.md inside
	// the worktree) as a fallback when the task artifact still has template text.
	// Agents that write to their workspace .agent/handoff.md produce valid content
	// there; this fallback mirrors the state.md and log.md patterns above.
	handoffDataOK := func(data []byte) bool {
		if len(strings.TrimSpace(string(data))) <= 80 {
			return false
		}
		for _, s := range handoffTemplateSentinels {
			if strings.Contains(string(data), s) {
				return false
			}
		}
		return true
	}
	handoffData, handoffErr := os.ReadFile(filepath.Join(artifactRoot, "handoff.md"))
	handoffOK := handoffErr == nil && handoffDataOK(handoffData)
	handoffNote := ""
	if !handoffOK {
		// Fallback: workspace agents write .agent/handoff.md inside their worktree.
		// Accept that copy when the task artifact is missing or still template text.
		if workspacePath != "" {
			wsHandoffData, wsErr := os.ReadFile(filepath.Join(workspacePath, ".agent", "handoff.md"))
			if wsErr == nil && handoffDataOK(wsHandoffData) {
				handoffOK = true
			}
		}
	}
	if !handoffOK {
		if handoffErr != nil {
			handoffNote = "handoff.md not found"
		} else {
			hasTemplate := false
			for _, s := range handoffTemplateSentinels {
				if strings.Contains(string(handoffData), s) {
					hasTemplate = true
					break
				}
			}
			if hasTemplate {
				handoffNote = "handoff.md still contains template placeholder text — update it before signaling completion"
			} else {
				handoffNote = "handoff.md appears empty or too sparse"
			}
		}
	}
	checks = append(checks, verifyCheck{"handoff.md filled", handoffOK, handoffNote})

	// Check 4: task.completed event in log.md.
	// For workspace agents also accept the event from workspace/.agent/log.md since that
	// is where the agent writes its own completion signal.
	logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree)
	logOK := hasTaskCompletedEvent(logPath)
	if !logOK && workspacePath != "" {
		wsLogPath := filepath.Join(workspacePath, ".agent", "log.md")
		logOK = hasTaskCompletedEvent(wsLogPath)
	}
	logNote := ""
	if !logOK {
		logNote = "task.completed event not found in log.md"
	}
	checks = append(checks, verifyCheck{"task.completed in log", logOK, logNote})

	// Invariant checks.
	// For workspace agents use the agents/<name> branch to scan changed files.
	invariantBranch := ""
	if view.Worktree != nil {
		invariantBranch = view.Worktree.BranchName
	} else if workspacePath != "" && strings.TrimSpace(view.Task.PreferredAgent) != "" {
		invariantBranch = "agents/" + strings.TrimSpace(view.Task.PreferredAgent)
	}
	if invariantBranch != "" {
		branch := invariantBranch
		defaultBranch := result.Project.DefaultBranch
		gitLogOut, gitLogErr := exec.Command("git", "-C", result.Project.RepoPath,
			"log", "--name-only", defaultBranch+".."+branch).Output()
		gitLogFiles := ""
		if gitLogErr == nil {
			gitLogFiles = string(gitLogOut)
		}
		for _, inv := range loadTaskInvariants(result, view.Task.ID) {
			parts := strings.SplitN(inv, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key, val := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			switch key {
			case "required-path":
				found := false
				for _, line := range strings.Split(gitLogFiles, "\n") {
					if strings.HasPrefix(strings.TrimSpace(line), val) {
						found = true
						break
					}
				}
				note := ""
				if !found {
					note = fmt.Sprintf("no changed file with prefix %q found on branch", val)
				}
				checks = append(checks, verifyCheck{"invariant: required-path=" + val, found, note})
			case "forbidden-path":
				violated := false
				violatingFile := ""
				for _, line := range strings.Split(gitLogFiles, "\n") {
					trimmed := strings.TrimSpace(line)
					if trimmed != "" && strings.HasPrefix(trimmed, val) {
						violated = true
						violatingFile = trimmed
						break
					}
				}
				note := ""
				if violated {
					note = fmt.Sprintf("forbidden file found on branch: %q", violatingFile)
				}
				checks = append(checks, verifyCheck{"invariant: forbidden-path=" + val, !violated, note})
			}
		}
	}

	return checks
}

// executeTaskVerify checks that a task's worktree evidence is consistent with completion:
// commits exist on branch, state.md updated, handoff.md filled, task.completed in log.
//
// --watch mode polls until all checks pass (or --timeout expires).
// Usage: aom task verify <task-id> [--watch] [--interval <dur>] [--timeout <dur>]
func (r Runner) executeTaskVerify(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}

	// Parse flags: first positional arg is task ID.
	taskID := strings.TrimSpace(args[0])
	watchMode := false
	interval := 10 * time.Second
	timeout := 30 * time.Minute

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--watch", "-w":
			watchMode = true
		case "--interval":
			i++
			if i >= len(args) {
				return fmt.Errorf("--interval requires a value (e.g. 10s, 30s, 1m)")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--interval: %w", err)
			}
			interval = d
		case "--timeout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--timeout requires a value (e.g. 10m, 1h)")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--timeout: %w", err)
			}
			timeout = d
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// printVerify prints the check table and returns whether all checks passed.
	printVerify := func(ts string) bool {
		view, err := r.loadTaskView(result, taskID)
		if err != nil || view == nil {
			fmt.Fprintf(r.stdout, "task %q not found\n", taskID)
			return false
		}
		checks := r.runTaskVerifyChecks(result, view)
		allOK := true
		if ts != "" {
			fmt.Fprintf(r.stdout, "─── %s ───────────────────────────────────────────\n", ts)
		} else {
			fmt.Fprintln(r.stdout, "Task Verify")
			fmt.Fprintln(r.stdout, "")
		}
		fmt.Fprintf(r.stdout, "Task:   %s\n", view.Task.ID)
		fmt.Fprintf(r.stdout, "Title:  %s\n", view.Task.Title)
		fmt.Fprintf(r.stdout, "Status: %s\n", view.Task.Status)
		fmt.Fprintln(r.stdout, "")
		for _, c := range checks {
			icon := "ok"
			if !c.ok {
				icon = "FAIL"
				allOK = false
			}
			if c.note != "" {
				fmt.Fprintf(r.stdout, "[%s]  %s — %s\n", icon, c.name, c.note)
			} else {
				fmt.Fprintf(r.stdout, "[%s]  %s\n", icon, c.name)
			}
		}
		fmt.Fprintln(r.stdout, "")
		if allOK {
			fmt.Fprintln(r.stdout, "All checks passed — task appears ready for review/accept")
		} else {
			fmt.Fprintln(r.stdout, "Some checks failed — review the items above before accepting this task")
		}
		return allOK
	}

	if !watchMode {
		printVerify("")
		return nil
	}

	// Watch mode: poll until all checks pass or timeout expires.
	fmt.Fprintf(r.stdout, "Watching %s — polling every %s, timeout %s\n\n", taskID, interval, timeout)
	deadline := time.Now().Add(timeout)
	iteration := 1

	for {
		ts := fmt.Sprintf("#%d  %s", iteration, time.Now().Format("15:04:05"))
		allOK := printVerify(ts)
		if allOK {
			fmt.Fprintf(r.stdout, "\nAll checks passed after %d poll(s) — run: aom task accept %s\n", iteration, taskID)
			return nil
		}
		if time.Now().After(deadline) {
			fmt.Fprintf(r.stdout, "\nTimeout after %s — some checks still failing\n", timeout)
			return fmt.Errorf("verify --watch: timeout after %s", timeout)
		}
		fmt.Fprintf(r.stdout, "  (next check in %s...)\n", interval)
		time.Sleep(interval)
		iteration++
	}
}

func (r Runner) executeTaskCancel(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("task cancel takes exactly one argument")
	}

	taskID := strings.TrimSpace(args[0])

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	current, err := taskService.Get(taskID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	switch current.Status {
	case "Draft", "Planned", "Ready":
		// allowed to cancel
	default:
		return fmt.Errorf("task %q is %s; only Draft, Planned, or Ready tasks can be cancelled", taskID, current.Status)
	}

	// Cancel (skip) all non-terminal steps.
	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	steps, err := stepService.ListByTask(taskID)
	if err != nil {
		return err
	}
	var cancelledSteps int
	for _, s := range steps {
		switch s.Status {
		case "Completed", "Skipped", "Canceled":
			continue
		}
		if _, updateErr := stepService.Update(s.ID, step.UpdateParams{Status: "Canceled"}); updateErr != nil {
			return fmt.Errorf("cancel step %s: %w", s.ID, updateErr)
		}
		cancelledSteps++
	}

	taskRecord, err := taskService.Update(taskID, task.UpdateParams{Status: "Archived"})
	if err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        "task.cancelled",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task cancelled from %s; %d step(s) cancelled", current.Status, cancelledSteps),
		StateEffect: "Task Archived",
	}, false); err != nil {
		return err
	}

	_ = r.refreshProjectBoard(result)

	fmt.Fprintln(r.stdout, "Task cancelled")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task:   %s\n", taskRecord.ID)
	fmt.Fprintf(r.stdout, "Status: %s\n", taskRecord.Status)
	if cancelledSteps > 0 {
		fmt.Fprintf(r.stdout, "Steps cancelled: %d\n", cancelledSteps)
	}
	return nil
}

// executeTaskSignal appends a signal event to a task's canonical log so that
// agents can call "aom task signal <type> --task <id>" instead of manually
// writing to .agent/log.md.  For workspace agents the event is also mirrored
// to workspace/.agent/log.md so the agent's own log file stays consistent.
//
// Usage:
//
//	aom task signal <event-type> --task <task-id> [--summary <text>] [--step <step-id>]
//
// Valid event types: task.completed, handoff.prepared, checkpoint.created, step.completed
func (r Runner) executeTaskSignal(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("event type is required — valid types: task.completed, handoff.prepared, checkpoint.created, step.completed")
	}

	validEventTypes := map[string]bool{
		"task.completed":     true,
		"handoff.prepared":   true,
		"checkpoint.created": true,
		"step.completed":     true,
	}
	eventType := strings.TrimSpace(args[0])
	if !validEventTypes[eventType] {
		return fmt.Errorf("unknown event type %q; valid types: task.completed, handoff.prepared, checkpoint.created, step.completed", eventType)
	}

	var taskID, summary, stepID string

	// Default actor from AOM_ACTOR env var (set by aom session spawn in agent profile).
	actor := strings.TrimSpace(os.Getenv("AOM_ACTOR"))
	if actor == "" {
		actor = "agent"
	}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--task":
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			taskID = strings.TrimSpace(args[i])
		case "--summary":
			i++
			if i >= len(args) {
				return fmt.Errorf("--summary requires a value")
			}
			summary = strings.TrimSpace(args[i])
		case "--step":
			i++
			if i >= len(args) {
				return fmt.Errorf("--step requires a value")
			}
			stepID = strings.TrimSpace(args[i])
		case "--actor":
			i++
			if i >= len(args) {
				return fmt.Errorf("--actor requires a value")
			}
			actor = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if taskID == "" {
		return fmt.Errorf("--task <id> is required")
	}
	if summary == "" {
		summary = eventType
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	event := artifact.Event{
		Type:    eventType,
		Actor:   actor,
		StepID:  stepID,
		Summary: summary,
	}

	// Write to the canonical task artifact log (checked by aom task verify).
	if err := r.syncTaskArtifacts(result, taskID, event, false); err != nil {
		return err
	}

	// For workspace agents: mirror log, and on task.completed auto-promote the
	// workspace handoff.md to the task artifact if the artifact still has template text.
	view, viewErr := r.loadTaskView(result, taskID)
	if viewErr == nil && view != nil {
		agentRecord := findAgentByName(result.Agents, view.Task.PreferredAgent)
		if agentRecord != nil && strings.TrimSpace(agentRecord.WorkspacePath) != "" {
			wsPath := strings.TrimSpace(agentRecord.WorkspacePath)

			// Mirror log event to workspace log so the agent's own log stays current.
			wsLogPath := filepath.Join(wsPath, ".agent", "log.md")
			appendSignalToWorkspaceLog(wsLogPath, eventType, actor, taskID, summary)

			// Auto-promote workspace handoff.md → task artifact on task.completed.
			// Agents write .agent/handoff.md in their workspace; the task artifact
			// handoff.md lives at a different path and is what aom task verify reads.
			// When they diverge, copy the workspace copy to the artifact so both are
			// consistent and verify passes without requiring agents to know both paths.
			if eventType == "task.completed" {
				artifactHandoffPath := filepath.Join(result.Project.RepoPath, result.StateDir, "tasks", taskID, "handoff.md")
				promoteWorkspaceHandoff(wsPath, artifactHandoffPath)
			}
		}
	}

	fmt.Fprintf(r.stdout, "Signal recorded\n\n")
	fmt.Fprintf(r.stdout, "Event: %s\n", eventType)
	fmt.Fprintf(r.stdout, "Task:  %s\n", taskID)
	if stepID != "" {
		fmt.Fprintf(r.stdout, "Step:  %s\n", stepID)
	}
	fmt.Fprintf(r.stdout, "Actor: %s\n", actor)
	return nil
}

// appendSignalToWorkspaceLog appends a minimal log entry to a workspace log.md file
// if the file already exists.  Uses the same heading format as the artifact service so
// hasTaskCompletedEvent can detect the signal.  Silently ignores write errors — the
// canonical log (task artifact) is already updated; this is a best-effort mirror.
func appendSignalToWorkspaceLog(logPath, eventType, actor, taskID, summary string) {
	// Only mirror if the workspace log already exists — we don't create it here.
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	entry := fmt.Sprintf(
		"\n### %s | %s\n- Actor: %s\n- Task: %s\n- Summary: %s\n",
		time.Now().UTC().Format(time.RFC3339),
		eventType,
		actor,
		taskID,
		summary,
	)
	_, _ = f.WriteString(entry)
}

// promoteWorkspaceHandoff copies the workspace .agent/handoff.md to the task
// artifact handoff.md when the artifact still contains template placeholder text.
// This is called on task.completed so that agents who write the correct content
// to their workspace handoff.md have it automatically reflected in the task
// artifact that aom task verify reads.  Silently ignores errors — best-effort.
func promoteWorkspaceHandoff(workspacePath, artifactHandoffPath string) {
	// Read workspace handoff.md — if it doesn't exist or is sparse, skip.
	wsData, err := os.ReadFile(filepath.Join(workspacePath, ".agent", "handoff.md"))
	if err != nil || len(strings.TrimSpace(string(wsData))) <= 80 {
		return
	}
	// Skip if workspace copy also has template text.
	for _, s := range handoffTemplateSentinels {
		if strings.Contains(string(wsData), s) {
			return
		}
	}
	// Read current artifact handoff.md — only promote if artifact has template text.
	artifactData, artifactErr := os.ReadFile(artifactHandoffPath)
	if artifactErr == nil {
		hasTemplate := false
		for _, s := range handoffTemplateSentinels {
			if strings.Contains(string(artifactData), s) {
				hasTemplate = true
				break
			}
		}
		if !hasTemplate {
			return // artifact is already good — don't overwrite
		}
	}
	// Copy workspace content to artifact (best-effort).
	_ = os.WriteFile(artifactHandoffPath, wsData, 0o644)
}

// invariantsPath returns the path to the task invariants file.
func invariantsPath(repoPath, stateDir, taskID string) string {
	return filepath.Join(repoPath, ".aom", stateDir, taskID, "invariants.txt")
}

// loadTaskInvariants reads the invariants file for a task. Returns nil if not found.
func loadTaskInvariants(result *project.OpenResult, taskID string) []string {
	data, err := os.ReadFile(invariantsPath(result.Project.RepoPath, result.StateDir, taskID))
	if err != nil {
		return nil
	}
	var invs []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			invs = append(invs, line)
		}
	}
	return invs
}
