package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/agent"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/artifact"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/plan"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/provider"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/session"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/step"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/task"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/tmux"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/worktree"
	"github.com/mattn/go-isatty"
)

// taskView combines a task record with its associated steps, worktree mapping,
// and computed review state for display and decision logic.
type taskView struct {
	Task                  task.Record
	Steps                 []step.Record
	Worktree              *worktree.Record
	WorktreeDrift         string
	UnresolvedReviewItems int
	ReviewOwnerHint       string
	ReviewOwnerAmbiguous  bool
}

func (r Runner) loadTaskView(result *project.OpenResult, taskID string) (*taskView, error) {
	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer taskDB.Close()

	record, err := taskService.Get(taskID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer stepDB.Close()

	steps, err := stepService.ListByTask(record.ID)
	if err != nil {
		return nil, err
	}

	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer worktreeDB.Close()

	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return nil, err
	}

	mapping, err := worktreeService.Reconcile(record.ID, result.Project.RepoPath, hasActiveTaskSession(sessions, record.ID))
	if err != nil {
		return nil, err
	}
	if mapping == nil {
		mapping, err = worktreeService.GetByTask(record.ID)
		if err != nil {
			return nil, err
		}
	}
	driftKind := worktree.DriftNone
	if mapping != nil {
		driftKind, err = worktreeService.DriftKind(record.ID, result.Project.RepoPath)
		if err != nil {
			return nil, err
		}
	}

	return &taskView{
		Task:          *record,
		Steps:         steps,
		Worktree:      mapping,
		WorktreeDrift: driftKind,
		UnresolvedReviewItems: unresolvedReviewItemsForTask(
			result.Project.RepoPath,
			result.StateDir,
			record.ID,
			mapping,
		),
		ReviewOwnerHint: func() string {
			owner, _ := reviewOwnerHintForTask(result.Project.RepoPath, result.StateDir, record.ID, mapping)
			return buildReviewOwnerHint(owner, resolveRoleHintAgent(result.Agents, owner))
		}(),
		ReviewOwnerAmbiguous: func() bool {
			_, ambiguous := reviewOwnerHintForTask(result.Project.RepoPath, result.StateDir, record.ID, mapping)
			return ambiguous
		}(),
	}, nil
}

func (r Runner) loadTaskViews(result *project.OpenResult, sessions []session.Record) ([]taskView, error) {
	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer taskDB.Close()

	tasks, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return nil, err
	}

	if len(tasks) == 0 {
		return nil, nil
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer stepDB.Close()

	views := make([]taskView, 0, len(tasks))
	for _, item := range tasks {
		steps, err := stepService.ListByTask(item.ID)
		if err != nil {
			return nil, err
		}
		views = append(views, taskView{
			Task:  item,
			Steps: steps,
		})
	}

	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer worktreeDB.Close()

	for i := range views {
		mapping, err := worktreeService.Reconcile(views[i].Task.ID, result.Project.RepoPath, hasActiveTaskSession(sessions, views[i].Task.ID))
		if err != nil {
			return nil, err
		}
		if mapping == nil {
			mapping, err = worktreeService.GetByTask(views[i].Task.ID)
			if err != nil {
				return nil, err
			}
		}
		views[i].Worktree = mapping
		if mapping != nil {
			views[i].WorktreeDrift, err = worktreeService.DriftKind(views[i].Task.ID, result.Project.RepoPath)
			if err != nil {
				return nil, err
			}
		}
		views[i].UnresolvedReviewItems = unresolvedReviewItemsForTask(result.Project.RepoPath, result.StateDir, views[i].Task.ID, views[i].Worktree)
		rawReviewOwnerHint, ambiguous := reviewOwnerHintForTask(result.Project.RepoPath, result.StateDir, views[i].Task.ID, views[i].Worktree)
		views[i].ReviewOwnerHint = buildReviewOwnerHint(rawReviewOwnerHint, resolveRoleHintAgent(result.Agents, rawReviewOwnerHint))
		views[i].ReviewOwnerAmbiguous = ambiguous
	}

	return views, nil
}

func (r Runner) loadTaskByID(result *project.OpenResult, taskID string) (*task.Record, error) {
	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer taskDB.Close()

	return taskService.Get(taskID)
}

func (r Runner) loadStepByID(result *project.OpenResult, stepID string) (*step.Record, error) {
	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer stepDB.Close()

	return stepService.Get(stepID)
}

// loadBlockedBy fetches the tasks that block taskID. Errors are silently ignored
// so that artifact sync never fails due to a dependency lookup problem.
func (r Runner) loadBlockedBy(result *project.OpenResult, taskID string) []task.Record {
	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return nil
	}
	defer sqlDB.Close()

	records, err := taskService.BlockedBy(taskID)
	if err != nil {
		return nil
	}
	return records
}

func (r Runner) loadUnblocks(result *project.OpenResult, taskID string) []task.Record {
	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return nil
	}
	defer sqlDB.Close()

	records, err := taskService.Unblocks(taskID)
	if err != nil {
		return nil
	}
	return records
}

func (r Runner) loadProjectSessions(result *project.OpenResult) ([]session.Record, error) {
	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return nil, err
	}

	reconciled := make([]session.Record, 0, len(sessions))
	for _, item := range sessions {
		record, err := r.reconcileSessionRecord(sessionService, item)
		if err != nil {
			return nil, err
		}
		reconciled = append(reconciled, *record)
	}

	return reconciled, nil
}

func (r Runner) loadSessionByIdentifier(identifier string) (*session.Record, error) {
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return nil, err
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	record, err := sessionService.Get(identifier)
	if err != nil {
		return nil, err
	}
	if record != nil {
		return r.reconcileSessionRecord(sessionService, *record)
	}

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return nil, err
	}

	// When matching by agent name, prefer the most recently created session.
	// ListByProject returns sessions in ascending creation order, so we scan
	// all of them and keep the last match.  This matters for workspace agents
	// that accumulate multiple sessions (e.g. one per task): the operator
	// running "aom session resume <agent>" from the workspace should land on
	// the newest session, not an old dead one.
	var best *session.Record
	for i := range sessions {
		if sessions[i].AgentName == identifier {
			cp := sessions[i]
			best = &cp
		}
	}
	if best != nil {
		return r.reconcileSessionRecord(sessionService, *best)
	}

	return nil, fmt.Errorf("session %q not found", identifier)
}

func (r Runner) reconcileSessionRecord(sessionService *session.Service, record session.Record) (*session.Record, error) {
	if sessionService == nil {
		return nil, fmt.Errorf("session service is required")
	}

	availability := r.app.Tmux.Availability()
	if !availability.Available {
		return &record, nil
	}

	paneExists, err := r.app.Tmux.PaneExists(record.TmuxPane)
	if err != nil {
		return nil, err
	}

	updated, err := sessionService.ReconcileBinding(record, paneExists)
	if err != nil {
		return nil, err
	}

	// When a session's pane just disappeared, kill any lingering agent processes
	// (claude/codex may survive a kill-pane if they ignore HUP or run in a sub-shell).
	if record.Status != "Detached" && updated.Status == "Detached" {
		_ = provider.CleanupSession(updated.ID)
	}

	return updated, nil
}

func (r Runner) loadTaskCount(result *project.OpenResult) (int, error) {
	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return 0, err
	}
	defer sqlDB.Close()

	return taskService.CountByProject(result.Project.ID)
}

func hasActiveTaskSession(sessions []session.Record, taskID string) bool {
	for _, item := range sessions {
		if item.TaskID != taskID {
			continue
		}
		switch item.Status {
		case "Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked":
			return true
		}
	}
	return false
}

func (r Runner) syncTaskArtifacts(result *project.OpenResult, taskID string, event artifact.Event, seed bool) error {
	return r.syncTaskArtifactsWithSessionEvents(result, taskID, seed, nil, event)
}

func (r Runner) syncTaskArtifactsWithSession(result *project.OpenResult, taskID string, event artifact.Event, seed bool, activeSession *session.Record) error {
	return r.syncTaskArtifactsWithSessionEvents(result, taskID, seed, activeSession, event)
}

func (r Runner) syncTaskArtifactsWithSessionEvents(result *project.OpenResult, taskID string, seed bool, activeSession *session.Record, events ...artifact.Event) error {
	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found for artifact sync", taskID)
	}

	service := artifact.NewService(result.Project.RepoPath, result.StateDir)
	updatedBy := "aom"
	if len(events) > 0 {
		updatedBy = events[len(events)-1].Actor
	}
	blockedByRecords := r.loadBlockedBy(result, taskID)
	unblocksRecords := r.loadUnblocks(result, taskID)

	params := artifact.SyncParams{
		Task:                  view.Task,
		Steps:                 view.Steps,
		ActiveSession:         activeSession,
		Worktree:              view.Worktree,
		BlockedBy:             blockedByRecords,
		Unblocks:              unblocksRecords,
		CreatedBy:             "operator",
		UpdatedBy:             updatedBy,
		ReviewOwnerHint:       view.ReviewOwnerHint,
		ReviewOwnerAmbiguous:  view.ReviewOwnerAmbiguous,
		RecommendedNextAction: recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous),
		AgentWorkspacePath:    agentWorkspacePath(result.Agents, view.Task.PreferredAgent),
	}
	if seed {
		return service.SeedTaskArtifacts(params)
	}
	if err := service.RefreshTaskArtifacts(params); err != nil {
		return err
	}
	return service.AppendEvents(params, events)
}

func (r Runner) refreshTaskArtifacts(result *project.OpenResult, taskID string, activeSession *session.Record) error {
	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found for artifact refresh", taskID)
	}

	service := artifact.NewService(result.Project.RepoPath, result.StateDir)
	params := artifact.SyncParams{
		Task:                  view.Task,
		Steps:                 view.Steps,
		ActiveSession:         activeSession,
		Worktree:              view.Worktree,
		CreatedBy:             "operator",
		UpdatedBy:             "aom",
		ReviewOwnerHint:       view.ReviewOwnerHint,
		ReviewOwnerAmbiguous:  view.ReviewOwnerAmbiguous,
		RecommendedNextAction: recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous),
	}
	return service.RefreshTaskArtifacts(params)
}

func (r Runner) ensureTaskHandoffTemplate(result *project.OpenResult, taskID string, activeSession *session.Record) error {
	if activeSession == nil {
		return fmt.Errorf("active session is required for handoff template seeding")
	}

	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found for handoff template seeding", taskID)
	}

	service := artifact.NewService(result.Project.RepoPath, result.StateDir)
	params := artifact.SyncParams{
		Task:                  view.Task,
		Steps:                 view.Steps,
		ActiveSession:         activeSession,
		Worktree:              view.Worktree,
		CreatedBy:             "operator",
		UpdatedBy:             "aom",
		ReviewOwnerHint:       view.ReviewOwnerHint,
		ReviewOwnerAmbiguous:  view.ReviewOwnerAmbiguous,
		RecommendedNextAction: recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous),
	}
	return service.EnsureHandoffTemplate(params, *activeSession)
}

func (r Runner) refreshProjectBoard(result *project.OpenResult) error {
	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	tasks, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	briefTasks := make([]artifact.TeamBriefTask, 0, len(tasks))
	for _, t := range tasks {
		blockers, _ := taskService.BlockedBy(t.ID)
		blockerIDs := make([]string, 0, len(blockers))
		for _, b := range blockers {
			blockerIDs = append(blockerIDs, b.ID)
		}
		briefTasks = append(briefTasks, artifact.TeamBriefTask{
			ID:        t.ID,
			Title:     t.Title,
			Status:    t.Status,
			Priority:  task.PriorityLabel(t.Priority),
			Agent:     t.PreferredAgent,
			BlockedBy: blockerIDs,
		})
	}

	svc := artifact.NewService(result.Project.RepoPath, result.StateDir)
	_, err = svc.WriteProjectBoard(result.Project.Name, briefTasks)
	return err
}

func taskArtifactRoot(repoPath, stateDir, taskID string, mapping *worktree.Record) string {
	if mapping != nil && strings.TrimSpace(mapping.WorktreePath) != "" {
		switch mapping.Status {
		case worktree.StatusReady, worktree.StatusActive:
			return filepath.Join(mapping.WorktreePath, ".agent")
		}
	}

	return filepath.Join(repoPath, ".aom", stateDir, taskID)
}

func taskArtifactLogPath(repoPath, stateDir, taskID string, mapping *worktree.Record) string {
	return filepath.Join(taskArtifactRoot(repoPath, stateDir, taskID, mapping), "log.md")
}

func (r Runner) printProjectSummary(title string, result *project.OpenResult, workspace *tmux.Workspace, sessions []session.Record, taskCount int, taskViews []taskView) {
	terminalAvailability := r.app.Tmux.Availability()
	workspaceName := r.app.Tmux.ProjectSessionName(result.SessionPrefix)

	fmt.Fprintln(r.stdout, title)
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Name: %s\n", result.Project.Name)
	fmt.Fprintf(r.stdout, "Repo: %s\n", result.Project.RepoPath)
	fmt.Fprintf(r.stdout, "Default branch: %s\n", result.Project.DefaultBranch)
	fmt.Fprintf(r.stdout, "DB: %s\n", result.DBPath)
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, sectionLabel("Terminal:", r.stdout))
	fmt.Fprintf(r.stdout, "  Driver: %s\n", result.TerminalDriver)
	fmt.Fprintf(r.stdout, "  Available: %t\n", terminalAvailability.Available)
	if terminalAvailability.Available {
		fmt.Fprintf(r.stdout, "  Binary: %s\n", terminalAvailability.BinaryPath)
	} else {
		fmt.Fprintln(r.stdout, "  Binary: not found")
	}
	fmt.Fprintf(r.stdout, "  Workspace: %s\n", workspaceName)
	if workspace != nil {
		state := "reused"
		if workspace.Created {
			state = "created"
		}
		fmt.Fprintf(r.stdout, "  Workspace state: %s\n", state)
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, sectionLabel("Agents:", r.stdout))
	for _, agent := range result.Agents {
		modelSuffix := ""
		if agent.Model != "" {
			modelSuffix = " | model=" + agent.Model
		}
		fmt.Fprintf(r.stdout, "  - %s | role=%s | runtime=%s | enabled=%t%s\n", agent.Name, agent.Role, agent.Runtime, agent.Enabled, modelSuffix)
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, sectionLabel("Sessions:", r.stdout))
	if len(sessions) == 0 {
		fmt.Fprintln(r.stdout, "  None")
	} else {
		for _, item := range sessions {
			statusLabel := colorStatus(item.Status, r.stdout)
			// For Idle sessions, surface whether the tmux pane is still live.
			// "Idle (pane live)" means the tmux pane + underlying process are still
			// attached and consuming resources; operator may want to stop it.
			if item.Status == "Idle" && item.TmuxPane != "" {
				if alive, _ := r.app.Tmux.PaneExists(item.TmuxPane); alive {
					statusLabel += colorize(" (pane live)", ansiYellow, r.stdout)
				}
			}
			fmt.Fprintf(
				r.stdout,
				"  - %s | agent=%s | role=%s | runtime=%s | status=%s | tmux=%s %s %s\n",
				item.ID,
				item.AgentName,
				item.RoleName,
				item.Runtime,
				statusLabel,
				item.TmuxSessionName,
				item.TmuxWindow,
				item.TmuxPane,
			)
			if item.Status == "Detached" {
				fmt.Fprintf(r.stdout, "    next=%s\n", detachedSessionHint(item))
			}
			if item.Status == "Idle" && item.TmuxPane != "" {
				if alive, _ := r.app.Tmux.PaneExists(item.TmuxPane); alive {
					fmt.Fprintf(r.stdout, "    attached=yes — process still running; run: aom session stop %s\n", item.ID)
				}
			}
			bgCount := r.app.Tmux.CountDescendants(item.TmuxPane)
			if readiness := sessionReadiness(result.Project.RepoPath, item, bgCount); readiness != "" {
				fmt.Fprintf(r.stdout, "    readiness=%s\n", readiness)
				if readiness == "stuck-retrying" {
					fmt.Fprintf(r.stdout, "    bg-terminals=%d — agent may be retry-looping; run: aom session stop %s\n", bgCount, item.ID)
				}
			}
		}
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, sectionLabel("Tasks:", r.stdout))
	if len(taskViews) == 0 {
		fmt.Fprintln(r.stdout, "  None")
	} else {
		for _, item := range taskViews {
			fmt.Fprintf(
				r.stdout,
				"  - %s | title=%s | mode=%s | status=%s | role=%s | agent=%s | steps=%d\n",
				item.Task.ID,
				item.Task.Title,
				item.Task.Mode,
				colorStatus(item.Task.Status, r.stdout),
				emptyFallback(item.Task.PreferredRole),
				emptyFallback(item.Task.PreferredAgent),
				len(item.Steps),
			)
			if item.UnresolvedReviewItems > 0 {
				fmt.Fprintf(r.stdout, "    reviews=open:%d\n", item.UnresolvedReviewItems)
				fmt.Fprintf(r.stdout, "    review-owner=%s\n", reviewOwnerHintDisplay(item.ReviewOwnerHint, item.ReviewOwnerAmbiguous))
			}
			if item.Worktree != nil {
				fmt.Fprintf(
					r.stdout,
					"    worktree=%s | branch=%s | path=%s\n",
					colorStatus(item.Worktree.Status, r.stdout),
					item.Worktree.BranchName,
					item.Worktree.WorktreePath,
				)
				if hint := worktreeHint(item.Task.ID, item.Worktree, item.WorktreeDrift); hint != "" {
					label := "note"
					if item.Worktree.Status == worktree.StatusNeedsRepair {
						label = "repair"
					}
					fmt.Fprintf(r.stdout, "    %s=%s\n", label, hint)
				}
			}
			fmt.Fprintf(
				r.stdout,
				"    artifacts=%s | log=%s\n",
				taskArtifactRoot(result.Project.RepoPath, result.StateDir, item.Task.ID, item.Worktree),
				taskArtifactLogPath(result.Project.RepoPath, result.StateDir, item.Task.ID, item.Worktree),
			)
			fmt.Fprintf(r.stdout, "    next=%s\n", recommendTaskAction(item.Task.Status, item.Steps, item.Worktree, item.WorktreeDrift, item.UnresolvedReviewItems, item.ReviewOwnerHint, item.ReviewOwnerAmbiguous))
			for _, taskStep := range item.Steps {
				fmt.Fprintf(
					r.stdout,
					"    * %s | type=%s | title=%s | status=%s | role=%s | agent=%s | dependencies=%s\n",
					taskStep.ID,
					taskStep.StepType,
					taskStep.Title,
					colorStatus(taskStep.Status, r.stdout),
					emptyFallback(taskStep.RoleName),
					emptyFallback(taskStep.AgentName),
					formatDependencies(taskStep.Dependencies),
				)
			}
		}
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Counts:")
	fmt.Fprintf(r.stdout, "  Tasks: %d\n", taskCount)
	fmt.Fprintf(r.stdout, "  Sessions: %d\n", len(sessions))

	if pending := countPendingOutboxMessages(result.Project.RepoPath); pending > 0 {
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, colorize("  ! %d outbox message(s) pending — run: aom outbox flush\n", ansiYellow, r.stdout), pending)
	}
}

func recommendTaskAction(status string, steps []step.Record, mapping *worktree.Record, driftKind string, unresolvedReviewItems int, reviewOwnerHint string, reviewOwnerAmbiguous bool) string {
	if mapping != nil && mapping.Status == worktree.StatusNeedsRepair {
		switch driftKind {
		case worktree.DriftMissingPath:
			return "recreate the missing task worktree before continuing"
		case worktree.DriftUnregisteredArtifactOnlyPath:
			return "run worktree repair to recreate the unregistered task worktree"
		case worktree.DriftUnregisteredDirtyPath:
			return "inspect the existing task worktree path and clean it up manually before continuing"
		default:
			return "repair the task worktree before continuing"
		}
	}
	if status == "Done" {
		return "task is closed; archive later if needed"
	}
	if status == "NeedsAttention" {
		return "operator review is needed before work continues"
	}
	if unresolvedReviewItems > 0 {
		if reviewOwnerAmbiguous {
			return "review findings have mixed owners; operator must choose the follow-up owner before continuing"
		}
		if strings.TrimSpace(reviewOwnerHint) != "" {
			return fmt.Sprintf("address unresolved review items and route follow-up work to %s", reviewOwnerHint)
		}
		return "address unresolved review items before continuing implementation"
	}
	for _, item := range steps {
		if item.Status == "NeedsAttention" {
			return "resolve the step that needs operator attention"
		}
	}
	for _, item := range steps {
		if item.Status == "Blocked" {
			return "unblock the blocked step or move it to NeedsAttention"
		}
	}
	if status == "Planned" && len(steps) > 0 && steps[0].Status == "Proposed" {
		return "confirm the proposed step and move the task to Ready"
	}
	for _, item := range steps {
		if item.Status == "Ready" {
			return fmt.Sprintf("start step %s", item.ID)
		}
		if item.Status == "InProgress" {
			return fmt.Sprintf("continue step %s", item.ID)
		}
	}

	return "inspect steps and choose the next operator action"
}

func unresolvedReviewItemsForTask(repoPath, stateDir, taskID string, mapping *worktree.Record) int {
	return artifact.CountUnresolvedReviewItems(filepath.Join(taskArtifactRoot(repoPath, stateDir, taskID, mapping), "review-notes.md"))
}

func reviewOwnerHintForTask(repoPath, stateDir, taskID string, mapping *worktree.Record) (string, bool) {
	return artifact.ReviewOwnerHint(filepath.Join(taskArtifactRoot(repoPath, stateDir, taskID, mapping), "review-notes.md"))
}

func reviewOwnerHintDisplay(owner string, ambiguous bool) string {
	if ambiguous {
		return "mixed owners - operator must choose"
	}
	if strings.TrimSpace(owner) == "" {
		return "-"
	}
	return owner
}

func buildReviewOwnerHint(roleName, agentName string) string {
	roleName = strings.TrimSpace(roleName)
	agentName = strings.TrimSpace(agentName)
	if roleName == "" {
		return ""
	}
	if agentName == "" {
		return roleName
	}
	return fmt.Sprintf("%s (%s)", agentName, roleName)
}

func resolveRoleHintAgent(agents []agent.Record, roleName string) string {
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return ""
	}

	matches := make([]string, 0, 1)
	for _, item := range agents {
		if !item.Enabled || !strings.EqualFold(item.Role, roleName) {
			continue
		}
		matches = append(matches, item.Name)
		if len(matches) > 1 {
			return ""
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

func mapTaskEventType(status, mode string) string {
	if strings.TrimSpace(mode) != "" {
		return "task.mode_changed"
	}
	if strings.EqualFold(strings.TrimSpace(status), "done") {
		return "task.closed"
	}
	return "task.updated"
}

func emptyFallback(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}

	return value
}

// interpretEscapes converts literal escape sequences in s to their real byte
// equivalents (\n → newline, \t → tab). This lets callers embed multi-line
// content in a single shell argument without requiring ANSI-C quoting.
func interpretEscapes(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")
	return s
}

// briefSummary returns a single-line summary of a brief for use in log events.
// Multi-line briefs are truncated to their first non-empty line so that the
// log scanner never mistakes embedded log-format templates for real events.
// isShellProcess returns true when the given process name indicates a bare shell
// rather than an AI runtime. Used by session send to warn operators before
// injecting a brief into a pane where no agent is running.
func isShellProcess(cmd string) bool {
	switch strings.ToLower(strings.TrimSpace(cmd)) {
	case "bash", "sh", "zsh", "fish", "dash", "ksh", "tcsh", "csh":
		return true
	}
	return false
}

func briefSummary(message string) string {
	const maxLen = 200
	for _, line := range strings.Split(message, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			if len(trimmed) > maxLen {
				return trimmed[:maxLen] + "..."
			}
			return trimmed
		}
	}
	return strings.TrimSpace(message)
}

func formatDependencies(values []string) string {
	if len(values) == 0 {
		return "-"
	}

	return strings.Join(values, ",")
}

func parseCommaSeparatedValues(value string) []string {
	parts := strings.Split(value, ",")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func isTTYReader(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(file.Fd()) || isatty.IsCygwinTerminal(file.Fd())
}

func buildPlanStepSeeds(steps []plan.StepProposal) []task.StepSeed {
	if len(steps) == 0 {
		return nil
	}

	seeds := make([]task.StepSeed, 0, len(steps))
	for _, item := range steps {
		seed := task.StepSeed{
			Type:      item.Type,
			Title:     item.Title,
			RoleName:  item.RoleName,
			AgentName: item.AgentName,
		}
		seeds = append(seeds, seed)
	}
	return seeds
}

func findAgent(agents []agent.Record, name string) (*agent.Record, error) {
	for _, item := range agents {
		if item.Name != name {
			continue
		}
		if !item.Enabled {
			return nil, fmt.Errorf("agent %q is disabled", name)
		}

		agentCopy := item
		return &agentCopy, nil
	}

	return nil, fmt.Errorf("agent %q not found", name)
}

func findAgentByName(agents []agent.Record, name string) *agent.Record {
	for i := range agents {
		if agents[i].Name == name {
			return &agents[i]
		}
	}
	return nil
}

// agentWorkspacePath returns the WorkspacePath for the named agent, or ""
// if the agent has no workspace or the name is blank.  Used to populate
// SyncParams.AgentWorkspacePath so artifact rendering can adjust paths.
func agentWorkspacePath(agents []agent.Record, agentName string) string {
	if strings.TrimSpace(agentName) == "" {
		return ""
	}
	for _, ag := range agents {
		if ag.Name == agentName {
			return strings.TrimSpace(ag.WorkspacePath)
		}
	}
	return ""
}

func activeStepID(steps []step.Record) string {
	for _, item := range steps {
		switch item.Status {
		case "InProgress", "Ready", "Confirmed", "Blocked", "NeedsAttention", "Proposed":
			return item.ID
		}
	}
	return ""
}

func activeStepTitle(steps []step.Record) string {
	for _, item := range steps {
		switch item.Status {
		case "InProgress", "Ready", "Confirmed", "Blocked", "NeedsAttention", "Proposed":
			return item.Title
		}
	}
	return ""
}

// lastChannelMessages returns the last n message blocks from channel.md.
func lastChannelMessages(repoPath string, n int) []string {
	content, err := readChannelFile(repoPath)
	if err != nil || content == "" {
		return nil
	}

	// Split on "### " to get individual message blocks.
	parts := strings.Split(content, "\n### ")
	if len(parts) <= 1 {
		return nil
	}

	// Drop the header part (before first ###).
	msgs := parts[1:]
	if len(msgs) > n {
		msgs = msgs[len(msgs)-n:]
	}

	result := make([]string, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, "### "+strings.TrimSpace(m))
	}
	return result
}
