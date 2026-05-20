package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/agent"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/artifact"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/config"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/project"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/provider"
	aomruntime "github.com/lattapon-aek/agents-orchestrator-management-private/internal/runtime"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/session"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/step"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/task"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/worktree"
)

func (r Runner) executeSessionSpawn(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent name is required")
	}

	params := sessionSpawnParams{
		agentName:  strings.TrimSpace(args[0]),
		launchMode: aomruntime.LaunchModePlaceholder,
	}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--task":
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			params.taskID = strings.TrimSpace(args[i])
		case "--step":
			i++
			if i >= len(args) {
				return fmt.Errorf("--step requires a value")
			}
			params.stepID = strings.TrimSpace(args[i])
		case "--mock":
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModeMock); err != nil {
				return err
			}
		case "--real":
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModeReal); err != nil {
				return err
			}
		case "--fresh":
			params.freshStart = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	agentRecord, err := findAgent(result.Agents, params.agentName)
	if err != nil {
		return err
	}

	// Warn when a mini model is used for a task with multiple steps.
	if agentRecord.Model != "" && strings.HasSuffix(agentRecord.Model, "-mini") && params.taskID != "" {
		stepService, stepDB, stepErr := r.app.OpenStepService(result.DBPath)
		if stepErr == nil {
			steps, _ := stepService.ListByTask(params.taskID)
			stepDB.Close()
			if len(steps) > 1 {
				fmt.Fprintf(r.stdout, "Warning: model %q (mini) may not handle multi-step tasks reliably (%d steps).\n", agentRecord.Model, len(steps))
				fmt.Fprintln(r.stdout, "         Consider using a larger model: aom agent set-model "+agentRecord.Name+" <model>")
				fmt.Fprintln(r.stdout, "")
			}
		}
	}

	_, err = r.executeResolvedSessionSpawn(result, agentRecord, params)
	return err
}

func (r Runner) executeResolvedSessionSpawn(result *project.OpenResult, agentRecord *agent.Record, params sessionSpawnParams) (*session.Record, error) {
	var err error
	var taskRecord *task.Record
	if params.taskID != "" {
		taskRecord, err = r.loadTaskByID(result, params.taskID)
		if err != nil {
			return nil, err
		}
		if taskRecord == nil {
			return nil, fmt.Errorf("task %q not found", params.taskID)
		}
	}

	executionPath := result.Project.RepoPath
	var taskWorktree *worktree.Record
	if taskRecord != nil {
		taskWorktree, executionPath, err = r.resolveTaskExecutionPath(result, *taskRecord)
		if err != nil {
			return nil, err
		}
	}

	var stepRecord *step.Record
	if params.stepID != "" {
		if taskRecord == nil {
			return nil, fmt.Errorf("--step requires --task in the current milestone")
		}
		stepRecord, err = r.loadStepByID(result, params.stepID)
		if err != nil {
			return nil, err
		}
		if stepRecord == nil {
			return nil, fmt.Errorf("step %q not found", params.stepID)
		}
		if stepRecord.TaskID != taskRecord.ID {
			return nil, fmt.Errorf("step %q does not belong to task %q", stepRecord.ID, taskRecord.ID)
		}
	}

	if err := r.enforceWriterWorktreeBoundary(result, agentRecord, params); err != nil {
		return nil, err
	}

	if _, err := newLaunchBuilder().Build(aomruntime.SessionSpec{
		SessionID:    "",
		AgentName:    agentRecord.Name,
		RoleName:     agentRecord.Role,
		Runtime:      agentRecord.Runtime,
		DenyCommands: result.Policy.Policy.DenyCommands,
		Model:        agentRecord.Model,
	}, params.launchMode); err != nil {
		return nil, err
	}

	workspace, err := r.app.Tmux.EnsureWorkspace(result.SessionPrefix, result.Project.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("ensure tmux workspace: %w", err)
	}

	// Inject AOM_BIN so agents can call `aom` regardless of PATH configuration.
	if selfBinEarly, binErr := os.Executable(); binErr == nil && selfBinEarly != "" {
		_ = r.app.Tmux.SetSessionEnv(workspace.Target, "AOM_BIN", selfBinEarly)
	}
	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	// Guard: refuse if agent already has an active session for the same task to prevent
	// RAM/process pile-up when operators accidentally spawn twice.
	// Skip sessions being replaced (ignoreSessionID) — those are intentional handoffs.
	if params.taskID != "" {
		existing, checkErr := sessionService.ActiveByAgent(result.Project.ID, agentRecord.Name)
		if checkErr != nil {
			return nil, fmt.Errorf("check active sessions: %w", checkErr)
		}
		for _, s := range existing {
			if s.TaskID == params.taskID && s.ID != params.ignoreSessionID {
				return nil, fmt.Errorf(
					"agent %q already has an active session %s (status: %s) for this task\nstop it first:  aom session stop %s",
					agentRecord.Name, s.ID, s.Status, s.ID,
				)
			}
		}

		// Warn if a different agent with the same role already has an active session
		// for this task — this usually means project init created a default agent
		// (e.g. reviewer-main) and the operator also added a custom one (e.g. claude-reviewer),
		// leading to two reviewer sessions consuming the same task context.
		allActive, listErr := sessionService.ListByProject(result.Project.ID)
		if listErr == nil {
			for _, s := range allActive {
				if s.AgentName != agentRecord.Name && s.RoleName == agentRecord.Role &&
					s.TaskID == params.taskID && isActiveSessionStatus(s.Status) {
					fmt.Fprintf(r.stdout, "Warning: agent %q (role=%s) already has an active session %s for this task.\n", s.AgentName, s.RoleName, s.ID)
					fmt.Fprintln(r.stdout, "         Two agents with the same role on the same task may duplicate work or conflict.")
					fmt.Fprintf(r.stdout, "         If %q is no longer needed, stop it first: aom session stop %s\n", s.AgentName, s.ID)
					fmt.Fprintln(r.stdout, "")
				}
			}
		}
	}

	record, err := sessionService.Create(session.CreateParams{
		ProjectID:       result.Project.ID,
		AgentID:         agentRecord.ID,
		AgentName:       agentRecord.Name,
		RoleName:        agentRecord.Role,
		TaskID:          params.taskID,
		Runtime:         agentRecord.Runtime,
		Model:           agentRecord.Model,
		Status:          "Booting",
		RepoPath:        result.Project.RepoPath,
		WorktreePath:    executionPath,
		TmuxSessionName: workspace.Name,
	})
	if err != nil {
		return nil, err
	}

	if taskRecord != nil {
		if err := r.syncTaskArtifactsWithSessionEvents(result, taskRecord.ID, false, record, artifact.Event{
			Type:        "session.created",
			Actor:       "aom",
			StepID:      params.stepID,
			SessionID:   record.ID,
			Summary:     fmt.Sprintf("Session record created for %s using %s launch mode", agentRecord.Name, params.launchMode),
			StateEffect: "Session Booting",
		}); err != nil {
			return nil, err
		}
	}

	agentSessionID := ""
	if params.taskID != "" && !params.freshStart {
		agentSessionID, _ = sessionService.LatestVendorSessionID(params.taskID, agentRecord.Name)
	}

	selfBin, _ := os.Executable()
	launchCommand, err := newLaunchBuilder().Build(aomruntime.SessionSpec{
		SessionID:      record.ID,
		AgentName:      record.AgentName,
		RoleName:       record.RoleName,
		Runtime:        record.Runtime,
		AgentSessionID: agentSessionID,
		DenyCommands:   result.Policy.Policy.DenyCommands,
		ProjectBin:     selfBin,
		Model:          agentRecord.Model,
	}, params.launchMode)
	if err != nil {
		return nil, r.failTaskBoundSessionSpawn(result, sessionService, record, taskRecord, params.stepID, "session launch validation failed before session became interactive", err)
	}

	if err := r.materializeAgentContext(result, agentRecord, executionPath); err != nil {
		return nil, fmt.Errorf("materialize agent context: %w", err)
	}

	r.enforcePolicyDefaults(result, agentRecord.Runtime, agentRecord.Model)

	// Capture before pane creation so native session files written during process
	// startup have mtime >= spawnedAt (used by claude's filesystem-based detection).
	spawnedAt := time.Now()

	paneBinding, err := r.app.Tmux.CreatePane(workspace.Target, executionPath, launchCommand)
	if err != nil {
		return nil, r.failTaskBoundSessionSpawn(result, sessionService, record, taskRecord, params.stepID, "pane creation failed before session became interactive", err)
	}

	// Set pane binding on record before any early-exit paths so failure saves include tmux context.
	record.TmuxWindow = paneBinding.WindowID
	record.TmuxPane = paneBinding.PaneID
	record.TmuxSessionName = workspace.Name

	// For real-mode sessions, verify the runtime process didn't exit immediately.
	// Some runtimes (e.g. codex) silently quit on auth failure, leaving the pane closed.
	if params.launchMode == aomruntime.LaunchModeReal {
		time.Sleep(1200 * time.Millisecond)
		if alive, _ := r.app.Tmux.PaneExists(paneBinding.PaneID); !alive {
			_ = r.app.Tmux.KillPane(paneBinding.PaneID)
			return nil, r.failTaskBoundSessionSpawn(result, sessionService, record, taskRecord, params.stepID,
				fmt.Sprintf("%q runtime exited within 1.2s of launch — check binary, PATH, and authentication", agentRecord.Runtime),
				fmt.Errorf("runtime %q pane %s closed immediately after spawn — check binary and authentication", agentRecord.Runtime, paneBinding.PaneID))
		}
	}

	record.Status = "Idle"

	// Label the window with the agent name so operators can identify sessions at a glance.
	_ = r.app.Tmux.RenameWindow(paneBinding.WindowID, agentRecord.Name)

	record, err = sessionService.Save(*record)
	if err != nil {
		return nil, err
	}

	if taskRecord != nil {
		worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
		if err != nil {
			return nil, err
		}
		taskWorktree, err = worktreeService.Reconcile(taskRecord.ID, result.Project.RepoPath, true)
		worktreeDB.Close()
		if err != nil {
			return nil, err
		}
	}

	if taskRecord != nil {
		if err := r.syncTaskArtifactsWithSessionEvents(result, taskRecord.ID, false, record, artifact.Event{
			Type:        "session.ready",
			Actor:       "aom",
			StepID:      params.stepID,
			SessionID:   record.ID,
			Summary:     fmt.Sprintf("Session pane attached for %s and ready for operator inspection", agentRecord.Name),
			StateEffect: fmt.Sprintf("Session %s", record.Status),
		}); err != nil {
			return nil, err
		}
		if err := r.ensureTaskHandoffTemplate(result, taskRecord.ID, record); err != nil {
			return nil, err
		}
		_ = appendChannelMessage(result.Project.RepoPath, "aom",
			fmt.Sprintf("spawned %s (session %s) for task %s — waiting for operator prompt",
				agentRecord.Name, record.ID, taskRecord.ID),
			time.Now())
	}

	if err := r.app.Tmux.AnnotatePane(record.TmuxPane, map[string]string{
		"@aom_session_id": record.ID,
		"@aom_agent":      record.AgentName,
		"@aom_role":       record.RoleName,
	}); err != nil {
		return nil, r.failTaskBoundSessionSpawn(result, sessionService, record, taskRecord, params.stepID, "pane annotation failed after session launch", err)
	}

	fmt.Fprintln(r.stdout, "Session spawned")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session: %s\n", record.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", record.AgentName)
	fmt.Fprintf(r.stdout, "Role: %s\n", record.RoleName)
	if taskRecord != nil {
		fmt.Fprintf(r.stdout, "Task: %s\n", taskRecord.ID)
	}
	if stepRecord != nil {
		fmt.Fprintf(r.stdout, "Step: %s\n", stepRecord.ID)
	}
	fmt.Fprintf(r.stdout, "Runtime: %s\n", record.Runtime)
	if record.Model != "" {
		fmt.Fprintf(r.stdout, "Model: %s\n", record.Model)
	} else if hint := r.registry.Lookup(record.Runtime).ModelHint(); hint != "" && params.launchMode == aomruntime.LaunchModeReal {
		fmt.Fprintf(r.stdout, "Model: (provider default) — %s\n", hint)
	}
	fmt.Fprintf(r.stdout, "Launch mode: %s\n", params.launchMode)
	if taskWorktree != nil {
		fmt.Fprintf(r.stdout, "Worktree status: %s\n", taskWorktree.Status)
	}
	fmt.Fprintf(r.stdout, "Worktree path: %s\n", record.WorktreePath)
	fmt.Fprintf(r.stdout, "Workspace: %s\n", workspace.Target)
	fmt.Fprintf(r.stdout, "Window: %s\n", record.TmuxWindow)
	fmt.Fprintf(r.stdout, "Pane: %s\n", record.TmuxPane)

	if params.taskID != "" {
		fmt.Fprintln(r.stdout, "")
		if agentSessionID != "" {
			fmt.Fprintf(r.stdout, "Continuity: resuming previous session %s\n", agentSessionID)
			fmt.Fprintln(r.stdout, "            Use --fresh to start a clean context if this is unrelated work.")
		} else if params.freshStart {
			fmt.Fprintln(r.stdout, "Continuity: fresh start (--fresh flag set — previous context ignored)")
		} else {
			fmt.Fprintln(r.stdout, "Continuity: fresh start (no previous session found for this task)")
		}

		artifactRoot := taskArtifactRoot(result.Project.RepoPath, result.StateDir, params.taskID, taskWorktree)
		taskMDPath := filepath.Join(artifactRoot, "task.md")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "Context: %s\n", taskMDPath)
		fmt.Fprintln(r.stdout, "         Read this file before starting work on the task.")
		if params.launchMode == aomruntime.LaunchModeReal {
			fmt.Fprintf(r.stdout, "         To send it: aom session send %s \"@%s\"\n", record.ID, taskMDPath)
		}
	}

	if params.launchMode == aomruntime.LaunchModeReal {
		if strategy := r.registry.Lookup(record.Runtime).NativeSessionDetection(); strategy != nil {
			if agentSessionID != "" {
				fmt.Fprintf(r.stdout, "Native session ID: %s (resumed)\n", agentSessionID)
			} else {
				fmt.Fprintln(r.stdout, "")
				fmt.Fprintln(r.stdout, "Detecting native session ID (this may take up to 90s)...")
				if key := r.registry.Lookup(record.Runtime).StartupDialogResponse(); key != "" {
					// Send the startup dialog response up to 3 times at 4s intervals to
					// cover runtimes whose interactive prompt takes longer to appear.
					for attempt := 0; attempt < 3; attempt++ {
						time.Sleep(4 * time.Second)
						_ = r.app.Tmux.SendKeys(record.TmuxPane, key)
					}
				} else {
					time.Sleep(3 * time.Second)
				}
				detected := r.detectUniqueVendorSessionID(strategy, sessionService, *record, spawnedAt)
				if detected != "" {
					if updated, err := sessionService.SetVendorSessionID(record.ID, detected); err == nil {
						record = updated
					}
					fmt.Fprintf(r.stdout, "Native session ID: %s (auto-detected)\n", detected)
				} else {
					fmt.Fprintln(r.stdout, "Native session ID not yet available")
					fmt.Fprintf(r.stdout, "To register manually: aom session set-agent-id %s <uuid>\n", record.ID)
				}
			}
		}
	}

	go runHook(result.Project.RepoPath, "on-session-spawn", record.ID, record.AgentName, record.TaskID)

	// For real-mode codex sessions with a task, auto-send a commit reminder so
	// codex knows to commit its work before signaling completion. Sent after the
	// startup dialog loop to avoid interfering with the "1" response keys.
	if record.Runtime == "codex" && params.taskID != "" && params.launchMode == aomruntime.LaunchModeReal {
		commitReminder := fmt.Sprintf(
			"IMPORTANT: when finished, commit synchronously in the foreground (NOT in a background terminal):\n"+
				"  git add -A && git commit -m \"implement %s\"\n"+
				"  If that fails for ANY reason, use: aom worktree commit %s -m \"implement %s\"\n"+
				"  Do NOT use timeout wrappers, perl alarms, or retry loops — use aom worktree commit instead.\n"+
				"Then append task.completed to .agent/log.md",
			params.taskID, params.taskID, params.taskID,
		)
		_ = r.app.Tmux.SendKeys(record.TmuxPane, commitReminder)
	}

	return record, nil
}

func (r Runner) executeSessionReplace(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	params := sessionReplaceParams{
		id:         strings.TrimSpace(args[0]),
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
		case "--reason":
			i++
			if i >= len(args) {
				return fmt.Errorf("--reason requires a value")
			}
			params.reason = strings.TrimSpace(args[i])
		case "--mock":
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModeMock); err != nil {
				return err
			}
		case "--real":
			// --real is the default; accepted for backwards compatibility
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModeReal); err != nil {
				return err
			}
		case "--placeholder":
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModePlaceholder); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if params.agentName == "" {
		return fmt.Errorf("--agent is required")
	}

	oldRecord, err := r.loadSessionByIdentifier(params.id)
	if err != nil {
		return err
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	agentRecord, err := findAgent(result.Agents, params.agentName)
	if err != nil {
		return err
	}

	if err := r.replaceSession(result, *oldRecord, agentRecord, params); err != nil {
		return err
	}
	return nil
}

func (r Runner) failTaskBoundSessionSpawn(
	result *project.OpenResult,
	sessionService *session.Service,
	record *session.Record,
	taskRecord *task.Record,
	stepID string,
	summary string,
	cause error,
) error {
	if record == nil {
		return cause
	}

	record.Status = "Failed"
	if _, err := sessionService.Save(*record); err != nil {
		return fmt.Errorf("%w (also failed to persist failed session state: %v)", cause, err)
	}

	if taskRecord != nil {
		appendErr := r.syncTaskArtifactsWithSessionEvents(result, taskRecord.ID, false, record, artifact.Event{
			Type:        "session.failed",
			Actor:       "aom",
			StepID:      stepID,
			SessionID:   record.ID,
			Summary:     summary,
			StateEffect: "Session Failed",
		})
		if appendErr != nil {
			return fmt.Errorf("%w (also failed to append task log event: %v)", cause, appendErr)
		}
	}

	return cause
}

type sessionSpawnParams struct {
	agentName       string
	taskID          string
	stepID          string
	ignoreSessionID string
	launchMode      aomruntime.LaunchMode
	freshStart      bool
}

type sessionReplaceParams struct {
	id         string
	agentName  string
	reason     string
	launchMode aomruntime.LaunchMode
}

func (r Runner) executeSessionList(args []string) error {
	activeOnly := false
	for _, arg := range args {
		if arg == "--active" {
			activeOnly = true
		} else {
			return fmt.Errorf("unknown flag %q — session list accepts only --active", arg)
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return err
	}

	if activeOnly {
		fmt.Fprintln(r.stdout, "Sessions (active)")
	} else {
		fmt.Fprintln(r.stdout, "Sessions")
	}
	fmt.Fprintln(r.stdout, "")

	printed := 0
	for _, item := range sessions {
		if activeOnly && !isActiveSessionStatus(item.Status) {
			continue
		}
		modelSuffix := ""
		if item.Model != "" {
			modelSuffix = " | model=" + item.Model
		}
		fmt.Fprintf(
			r.stdout,
			"  - %s | agent=%s | role=%s | task=%s | runtime=%s%s | status=%s | tmux=%s %s %s\n",
			item.ID,
			item.AgentName,
			item.RoleName,
			emptyFallback(item.TaskID),
			item.Runtime,
			modelSuffix,
			colorStatus(item.Status, r.stdout),
			item.TmuxSessionName,
			item.TmuxWindow,
			item.TmuxPane,
		)
		if item.Status == "Detached" {
			fmt.Fprintf(r.stdout, "    next=%s\n", detachedSessionHint(item))
		}
		if readiness := sessionReadiness(result.Project.RepoPath, item); readiness != "" {
			fmt.Fprintf(r.stdout, "    readiness=%s\n", readiness)
		}
		printed++
	}

	if printed == 0 {
		if activeOnly {
			fmt.Fprintln(r.stdout, "No active sessions")
		} else {
			fmt.Fprintln(r.stdout, "No sessions")
		}
	}

	return nil
}

// sessionReadiness returns a computed readiness label for the session, giving
// operators a quick signal beyond just the status string.
func sessionReadiness(repoPath string, s session.Record) string {
	if s.TaskID == "" {
		return ""
	}
	switch s.Status {
	case "WaitingApproval", "WaitingHandoff", "Blocked":
		return "needs-operator"
	case "Stopped", "Failed", "Detached", "Archived":
		return ""
	}
	// Idle or Working — compute from artifacts.
	if s.WorktreePath != "" {
		logPath := filepath.Join(s.WorktreePath, ".agent", "log.md")
		if hasTaskCompletedEvent(logPath) {
			return "done-pending-review"
		}
		// Check outbox.
		if _, err := os.Stat(filepath.Join(s.WorktreePath, ".agent", "outbox.md")); err == nil {
			return "awaiting-peer"
		}
	}
	// Check if agent posted to channel.
	channelData, _ := readChannelFile(repoPath)
	if strings.Contains(channelData, "| "+s.AgentName+"\n") {
		return "in-progress"
	}
	return "no-progress"
}

// detachedSessionHint returns the recommended next-action command for a Detached session.
func detachedSessionHint(s session.Record) string {
	if s.VendorSessionID != "" {
		return fmt.Sprintf("aom session resume %s  (native session available)", s.ID)
	}
	if s.TaskID != "" {
		return fmt.Sprintf("aom session recover %s  (task-backed; inspect then spawn or archive)", s.ID)
	}
	return fmt.Sprintf("aom session archive %s  (no task or native session — safe to archive)", s.ID)
}

func (r Runner) executeSessionShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("session show does not accept extra positional arguments in the current milestone")
	}

	sessionRecord, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Session")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "ID: %s\n", sessionRecord.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", sessionRecord.AgentName)
	fmt.Fprintf(r.stdout, "Role: %s\n", sessionRecord.RoleName)
	fmt.Fprintf(r.stdout, "Task: %s\n", emptyFallback(sessionRecord.TaskID))
	fmt.Fprintf(r.stdout, "Runtime: %s\n", sessionRecord.Runtime)
	fmt.Fprintf(r.stdout, "Status: %s\n", sessionRecord.Status)
	fmt.Fprintf(r.stdout, "Repo: %s\n", sessionRecord.RepoPath)
	fmt.Fprintf(r.stdout, "Worktree: %s\n", sessionRecord.WorktreePath)
	fmt.Fprintf(r.stdout, "Tmux session: %s\n", sessionRecord.TmuxSessionName)
	fmt.Fprintf(r.stdout, "Tmux window: %s\n", sessionRecord.TmuxWindow)
	fmt.Fprintf(r.stdout, "Tmux pane: %s\n", sessionRecord.TmuxPane)
	if sessionRecord.VendorSessionID != "" {
		fmt.Fprintf(r.stdout, "Native session ID: %s\n", sessionRecord.VendorSessionID)
	}

	return nil
}

func (r Runner) replaceSession(result *project.OpenResult, oldRecord session.Record, agentRecord *agent.Record, params sessionReplaceParams) error {
	if agentRecord == nil {
		return fmt.Errorf("replacement agent is required")
	}

	spawnParams := sessionSpawnParams{
		agentName:       agentRecord.Name,
		taskID:          oldRecord.TaskID,
		ignoreSessionID: oldRecord.ID,
		launchMode:      params.launchMode,
	}

	taskRecord, err := r.loadTaskByID(result, oldRecord.TaskID)
	if err != nil {
		return err
	}
	if oldRecord.TaskID != "" && taskRecord == nil {
		return fmt.Errorf("task %q not found", oldRecord.TaskID)
	}

	oldStatusBefore := oldRecord.Status
	newSession, err := r.executeResolvedSessionSpawn(result, agentRecord, spawnParams)
	if err != nil {
		return err
	}

	stoppedStatus := oldRecord.Status
	oldSessionOutcome := fmt.Sprintf("left running (%s requires operator intervention)", oldRecord.Status)
	oldSessionHint := ""
	stopWarning := ""
	if archivableReplacementSession(oldRecord.Status) {
		stopped, warning, err := r.stopSessionRecord(result, oldRecord, false)
		if err != nil {
			return err
		}
		archived, err := r.archiveSessionRecord(result, *stopped, false, "Superseded session archived automatically after replacement")
		if err != nil {
			return err
		}
		stoppedStatus = archived.Status
		stopWarning = warning
		oldSessionOutcome = fmt.Sprintf("archived (%s)", archived.Status)
		if warning != "" {
			oldSessionOutcome = fmt.Sprintf("archived with warning (%s)", archived.Status)
		}
	} else if stoppableReplacementSession(oldRecord.Status) {
		stopped, warning, err := r.stopSessionRecord(result, oldRecord, false)
		if err != nil {
			return err
		}
		stoppedStatus = stopped.Status
		stopWarning = warning
		oldSessionOutcome = fmt.Sprintf("stopped (%s)", stopped.Status)
		if warning != "" {
			oldSessionOutcome = fmt.Sprintf("stopped with warning (%s)", stopped.Status)
		}
	} else {
		oldSessionHint = fmt.Sprintf("run \"aom session stop %s\" after inspecting whether the previous session should be shut down", oldRecord.ID)
	}

	if strings.TrimSpace(oldRecord.TaskID) != "" {
		reason := params.reason
		if reason == "" {
			reason = "operator requested replacement"
		}
		if err := r.syncTaskArtifactsWithSessionEvents(result, oldRecord.TaskID, false, newSession, artifact.Event{
			Type:        "session.replaced",
			Actor:       "operator",
			SessionID:   newSession.ID,
			Summary:     fmt.Sprintf("Session %s replaced %s using agent %s (%s)", newSession.ID, oldRecord.ID, agentRecord.Name, reason),
			StateEffect: fmt.Sprintf("Old session %s, new session %s", stoppedStatus, newSession.Status),
		}); err != nil {
			return err
		}
	}

	fmt.Fprintln(r.stdout, "Session replaced")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Old session: %s\n", oldRecord.ID)
	fmt.Fprintf(r.stdout, "Old status: %s\n", oldStatusBefore)
	fmt.Fprintf(r.stdout, "Old session result: %s\n", oldSessionOutcome)
	fmt.Fprintf(r.stdout, "New session: %s\n", newSession.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", newSession.AgentName)
	fmt.Fprintf(r.stdout, "Reason: %s\n", emptyFallback(params.reason))
	if stopWarning != "" {
		fmt.Fprintf(r.stdout, "Warning: %s\n", stopWarning)
	}
	if oldSessionHint != "" {
		fmt.Fprintf(r.stdout, "Action hint: %s\n", oldSessionHint)
	}
	fmt.Fprintln(r.stdout, "Continuity quality: artifact-backed")
	fmt.Fprintln(r.stdout, "Next recommended action: inspect the replacement session and continue work from the same task context")

	return nil
}

func (r Runner) stopSessionRecord(result *project.OpenResult, record session.Record, print bool) (*session.Record, string, error) {
	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return nil, "", err
	}
	defer sqlDB.Close()

	recordRefreshed, err := r.reconcileSessionRecord(sessionService, record)
	if err != nil {
		return nil, "", err
	}
	record = *recordRefreshed

	warning := ""
	if strings.TrimSpace(record.TmuxPane) != "" && record.Status != "Detached" {
		paneExists, err := r.app.Tmux.PaneExists(record.TmuxPane)
		if err != nil {
			return nil, "", err
		}
		if paneExists {
			if err := r.app.Tmux.KillPane(record.TmuxPane); err != nil {
				warning = fmt.Sprintf("tmux pane cleanup failed for %s: %v", record.TmuxPane, err)
			}
		}
	}

	// Clean up any policy-wrapper processes and /tmp artefacts left behind by
	// codex's PATH-based enforcement. Safe to call for all runtimes: the
	// function is a no-op when the session's policy directory does not exist.
	if err := provider.CleanupSession(record.ID); err != nil && warning == "" {
		warning = fmt.Sprintf("policy dir cleanup warning: %v", err)
	}

	stopped, err := sessionService.Stop(record)
	if err != nil {
		return nil, warning, err
	}

	if strings.TrimSpace(stopped.TaskID) != "" {
		summary := "Session stopped explicitly by operator"
		stateEffect := "Session Stopped"
		if warning != "" {
			summary = fmt.Sprintf("%s (tmux cleanup warning: %s)", summary, warning)
			stateEffect = "Session Stopped with tmux cleanup warning"
		}
		if err := r.syncTaskArtifacts(result, stopped.TaskID, artifact.Event{
			Type:        "session.stopped",
			Actor:       "operator",
			SessionID:   stopped.ID,
			Summary:     summary,
			StateEffect: stateEffect,
		}, false); err != nil {
			return nil, warning, err
		}
	}

	if print {
		fmt.Fprintln(r.stdout, "Session stopped")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "Session: %s\n", stopped.ID)
		fmt.Fprintf(r.stdout, "Status: %s\n", stopped.Status)
		fmt.Fprintf(r.stdout, "Task: %s\n", emptyFallback(stopped.TaskID))
		if warning != "" {
			fmt.Fprintf(r.stdout, "Warning: %s\n", warning)
		}
	}

	return stopped, warning, nil
}

func (r Runner) archiveSessionRecord(result *project.OpenResult, record session.Record, print bool, summary string) (*session.Record, error) {
	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	// Defensive cleanup: remove any remaining /tmp artefacts. The policy dir
	// was already removed by stopSessionRecord in most paths, but archive can
	// be called directly so we clean up here too.
	_ = provider.CleanupSession(record.ID)

	archived, err := sessionService.Archive(record)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(archived.TaskID) != "" {
		if err := r.syncTaskArtifacts(result, archived.TaskID, artifact.Event{
			Type:        "session.archived",
			Actor:       "operator",
			SessionID:   archived.ID,
			Summary:     summary,
			StateEffect: "Session Archived",
		}, false); err != nil {
			return nil, err
		}
	}

	if print {
		fmt.Fprintln(r.stdout, "Session archived")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "Session: %s\n", archived.ID)
		fmt.Fprintf(r.stdout, "Status: %s\n", archived.Status)
		fmt.Fprintf(r.stdout, "Task: %s\n", emptyFallback(archived.TaskID))
	}

	return archived, nil
}

func stoppableReplacementSession(status string) bool {
	switch status {
	case "Idle", "WaitingHandoff":
		return true
	default:
		return false
	}
}

func archivableReplacementSession(status string) bool {
	return status == "Detached"
}

func (r Runner) executeSessionStop(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("session stop does not accept extra positional arguments in the current milestone")
	}

	record, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	_, _, err = r.stopSessionRecord(result, *record, true)
	return err
}

func (r Runner) executeSessionArchive(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("session archive does not accept extra positional arguments in the current milestone")
	}

	record, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	_, err = r.archiveSessionRecord(result, *record, true, "Session archived explicitly by operator")
	return err
}

func (r Runner) executeSessionResume(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	sessionIdentifier := strings.TrimSpace(args[0])
	newTaskID := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--task":
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			newTaskID = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	record, err := r.loadSessionByIdentifier(sessionIdentifier)
	if err != nil {
		return err
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// With --task: rebind the session to a different task (original behaviour).
	if newTaskID != "" {
		return r.executeSessionResumeToTask(result, record, newTaskID)
	}

	// Without --task: smart auto-recovery — pick the best continuity path.
	return r.executeSessionAutoResume(result, record)
}

// executeSessionResumeToTask rebinds an Idle or WaitingHandoff session to a
// new task without spawning a new process. The live tmux pane is cd'd into the
// new worktree and agent context is re-materialised there.
func (r Runner) executeSessionResumeToTask(result *project.OpenResult, record *session.Record, newTaskID string) error {
	if record.Status != "WaitingHandoff" && record.Status != "Idle" {
		return fmt.Errorf("session %q cannot be resumed for a new task (status: %s); session must be Idle or WaitingHandoff", record.ID, record.Status)
	}

	newTaskRecord, err := r.loadTaskByID(result, newTaskID)
	if err != nil {
		return err
	}

	agentRecord, err := findAgent(result.Agents, record.AgentName)
	if err != nil {
		return err
	}

	if err := r.enforceWriterWorktreeBoundary(result, agentRecord, sessionSpawnParams{
		taskID:          newTaskID,
		ignoreSessionID: record.ID,
	}); err != nil {
		return err
	}

	newTaskWorktree, newExecutionPath, err := r.resolveTaskExecutionPath(result, *newTaskRecord)
	if err != nil {
		return err
	}

	oldTaskID := record.TaskID

	record.TaskID = newTaskID
	record.WorktreePath = newExecutionPath
	record.Status = "Idle"

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	updated, err := sessionService.Save(*record)
	if err != nil {
		return err
	}

	// Change the pane's working directory to the new task's worktree.
	if strings.TrimSpace(record.TmuxPane) != "" && strings.TrimSpace(newExecutionPath) != "" {
		_ = r.app.Tmux.SendKeys(record.TmuxPane, "cd "+newExecutionPath)
	}

	// Deliver agent context (identity, skills, MCP) into the new worktree.
	if agentRec := findAgentByName(result.Agents, record.AgentName); agentRec != nil {
		_ = r.materializeAgentContext(result, agentRec, newExecutionPath)
	}

	// Advance new task's worktree to Active.
	if newTaskWorktree != nil {
		worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
		if err == nil {
			_, _ = worktreeService.Reconcile(newTaskID, result.Project.RepoPath, true)
			worktreeDB.Close()
		}
	}

	// Record the session's departure from the old task.
	if strings.TrimSpace(oldTaskID) != "" && oldTaskID != newTaskID {
		_ = r.syncTaskArtifacts(result, oldTaskID, artifact.Event{
			Type:        "operator.intervention",
			Actor:       "operator",
			SessionID:   record.ID,
			Summary:     fmt.Sprintf("Session %s (%s) transferred to task %s", record.ID, record.AgentName, newTaskID),
			StateEffect: "Session transferred out",
		}, false)
	}

	// Bind session to the new task's artifacts.
	if err := r.syncTaskArtifactsWithSessionEvents(result, newTaskID, false, updated, artifact.Event{
		Type:        "session.ready",
		Actor:       "aom",
		SessionID:   updated.ID,
		Summary:     fmt.Sprintf("Session %s (%s) resumed and bound to this task", updated.ID, updated.AgentName),
		StateEffect: fmt.Sprintf("Session %s", updated.Status),
	}); err != nil {
		return err
	}

	_ = r.ensureTaskHandoffTemplate(result, newTaskID, updated)

	fmt.Fprintln(r.stdout, "Session resumed")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session: %s\n", updated.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", updated.AgentName)
	fmt.Fprintf(r.stdout, "Task: %s\n", updated.TaskID)
	if strings.TrimSpace(oldTaskID) != "" && oldTaskID != newTaskID {
		fmt.Fprintf(r.stdout, "Previous task: %s\n", oldTaskID)
	}
	fmt.Fprintf(r.stdout, "Status: %s\n", updated.Status)
	fmt.Fprintf(r.stdout, "Worktree: %s\n", updated.WorktreePath)
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Next: aom session send %s \"read .agent/task.md and begin\"\n", updated.ID)
	return nil
}

// executeSessionAutoResume picks the best continuity path for a session without
// requiring a new task assignment. Decision tree:
//
//  1. Tmux pane still alive → un-detach (fastest path, context fully intact)
//  2. VendorSessionID exists → new tmux pane resuming the native agent session
//     (claude --resume <uuid> / codex resume <session-id>)
//  3. TaskID exists but no native session → print spawn hint, no state mutation
//  4. Nothing recoverable → print archive hint
func (r Runner) executeSessionAutoResume(result *project.OpenResult, record *session.Record) error {
	// Path 1: pane still alive — un-detach without touching the agent process.
	if strings.TrimSpace(record.TmuxPane) != "" && r.app.Tmux.Availability().Available {
		if alive, _ := r.app.Tmux.PaneExists(record.TmuxPane); alive {
			sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			record.Status = "Idle"
			updated, err := sessionService.Save(*record)
			if err != nil {
				return err
			}

			if strings.TrimSpace(record.TaskID) != "" {
				_ = r.syncTaskArtifacts(result, record.TaskID, artifact.Event{
					Type:        "session.resumed",
					Actor:       "aom",
					SessionID:   record.ID,
					Summary:     fmt.Sprintf("Session %s (%s) resumed — pane still alive", record.ID, record.AgentName),
					StateEffect: "Session Idle",
				}, false)
			}

			fmt.Fprintln(r.stdout, "Session resumed (pane still alive)")
			fmt.Fprintln(r.stdout, "")
			fmt.Fprintf(r.stdout, "Session: %s\n", updated.ID)
			fmt.Fprintf(r.stdout, "Agent:   %s\n", updated.AgentName)
			fmt.Fprintf(r.stdout, "Pane:    %s\n", updated.TmuxPane)
			fmt.Fprintf(r.stdout, "Status:  Idle\n")
			if updated.TaskID != "" {
				fmt.Fprintf(r.stdout, "Task:    %s\n", updated.TaskID)
			}
			fmt.Fprintln(r.stdout, "")
			fmt.Fprintf(r.stdout, "Next: aom attach %s\n", updated.ID)
			return nil
		}
	}

	// Path 2: native session ID available — re-open the agent in a new tmux pane.
	if strings.TrimSpace(record.VendorSessionID) != "" {
		return r.resumeSessionNative(result, record)
	}

	// Path 3: task-bound but no native session ID.
	if strings.TrimSpace(record.TaskID) != "" {
		fmt.Fprintln(r.stdout, "No live pane and no native session ID — full spawn required.")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "To resume with task context:  aom session spawn %s --task %s --real\n", record.AgentName, record.TaskID)
		return nil
	}

	// Path 4: no recovery path.
	fmt.Fprintln(r.stdout, "No recovery path available (no live pane, no native session, no task).")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "To clean up: aom session archive %s\n", record.ID)
	return nil
}

// resumeSessionNative creates a new tmux pane that resumes the agent's native
// conversation context using the stored VendorSessionID. No startup dialog is
// sent — claude --resume and codex resume -a never skip interactive prompts.
func (r Runner) resumeSessionNative(result *project.OpenResult, record *session.Record) error {
	if !r.app.Tmux.Availability().Available {
		return fmt.Errorf("tmux is not available; cannot create a new pane to resume session")
	}

	// Guard: if the session is still in an active status, re-check the pane before
	// creating a new one. Prevents duplicate panes when the path-1 liveness check
	// in executeSessionAutoResume suffered a transient tmux glitch.
	if isActiveSessionStatus(record.Status) && strings.TrimSpace(record.TmuxPane) != "" {
		if alive, _ := r.app.Tmux.PaneExists(record.TmuxPane); alive {
			return fmt.Errorf(
				"session %q is %s and pane %s is still alive — use \"aom attach %s\" to reconnect",
				record.ID, record.Status, record.TmuxPane, record.ID,
			)
		}
	}

	workspace, err := r.app.Tmux.EnsureWorkspace(result.SessionPrefix, result.Project.RepoPath)
	if err != nil {
		return fmt.Errorf("ensure tmux workspace: %w", err)
	}

	executionPath := record.WorktreePath
	if strings.TrimSpace(executionPath) == "" {
		executionPath = result.Project.RepoPath
	}

	selfBin, _ := os.Executable()
	launchCmd, err := newLaunchBuilder().Build(aomruntime.SessionSpec{
		SessionID:      record.ID,
		AgentName:      record.AgentName,
		RoleName:       record.RoleName,
		Runtime:        record.Runtime,
		AgentSessionID: record.VendorSessionID,
		DenyCommands:   result.Policy.Policy.DenyCommands,
		ProjectBin:     selfBin,
		Model:          record.Model,
	}, aomruntime.LaunchModeReal)
	if err != nil {
		return fmt.Errorf("build resume launch command: %w", err)
	}

	paneBinding, err := r.app.Tmux.CreatePane(workspace.Target, executionPath, launchCmd)
	if err != nil {
		return fmt.Errorf("create tmux pane for resume: %w", err)
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		_ = r.app.Tmux.KillPane(paneBinding.PaneID)
		return err
	}
	defer sqlDB.Close()

	record.Status = "Idle"
	record.TmuxSessionName = workspace.Name
	record.TmuxWindow = paneBinding.WindowID
	record.TmuxPane = paneBinding.PaneID

	updated, err := sessionService.Save(*record)
	if err != nil {
		_ = r.app.Tmux.KillPane(paneBinding.PaneID)
		return err
	}

	_ = r.app.Tmux.RenameWindow(paneBinding.WindowID, record.AgentName)
	_ = r.app.Tmux.AnnotatePane(paneBinding.PaneID, map[string]string{
		"@aom_session_id": record.ID,
		"@aom_agent":      record.AgentName,
		"@aom_role":       record.RoleName,
	})

	if strings.TrimSpace(record.TaskID) != "" {
		_ = r.syncTaskArtifacts(result, record.TaskID, artifact.Event{
			Type:      "session.resumed",
			Actor:     "aom",
			SessionID: record.ID,
			Summary: fmt.Sprintf("Session %s (%s) resumed native %s context (session %s) in new pane",
				record.ID, record.AgentName, record.Runtime, record.VendorSessionID),
			StateEffect: "Session Idle",
		}, false)
	}

	fmt.Fprintln(r.stdout, "Session resumed (native context)")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session:        %s\n", updated.ID)
	fmt.Fprintf(r.stdout, "Agent:          %s\n", updated.AgentName)
	fmt.Fprintf(r.stdout, "Runtime:        %s\n", updated.Runtime)
	fmt.Fprintf(r.stdout, "Native session: %s (resumed)\n", updated.VendorSessionID)
	fmt.Fprintf(r.stdout, "Pane:           %s\n", updated.TmuxPane)
	fmt.Fprintf(r.stdout, "Window:         %s\n", updated.TmuxWindow)
	if updated.TaskID != "" {
		fmt.Fprintf(r.stdout, "Task:           %s\n", updated.TaskID)
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Next: aom attach %s\n", updated.ID)
	return nil
}

// executeSessionRebind reconnects a Detached session to a live tmux pane
// without replacing the session record or re-spawning the runtime. If the
// existing pane is still alive the session is simply un-detached. If the pane
// is gone a new placeholder pane is created in the project workspace.
func (r Runner) executeSessionRebind(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	record, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if record.Status != "Detached" {
		return fmt.Errorf("session %q is %s; rebind only applies to Detached sessions", record.ID, record.Status)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// If the existing pane is still alive, just un-detach.
	if strings.TrimSpace(record.TmuxPane) != "" {
		if alive, _ := r.app.Tmux.PaneExists(record.TmuxPane); alive {
			sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			record.Status = "Idle"
			if _, err := sessionService.Save(*record); err != nil {
				return err
			}

			fmt.Fprintln(r.stdout, "Session rebound (pane still alive)")
			fmt.Fprintln(r.stdout, "")
			fmt.Fprintf(r.stdout, "Session: %s\n", record.ID)
			fmt.Fprintf(r.stdout, "Pane: %s\n", record.TmuxPane)
			fmt.Fprintf(r.stdout, "Status: Idle\n")
			return nil
		}
	}

	// Pane is gone — create a new one in the project workspace.
	workspace, err := r.app.Tmux.EnsureWorkspace(result.SessionPrefix, result.Project.RepoPath)
	if err != nil {
		return fmt.Errorf("ensure workspace for rebind: %w", err)
	}

	launchCommand, err := newLaunchBuilder().Build(aomruntime.SessionSpec{
		SessionID:      record.ID,
		AgentName:      record.AgentName,
		RoleName:       record.RoleName,
		Runtime:        record.Runtime,
		AgentSessionID: record.VendorSessionID,
	}, aomruntime.LaunchModePlaceholder)
	if err != nil {
		return fmt.Errorf("build launch command for rebind: %w", err)
	}

	executionPath := record.WorktreePath
	if strings.TrimSpace(executionPath) == "" {
		executionPath = result.Project.RepoPath
	}

	paneBinding, err := r.app.Tmux.CreatePane(workspace.Target, executionPath, launchCommand)
	if err != nil {
		return fmt.Errorf("create pane for rebind: %w", err)
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	record.Status = "Idle"
	record.TmuxSessionName = workspace.Name
	record.TmuxWindow = paneBinding.WindowID
	record.TmuxPane = paneBinding.PaneID

	if _, err := sessionService.Save(*record); err != nil {
		return err
	}

	_ = r.app.Tmux.RenameWindow(paneBinding.WindowID, record.AgentName)

	fmt.Fprintln(r.stdout, "Session rebound (new pane)")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session: %s\n", record.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", record.AgentName)
	fmt.Fprintf(r.stdout, "Pane: %s\n", record.TmuxPane)
	fmt.Fprintf(r.stdout, "Window: %s\n", record.TmuxWindow)
	fmt.Fprintf(r.stdout, "Status: Idle\n")
	return nil
}

func (r Runner) executeSessionWait(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	sessionIdentifier := strings.TrimSpace(args[0])
	eventType := ""
	waitTimeout := 30 * time.Minute

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--event":
			i++
			if i >= len(args) {
				return fmt.Errorf("--event requires a value")
			}
			eventType = strings.TrimSpace(args[i])
		case "--timeout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--timeout requires a value")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--timeout value %q is not a valid duration (e.g. 30m, 1h): %w", args[i], err)
			}
			waitTimeout = d
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if eventType == "" {
		return fmt.Errorf("--event is required\nValid events: task.completed, handoff.prepared, checkpoint.created, step.completed, task.unblocked")
	}

	record, err := r.loadSessionByIdentifier(sessionIdentifier)
	if err != nil {
		return err
	}
	if strings.TrimSpace(record.TaskID) == "" {
		return fmt.Errorf("session %q is not bound to a task; session wait requires a task-bound session", record.ID)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, record.TaskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", record.TaskID)
	}

	logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, record.TaskID, view.Worktree)

	fmt.Fprintf(r.stdout, "Waiting for event %q\n", eventType)
	fmt.Fprintf(r.stdout, "Session: %s  Task: %s\n", record.ID, record.TaskID)
	fmt.Fprintf(r.stdout, "Log: %s\n", logPath)
	fmt.Fprintf(r.stdout, "Timeout: %s\n", waitTimeout)
	fmt.Fprintln(r.stdout, "")

	line, err := waitForLogEvent(logPath, eventType, waitTimeout)
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Event detected")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Event: %s\n", eventType)
	fmt.Fprintf(r.stdout, "Entry: %s\n", line)
	return nil
}


func (r Runner) executeSessionSetAgentID(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: aom session set-agent-id <session-id> <native-session-id>")
	}
	sessID := strings.TrimSpace(args[0])
	vendorID := strings.TrimSpace(args[1])

	record, err := r.loadSessionByIdentifier(sessID)
	if err != nil {
		return err
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	updated, err := sessionService.SetVendorSessionID(record.ID, vendorID)
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Agent session ID registered")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session: %s\n", updated.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", updated.AgentName)
	fmt.Fprintf(r.stdout, "Native session ID: %s\n", updated.VendorSessionID)
	fmt.Fprintf(r.stdout, "Next spawn for this agent on task %s will resume this session.\n", emptyFallback(updated.TaskID))
	return nil
}

func (r Runner) executeSessionHealth(args []string) error {
	showAll := false
	sessionID := ""

	for _, arg := range args {
		switch arg {
		case "--all":
			showAll = true
		default:
			if sessionID == "" {
				sessionID = strings.TrimSpace(arg)
			}
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	sessionService, sessDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sessDB.Close()

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	now := time.Now()

	if sessionID == "" {
		showAll = true
	}

	if showAll {
		fmt.Fprintln(r.stdout, "Session health")
		fmt.Fprintln(r.stdout, "")
		active := 0
		for _, s := range sessions {
			switch s.Status {
			case "Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked":
			default:
				continue
			}
			active++
			logPath := r.resolveTaskLogPath(result, s.TaskID)
			h := computeSessionHealth(logPath, s.ID, now)
			warning := ""
			if h.CheckpointWarning {
				warning += " [checkpoint overdue]"
			}
			fmt.Fprintf(r.stdout, "  %s  agent=%-20s  status=%-20s  since-checkpoint=%-8s%s\n",
				s.ID, s.AgentName, s.Status, h.TimeSinceCheckpoint, warning)
		}
		if active == 0 {
			fmt.Fprintln(r.stdout, "No active sessions.")
		}
		return nil
	}

	var targetSession *session.Record
	for i := range sessions {
		if sessions[i].ID == sessionID {
			targetSession = &sessions[i]
			break
		}
	}
	if targetSession == nil {
		return fmt.Errorf("session %q not found", sessionID)
	}

	logPath := r.resolveTaskLogPath(result, targetSession.TaskID)
	h := computeSessionHealth(logPath, sessionID, now)

	fmt.Fprintf(r.stdout, "Session health: %s\n\n", sessionID)
	fmt.Fprintf(r.stdout, "Agent:               %s\n", emptyFallback(targetSession.AgentName))
	fmt.Fprintf(r.stdout, "Status:              %s\n", targetSession.Status)
	fmt.Fprintf(r.stdout, "Time since checkpoint: %s\n", h.TimeSinceCheckpoint)
	if h.CheckpointWarning {
		fmt.Fprintf(r.stdout, "Warning: context may be stale — consider: aom checkpoint %s\n", sessionID)
	}

	return nil
}

// resolveTaskLogPath returns the log.md path for a task, falling back gracefully.
func (r Runner) resolveTaskLogPath(result *project.OpenResult, taskID string) string {
	if taskID == "" {
		return ""
	}
	view, err := r.loadTaskView(result, taskID)
	if err != nil || view == nil {
		return filepath.Join(result.Project.RepoPath, result.StateDir, "tasks", taskID, "log.md")
	}
	svc := artifact.NewService(result.Project.RepoPath, result.StateDir)
	return svc.TaskLogPath(artifact.SyncParams{Task: view.Task, Worktree: view.Worktree})
}

func (r Runner) executeSessionSend(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	sessionID := strings.TrimSpace(args[0])
	var filePath string
	var msgArgs []string
	for i := 1; i < len(args); i++ {
		if args[i] == "--file" {
			i++
			if i >= len(args) {
				return fmt.Errorf("--file requires a path")
			}
			filePath = args[i]
		} else {
			msgArgs = append(msgArgs, args[i])
		}
	}

	sessionRecord, err := r.loadSessionByIdentifier(sessionID)
	if err != nil {
		return err
	}
	if !sendableSessionStatus(sessionRecord.Status) || strings.TrimSpace(sessionRecord.TmuxPane) == "" {
		return fmt.Errorf("session %q does not have a live tmux pane binding", sessionRecord.ID)
	}

	var message string
	if filePath != "" {
		var data []byte
		var err error
		if filePath == "-" || filePath == "/dev/stdin" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(filePath)
		}
		if err != nil {
			return fmt.Errorf("read --file %q: %w", filePath, err)
		}
		message = strings.TrimSpace(string(data))
		if len(msgArgs) > 0 {
			return fmt.Errorf("--file and inline message are mutually exclusive")
		}
	} else {
		message = strings.TrimSpace(strings.Join(msgArgs, " "))
	}
	if message == "" {
		return fmt.Errorf("message is required (use --file <path> or pass message directly)")
	}

	// Interpret shell-style escape sequences so callers can embed newlines with \n.
	message = interpretEscapes(message)

	// Auto-flush staged outbox messages before delivering this prompt so the
	// receiving agent sees all pending channel/mailbox entries immediately.
	if repoPath, rerr := config.FindProjectRoot("."); rerr == nil {
		if n, ferr := flushAllOutboxes(repoPath); ferr == nil && n > 0 {
			fmt.Fprintf(r.stdout, "Auto-flushed %d outbox message(s) to channel/mailbox.\n", n)
		}
	}

	if r.app.Tmux.PaneInAlternateScreen(sessionRecord.TmuxPane) {
		fmt.Fprintf(r.stdout, "Warning: pane %s is showing an interactive overlay (e.g. a permission prompt or /status view).\n", sessionRecord.TmuxPane)
		fmt.Fprintln(r.stdout, "The message will be sent but the agent may not receive it until the overlay is dismissed.")
		fmt.Fprintln(r.stdout, "Tip: press 'q' or Escape in the pane to close the overlay first.")
		fmt.Fprintln(r.stdout)
	}

	if cmd := r.app.Tmux.PaneCurrentCommand(sessionRecord.TmuxPane); isShellProcess(cmd) {
		fmt.Fprintf(r.stdout, "Warning: pane %s is running a shell (%q) — the agent runtime is not active.\n", sessionRecord.TmuxPane, cmd)
		fmt.Fprintln(r.stdout, "The message will be sent as shell input and may be interpreted as shell commands.")
		fmt.Fprintf(r.stdout, "Tip: run \"aom session spawn %s --real\" (or \"aom session replace %s --agent %s\") to start the agent first.\n",
			sessionRecord.AgentName, sessionRecord.ID, sessionRecord.AgentName)
		fmt.Fprintln(r.stdout)
	}

	if err := r.app.Tmux.SendKeys(sessionRecord.TmuxPane, message); err != nil {
		return err
	}

	if strings.TrimSpace(sessionRecord.TaskID) != "" {
		result, err := r.app.Projects.Open(".")
		if err != nil {
			return err
		}

		actor := "operator"
		if env := strings.TrimSpace(os.Getenv("AOM_ACTOR")); env != "" {
			actor = env
		}

		if err := r.syncTaskArtifactsWithSession(result, sessionRecord.TaskID, artifact.Event{
			Type:        "orchestrator.prompt",
			Actor:       actor,
			SessionID:   sessionRecord.ID,
			Summary:     fmt.Sprintf("Prompt delivered to session %s: %s", sessionRecord.ID, briefSummary(message)),
			StateEffect: "Session Prompted",
		}, false, sessionRecord); err != nil {
			return err
		}
	}

	fmt.Fprintln(r.stdout, "Prompt delivered")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session: %s\n", sessionRecord.ID)
	fmt.Fprintf(r.stdout, "Pane: %s\n", sessionRecord.TmuxPane)
	fmt.Fprintf(r.stdout, "Message: %s\n", message)
	return nil
}

// executeSessionCleanup scans /tmp for stale aom-policy-* directories and
// capture state files whose sessions are no longer active, terminates any
// lingering wrapper processes, and removes the directories. Accepts an
// optional --stale flag (default behaviour) and --dry-run to report without
// removing anything.
func (r Runner) executeSessionCleanup(args []string) error {
	dryRun := false
	for _, arg := range args {
		switch arg {
		case "--stale":
			// default — accepted for script friendliness
		case "--dry-run":
			dryRun = true
		default:
			return fmt.Errorf("unknown flag %q (usage: aom session cleanup [--stale] [--dry-run])", arg)
		}
	}

	// Determine which sessions are still active (live pane or non-terminal status).
	activeIDs := make(map[string]bool)
	if result, err := r.app.Projects.Open("."); err == nil {
		if sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath); err == nil {
			defer sqlDB.Close()
			if sessions, err := sessionService.ListByProject(result.Project.ID); err == nil {
				for _, s := range sessions {
					switch s.Status {
					case "Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked", "NeedsAttention":
						activeIDs[s.ID] = true
					}
				}
			}
		}
	}

	staleDirs, err := provider.ScanStalePolicyDirs(activeIDs)
	if err != nil {
		return fmt.Errorf("scan stale policy dirs: %w", err)
	}

	if len(staleDirs) == 0 {
		fmt.Fprintln(r.stdout, "Session cleanup")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintln(r.stdout, "No stale policy dirs found.")
		return nil
	}

	fmt.Fprintln(r.stdout, "Session cleanup")
	fmt.Fprintln(r.stdout, "")
	if dryRun {
		fmt.Fprintf(r.stdout, "Dry run — %d stale policy dir(s) would be removed:\n", len(staleDirs))
	} else {
		fmt.Fprintf(r.stdout, "Removing %d stale policy dir(s):\n", len(staleDirs))
	}
	fmt.Fprintln(r.stdout, "")

	cleaned, failed := 0, 0
	for _, dir := range staleDirs {
		if dryRun {
			fmt.Fprintf(r.stdout, "  would remove: %s\n", dir)
			cleaned++
			continue
		}
		if cleanErr := provider.CleanupStaleDir(dir); cleanErr != nil {
			fmt.Fprintf(r.stdout, "  failed: %s (%v)\n", dir, cleanErr)
			failed++
		} else {
			fmt.Fprintf(r.stdout, "  removed: %s\n", dir)
			cleaned++
		}
	}

	fmt.Fprintln(r.stdout, "")
	if dryRun {
		fmt.Fprintf(r.stdout, "Dry run complete: %d dir(s) identified\n", cleaned)
		fmt.Fprintln(r.stdout, "Run without --dry-run to remove them.")
	} else {
		fmt.Fprintf(r.stdout, "Cleaned: %d  Failed: %d\n", cleaned, failed)
		if failed > 0 {
			return fmt.Errorf("session cleanup: %d dir(s) could not be removed", failed)
		}
	}
	return nil
}

// executeSessionRecover diagnoses a stopped or failed session and recommends
// the appropriate recovery action based on pane liveness, native session ID
// availability, and task artifact continuity.
func (r Runner) executeSessionRecover(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("session recover takes exactly one argument")
	}

	record, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// Tmux pane liveness.
	paneAlive := false
	if r.app.Tmux.Availability().Available && strings.TrimSpace(record.TmuxPane) != "" {
		paneAlive, _ = r.app.Tmux.PaneExists(record.TmuxPane)
	}

	// Runtime binary availability.
	runtimeAvail := "not found in PATH"
	if _, lookErr := exec.LookPath(record.Runtime); lookErr == nil {
		runtimeAvail = "available"
	}

	// Task and worktree continuity.
	taskSummary := "(none)"
	worktreePath := "(none)"
	if record.TaskID != "" {
		view, viewErr := r.loadTaskView(result, record.TaskID)
		if viewErr == nil && view != nil {
			taskSummary = fmt.Sprintf("%s (%s)", record.TaskID, view.Task.Status)
			if view.Worktree != nil {
				worktreePath = view.Worktree.WorktreePath
			}
		}
	}

	// Determine continuity quality and recommended action.
	var quality, action string
	switch {
	case paneAlive:
		quality = "high — pane still alive"
		action = fmt.Sprintf("aom session rebind %s", record.ID)
	case record.VendorSessionID != "":
		quality = "medium — native session ID available for resume"
		action = fmt.Sprintf("aom session replace %s --agent %s --real", record.ID, record.AgentName)
	case record.TaskID != "":
		quality = "artifact-backed — task.md and log.md preserve context"
		action = fmt.Sprintf("aom session spawn %s --task %s --real", record.AgentName, record.TaskID)
	default:
		quality = "low — no task bound and no native session ID"
		action = fmt.Sprintf("aom session archive %s", record.ID)
	}

	fmt.Fprintln(r.stdout, "Session recovery assessment")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session:          %s\n", record.ID)
	fmt.Fprintf(r.stdout, "Agent:            %s\n", record.AgentName)
	fmt.Fprintf(r.stdout, "Status:           %s\n", record.Status)
	fmt.Fprintf(r.stdout, "Runtime:          %s (%s)\n", record.Runtime, runtimeAvail)
	fmt.Fprintf(r.stdout, "Task:             %s\n", taskSummary)
	fmt.Fprintf(r.stdout, "Worktree:         %s\n", worktreePath)
	fmt.Fprintf(r.stdout, "Tmux pane:        %s\n", emptyFallback(record.TmuxPane))
	fmt.Fprintf(r.stdout, "Pane alive:       %v\n", paneAlive)
	fmt.Fprintf(r.stdout, "Native session:   %s\n", emptyFallback(record.VendorSessionID))
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Continuity:       %s\n", quality)
	fmt.Fprintf(r.stdout, "Recommended:      %s\n", action)
	return nil
}
