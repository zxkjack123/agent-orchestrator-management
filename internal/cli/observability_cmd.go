package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/artifact"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
	aomruntime "github.com/lattapon-aek/agent-orchestrator-management/internal/runtime"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/task"
)

func (r Runner) executeNext(args []string) error {
	var format string
	for i := 0; i < len(args); i++ {
		switch args[i] {
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

	unblocked, err := taskService.ListUnblocked(result.Project.ID)
	if err != nil {
		return err
	}

	all, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	// Collect blocked tasks (have at least one active blocker).
	unblockedIDs := make(map[string]bool)
	for _, t := range unblocked {
		unblockedIDs[t.ID] = true
	}

	var blocked []task.Record
	for _, t := range all {
		if t.Status == "Done" || t.Status == "Archived" {
			continue
		}
		if !unblockedIDs[t.ID] {
			blocked = append(blocked, t)
		}
	}

	if format == "json" {
		type taskEntry struct {
			ID             string   `json:"id"`
			Title          string   `json:"title"`
			Status         string   `json:"status"`
			Priority       string   `json:"priority"`
			PreferredRole  string   `json:"preferred_role,omitempty"`
			PreferredAgent string   `json:"preferred_agent,omitempty"`
			BlockedBy      []string `json:"blocked_by,omitempty"`
		}
		toEntry := func(t task.Record) taskEntry {
			blockers, _ := taskService.BlockedBy(t.ID)
			var ids []string
			for _, b := range blockers {
				if b.Status != "Done" && b.Status != "Archived" {
					ids = append(ids, b.ID)
				}
			}
			return taskEntry{
				ID:             t.ID,
				Title:          t.Title,
				Status:         t.Status,
				Priority:       task.PriorityLabel(t.Priority),
				PreferredRole:  t.PreferredRole,
				PreferredAgent: t.PreferredAgent,
				BlockedBy:      ids,
			}
		}
		unblockedEntries := make([]taskEntry, 0, len(unblocked))
		for _, t := range unblocked {
			unblockedEntries = append(unblockedEntries, toEntry(t))
		}
		blockedEntries := make([]taskEntry, 0, len(blocked))
		for _, t := range blocked {
			blockedEntries = append(blockedEntries, toEntry(t))
		}
		enc := json.NewEncoder(r.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"unblocked": unblockedEntries,
			"blocked":   blockedEntries,
		})
	}

	fmt.Fprintln(r.stdout, "Next tasks")
	fmt.Fprintln(r.stdout, "")

	if len(unblocked) == 0 {
		fmt.Fprintln(r.stdout, "Unblocked: none")
	} else {
		fmt.Fprintln(r.stdout, "Unblocked (work on these next):")
		for i, t := range unblocked {
			priority := task.PriorityLabel(t.Priority)
			owner := emptyFallback(t.PreferredAgent)
			if owner == "-" {
				owner = emptyFallback(t.PreferredRole)
			}
			fmt.Fprintf(r.stdout, "  %d. [%s] %s  %s  owner=%s\n", i+1, priority, t.ID, t.Title, owner)
		}
	}

	if len(blocked) > 0 {
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintln(r.stdout, "Blocked (waiting on dependencies):")
		for _, t := range blocked {
			blockers, _ := taskService.BlockedBy(t.ID)
			blockerIDs := make([]string, 0, len(blockers))
			for _, b := range blockers {
				if b.Status != "Done" && b.Status != "Archived" {
					blockerIDs = append(blockerIDs, b.ID)
				}
			}
			fmt.Fprintf(r.stdout, "  %s  %s  waiting on: %s\n", t.ID, t.Title, strings.Join(blockerIDs, ", "))
		}
	}

	return nil
}


func (r Runner) executeWatch(args []string) error {
	taskID := ""
	eventType := ""
	watchTimeout := 30 * time.Minute

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			taskID = strings.TrimSpace(args[i])
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
				return fmt.Errorf("--timeout value %q is not a valid duration (e.g. 30m, 2h): %w", args[i], err)
			}
			watchTimeout = d
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	if taskID != "" {
		return r.executeWatchSingleTask(result, taskID, eventType, watchTimeout)
	}

	return r.executeWatchAllTasks(result, eventType, watchTimeout)
}

// executeWatchSingleTask is the original single-task watch path.
func (r Runner) executeWatchSingleTask(result *project.OpenResult, taskID, eventType string, watchTimeout time.Duration) error {
	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, taskID, view.Worktree)

	if eventType != "" {
		fmt.Fprintf(r.stdout, "Watching for event %q\n", eventType)
		fmt.Fprintf(r.stdout, "Task: %s\n", taskID)
		fmt.Fprintf(r.stdout, "Log: %s\n", logPath)
		fmt.Fprintf(r.stdout, "Timeout: %s\n\n", watchTimeout)

		line, err := waitForLogEvent(logPath, eventType, watchTimeout)
		if err != nil {
			if strings.Contains(err.Error(), "timed out") {
				fmt.Fprintf(r.stdout, "Watch timeout reached (%s) — event %q not detected.\n", watchTimeout, eventType)
				return nil
			}
			return err
		}

		fmt.Fprintln(r.stdout, "Event detected")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "Event: %s\n", eventType)
		fmt.Fprintf(r.stdout, "Entry: %s\n", line)
		return nil
	}

	fmt.Fprintf(r.stdout, "Watching task %s (tail mode, timeout %s)\n", taskID, watchTimeout)
	fmt.Fprintf(r.stdout, "Log: %s\n\n", logPath)

	return tailLogEvents(r.stdout, logPath, watchTimeout)
}

// executeWatchAllTasks watches all active tasks simultaneously.
func (r Runner) executeWatchAllTasks(result *project.OpenResult, eventType string, watchTimeout time.Duration) error {
	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	allTasks, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer worktreeDB.Close()

	activeStatuses := map[string]bool{
		"InProgress":     true,
		"Blocked":        true,
		"NeedsAttention": true,
		"Ready":          true,
	}

	var entries []taskLogEntry
	for _, t := range allTasks {
		if !activeStatuses[t.Status] {
			continue
		}
		mapping, _ := worktreeService.GetByTask(t.ID)
		logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, t.ID, mapping)
		entries = append(entries, taskLogEntry{TaskID: t.ID, LogPath: logPath})
	}

	if len(entries) == 0 {
		fmt.Fprintf(r.stdout, "No active tasks found. Polling until tasks become active (timeout %s)...\n\n", watchTimeout)
		entries = r.waitForActiveTasks(result, watchTimeout)
		if len(entries) == 0 {
			fmt.Fprintln(r.stdout, "No active tasks appeared within timeout.")
			return nil
		}
		fmt.Fprintf(r.stdout, "%d active task(s) detected. Starting stream...\n\n", len(entries))
	}

	if eventType != "" {
		fmt.Fprintf(r.stdout, "Watching %d active task(s) for event %q (timeout %s)\n", len(entries), eventType, watchTimeout)
		for _, e := range entries {
			fmt.Fprintf(r.stdout, "  %s → %s\n", e.TaskID, e.LogPath)
		}
		fmt.Fprintln(r.stdout, "")

		matchedTask, matchedLine, err := waitForMultiTaskLogEvent(entries, eventType, watchTimeout)
		if err != nil {
			// Timeout is expected; print a note and exit 0 rather than propagating the error.
			if strings.Contains(err.Error(), "timed out") {
				fmt.Fprintf(r.stdout, "Watch timeout reached (%s) — event %q not detected.\n", watchTimeout, eventType)
				return nil
			}
			return err
		}

		fmt.Fprintln(r.stdout, "Event detected")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "Task: %s\n", matchedTask)
		fmt.Fprintf(r.stdout, "Event: %s\n", eventType)
		fmt.Fprintf(r.stdout, "Entry: %s\n", matchedLine)
		return nil
	}

	fmt.Fprintf(r.stdout, "Watching %d active task(s) (tail mode, timeout %s)\n", len(entries), watchTimeout)
	for _, e := range entries {
		fmt.Fprintf(r.stdout, "  %s → %s\n", e.TaskID, e.LogPath)
	}
	fmt.Fprintln(r.stdout, "")

	return tailMultiTaskLogEvents(r.stdout, entries, watchTimeout)
}

// waitForActiveTasks polls every 5 seconds until at least one active task is found or
// the timeout elapses. Returns the entries slice (may be empty on timeout).
func (r Runner) waitForActiveTasks(result *project.OpenResult, timeout time.Duration) []taskLogEntry {
	activeStatuses := map[string]bool{
		"InProgress":     true,
		"Blocked":        true,
		"NeedsAttention": true,
		"Ready":          true,
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(5 * time.Second)

		taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
		if err != nil {
			continue
		}
		allTasks, err := taskService.ListByProject(result.Project.ID)
		taskDB.Close()
		if err != nil {
			continue
		}

		worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
		if err != nil {
			continue
		}
		var entries []taskLogEntry
		for _, t := range allTasks {
			if !activeStatuses[t.Status] {
				continue
			}
			mapping, _ := worktreeService.GetByTask(t.ID)
			logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, t.ID, mapping)
			entries = append(entries, taskLogEntry{TaskID: t.ID, LogPath: logPath})
		}
		worktreeDB.Close()

		if len(entries) > 0 {
			return entries
		}
	}
	return nil
}

// ── M14: task request / team brief ──────────────────────────────────────────

func (r Runner) executeTeam(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("team subcommand is required (try: team brief, team roster, team view)")
	}

	switch args[0] {
	case "brief":
		return r.executeTeamBrief(args[1:])
	case "roster":
		return r.executeTeamRoster(args[1:])
	case "view":
		return r.executeTeamView(args[1:])
	default:
		return fmt.Errorf("unknown team command %q", args[0])
	}
}

// executeTeamView attaches the operator to the shared team tmux window where all
// grid-mode agent panes live side-by-side. The window is created if it does not
// yet exist (empty). Use aom orchestrate to populate it.
//
// Usage: aom team view [--layout tiled|even-horizontal|even-vertical]
func (r Runner) executeTeamView(args []string) error {
	layout := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--layout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--layout requires a value")
			}
			layout = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	workspace, err := r.app.Tmux.EnsureWorkspace(result.SessionPrefix, result.Project.RepoPath)
	if err != nil {
		return fmt.Errorf("ensure tmux workspace: %w", err)
	}

	const teamWindowName = "team"
	windowTarget, _, err := r.app.Tmux.EnsureTeamWindow(workspace.Target, teamWindowName)
	if err != nil {
		return fmt.Errorf("ensure team window: %w", err)
	}

	if layout != "" {
		if err := r.app.Tmux.SelectLayout(windowTarget, layout); err != nil {
			fmt.Fprintf(r.stderr, "warning: select-layout %q: %v\n", layout, err)
		}
	}

	fmt.Fprintf(r.stdout, "Attaching to team window in session %s\n", workspace.Target)
	fmt.Fprintf(r.stdout, "Use Ctrl+B then arrow keys to navigate panes.\n")

	return r.app.Tmux.AttachWindow(workspace.Target, windowTarget)
}

func (r Runner) executeTeamBrief(args []string) error {
	_ = args

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	allTasks, err := taskService.ListByProject(result.Project.ID)
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

	// Build session status index by task.
	taskSessionStatus := make(map[string]string)
	for _, s := range sessions {
		if s.TaskID != "" {
			taskSessionStatus[s.TaskID] = s.Status
		}
	}

	// Build agent session status index.
	agentSessionStatus := make(map[string]string)
	for _, s := range sessions {
		if s.AgentName != "" {
			agentSessionStatus[s.AgentName] = s.Status
		}
	}

	var briefTasks []artifact.TeamBriefTask
	for _, t := range allTasks {
		if t.Status == "Done" || t.Status == "Archived" {
			continue
		}
		blockers, _ := taskService.BlockedBy(t.ID)
		blockerIDs := make([]string, 0, len(blockers))
		for _, b := range blockers {
			if b.Status != "Done" && b.Status != "Archived" {
				blockerIDs = append(blockerIDs, b.ID)
			}
		}
		owner := t.PreferredAgent
		if owner == "" {
			owner = t.PreferredRole
		}
		briefTasks = append(briefTasks, artifact.TeamBriefTask{
			ID:        t.ID,
			Title:     t.Title,
			Status:    t.Status,
			Priority:  task.PriorityLabel(t.Priority),
			Agent:     owner,
			BlockedBy: blockerIDs,
		})
	}

	// Pending requests.
	pendingReqs, _ := readPendingRequests(result.Project.RepoPath)
	var reqLines []string
	for _, req := range pendingReqs {
		reqLines = append(reqLines, fmt.Sprintf("%s: \"%s\" from %s [%s]",
			req.ID, req.Title, emptyFallback(req.RequestedBy), req.Priority))
	}

	// Last 5 channel messages (raw lines from channel.md).
	channelTail := lastChannelMessages(result.Project.RepoPath, 5)

	// Agents from config.
	var briefAgents []artifact.TeamBriefAgent
	for _, a := range result.Agents {
		status := agentSessionStatus[a.Name]
		briefAgents = append(briefAgents, artifact.TeamBriefAgent{
			Name:          a.Name,
			Role:          a.Role,
			Runtime:       a.Runtime,
			SessionStatus: status,
		})
	}

	svc := artifact.NewService(result.Project.RepoPath, result.StateDir)
	briefPath, err := svc.GenerateTeamBrief(artifact.TeamBriefParams{
		ProjectName:     result.Project.Name,
		Tasks:           briefTasks,
		PendingRequests: reqLines,
		ChannelTail:     channelTail,
		Agents:          briefAgents,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Team brief generated\n\n")
	fmt.Fprintf(r.stdout, "Path:    %s\n", briefPath)
	fmt.Fprintf(r.stdout, "Tasks:   %d active\n", len(briefTasks))
	fmt.Fprintf(r.stdout, "Pending requests: %d\n", len(pendingReqs))
	return nil
}

// executeTeamRoster refreshes .agent/team-roster.md in the current working directory
// (the agent's worktree) with a live snapshot of the team, session statuses, and
// dependency graph. Agents call this mid-session to get an up-to-date team view.
//
// Usage: aom team roster [--agent <name>]
// When --agent is omitted, the agent name is read from the AOM_ACTOR env var.
func (r Runner) executeTeamRoster(args []string) error {
	agentName := ""
	for i := 0; i < len(args); i++ {
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

	// Fall back to AOM_ACTOR environment variable.
	if agentName == "" {
		agentName = strings.TrimSpace(os.Getenv("AOM_ACTOR"))
	}
	if agentName == "" {
		return fmt.Errorf("--agent <name> is required (or set AOM_ACTOR env var)")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	agentRecord, err := findAgent(result.Agents, agentName)
	if err != nil {
		return fmt.Errorf("agent %q not found in project config: %w", agentName, err)
	}

	// Write the roster file into .agent/ in the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	content := r.buildTeamRosterFileContent(result, agentRecord)
	if content == "" {
		fmt.Fprintln(r.stdout, "No team data available.")
		return nil
	}

	agentDir := filepath.Join(cwd, ".agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return fmt.Errorf("create .agent dir: %w", err)
	}
	rosterPath := filepath.Join(agentDir, "team-roster.md")
	if err := os.WriteFile(rosterPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write team-roster.md: %w", err)
	}

	fmt.Fprintf(r.stdout, "Team roster updated\n\n")
	fmt.Fprintf(r.stdout, "Path:  %s\n", rosterPath)
	fmt.Fprintf(r.stdout, "Agent: %s\n", agentName)
	fmt.Fprintf(r.stdout, "\nRead it: cat .agent/team-roster.md\n")
	return nil
}

func (r Runner) executeEvents(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("events subcommand is required: tail")
	}
	switch args[0] {
	case "tail":
		return r.executeEventsTail(args[1:])
	default:
		return fmt.Errorf("unknown events command %q", args[0])
	}
}

// executeEventsTail streams new log.md events for a task to stdout as they
// appear, polling every 2 seconds. Requires --task <id> or AOM_ACTOR env var
// to auto-detect the current agent's active task.
func (r Runner) executeEventsTail(args []string) error {
	taskID := ""
	tailTimeout := 30 * time.Minute

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			taskID = strings.TrimSpace(args[i])
		case "--timeout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--timeout requires a value")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--timeout value %q is not a valid duration: %w", args[i], err)
			}
			tailTimeout = d
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// Auto-detect task from AOM_ACTOR when --task is not provided.
	if taskID == "" {
		actorName := strings.TrimSpace(os.Getenv("AOM_ACTOR"))
		if actorName == "" {
			return fmt.Errorf("--task is required (or set AOM_ACTOR env var to auto-detect)")
		}
		sessions, sessErr := r.loadProjectSessions(result)
		if sessErr != nil {
			return sessErr
		}
		for _, s := range sessions {
			if s.AgentName == actorName && s.TaskID != "" {
				taskID = s.TaskID
				break
			}
		}
		if taskID == "" {
			return fmt.Errorf("no active task found for agent %q — use --task <id>", actorName)
		}
	}

	view, viewErr := r.loadTaskView(result, taskID)
	if viewErr != nil {
		return viewErr
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, taskID, view.Worktree)

	fmt.Fprintf(r.stdout, "Tailing events for task %s (timeout: %s)\n", taskID, tailTimeout)
	fmt.Fprintf(r.stdout, "Log: %s\n\n", logPath)

	return tailLogEvents(r.stdout, logPath, tailTimeout)
}

// executeOrchestrate spawns all enabled agents into the shared team tmux window
// and applies a grid layout so the operator can watch all agents at once.
//
// Usage: aom orchestrate [--layout tiled|even-horizontal|even-vertical] [--mock] [--allow-collision]
//
// Only agents that do not already have a live session are spawned. Agents that
// already have a live pane are skipped to avoid duplication. After spawning,
// the operator is attached to the team window automatically.
func (r Runner) executeOrchestrate(args []string) error {
	layout := "tiled"
	gridLayout := ""
	launchMode := aomruntime.LaunchModePlaceholder
	allowCollision := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--layout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--layout requires a value")
			}
			gridLayout = strings.TrimSpace(args[i])
		case "--mock":
			launchMode = aomruntime.LaunchModeMock
		case "--real":
			launchMode = aomruntime.LaunchModeReal
		case "--allow-collision":
			allowCollision = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if gridLayout != "" {
		layout = gridLayout
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	workspace, err := r.app.Tmux.EnsureWorkspace(result.SessionPrefix, result.Project.RepoPath)
	if err != nil {
		return fmt.Errorf("ensure tmux workspace: %w", err)
	}

	// Inject AOM_BIN so spawned agents inherit the correct binary path.
	if selfBinEarly, binErr := os.Executable(); binErr == nil && selfBinEarly != "" {
		_ = r.app.Tmux.SetSessionEnv(workspace.Target, "AOM_BIN", selfBinEarly)
	}

	enabled := make([]string, 0, len(result.Agents))
	for _, a := range result.Agents {
		if a.Enabled {
			enabled = append(enabled, a.Name)
		}
	}
	if len(enabled) == 0 {
		return fmt.Errorf("no enabled agents found — add agents to agents.yaml first")
	}

	// Resolve the team window early so we can check whether live panes are
	// already inside it. This prevents falsely skipping an agent whose pane is
	// alive in a *solo* window — it still needs to be spawned into the team grid.
	const teamWindowName = "team"
	teamWindowTarget, blankPaneEarly, teamErr := r.app.Tmux.EnsureTeamWindow(workspace.Target, teamWindowName)
	if teamErr != nil {
		return fmt.Errorf("ensure team window: %w", teamErr)
	}
	// Kill the initial blank pane created by EnsureTeamWindow (only if this is a
	// brand-new window; blankPaneEarly is empty when the window already existed).
	if blankPaneEarly != "" {
		_ = r.app.Tmux.KillPane(blankPaneEarly)
	}

	// Build set of pane IDs currently inside the team window.
	teamPaneSet := make(map[string]bool)
	if panes, pErr := r.app.Tmux.ListPanesInWindow(teamWindowTarget); pErr == nil {
		for _, p := range panes {
			teamPaneSet[p] = true
		}
	}

	// Collect live sessions — only mark an agent as "already in team" when its
	// pane is confirmed to be inside the team window. A pane alive in a solo
	// window is not in the team grid and must be (re-)spawned.
	inTeam := make(map[string]bool)   // agentName → pane confirmed in team window
	inTeamPane := make(map[string]string) // agentName → pane ID
	sessions, _ := r.loadProjectSessions(result)
	for _, s := range sessions {
		if strings.TrimSpace(s.TmuxPane) == "" {
			continue
		}
		if alive, _ := r.app.Tmux.PaneExists(s.TmuxPane); alive && teamPaneSet[s.TmuxPane] {
			inTeam[s.AgentName] = true
			inTeamPane[s.AgentName] = s.TmuxPane
		}
	}

	fmt.Fprintf(r.stdout, "Orchestrating %d agent(s) into team window (layout: %s)\n\n", len(enabled), layout)

	spawned := 0
	skipped := 0
	var firstAgentPane string
	// Track which agents are confirmed in the team window (for cleanup).
	confirmedInTeam := make(map[string]bool)
	for _, name := range enabled {
		if inTeam[name] {
			fmt.Fprintf(r.stdout, "  %-24s skipped (already in team window)\n", name)
			skipped++
			confirmedInTeam[name] = true
			if firstAgentPane == "" {
				firstAgentPane = inTeamPane[name]
			}
			continue
		}
		agentRecord, aErr := findAgent(result.Agents, name)
		if aErr != nil {
			fmt.Fprintf(r.stdout, "  %-24s error: %v\n", name, aErr)
			continue
		}
		rec, spawnErr := r.executeResolvedSessionSpawn(result, agentRecord, sessionSpawnParams{
			agentName:      name,
			launchMode:     launchMode,
			allowCollision: allowCollision,
			gridMode:       true,
			gridLayout:     layout,
		})
		if spawnErr != nil {
			fmt.Fprintf(r.stdout, "  %-24s spawn failed: %v\n", name, spawnErr)
			continue
		}
		fmt.Fprintf(r.stdout, "  %-24s spawned\n", name)
		spawned++
		confirmedInTeam[name] = true
		if firstAgentPane == "" && rec != nil && rec.TmuxPane != "" {
			firstAgentPane = rec.TmuxPane
		}
	}

	fmt.Fprintf(r.stdout, "\nSpawned: %d  Skipped: %d\n", spawned, skipped)

	if spawned == 0 && skipped == 0 {
		return fmt.Errorf("all agents failed to spawn — check the errors above")
	}

	_ = r.app.Tmux.SelectLayout(teamWindowTarget, layout)
	if firstAgentPane != "" {
		_ = r.app.Tmux.FocusPane(firstAgentPane)
	}

	// Remove stale solo windows ONLY for agents confirmed in the team window.
	// Agents that failed to spawn are left untouched so their existing pane
	// (if any) survives. In iTerm2 -CC mode each tmux window is a separate
	// native window, so solo ghosts must be cleaned up to keep the view tidy.
	teamWindowID := teamWindowTarget
	if idx := strings.LastIndex(teamWindowTarget, ":"); idx >= 0 {
		teamWindowID = teamWindowTarget[idx+1:]
	}
	if windows, listErr := r.app.Tmux.ListWindowsInSession(workspace.Target); listErr == nil {
		for _, w := range windows {
			if w.ID == teamWindowID {
				continue // never kill the team window itself
			}
			if confirmedInTeam[w.Name] {
				_ = r.app.Tmux.KillWindow(workspace.Target + ":" + w.ID)
			}
		}
	}

	fmt.Fprintf(r.stdout, "\nAttaching to team window (Ctrl+B then arrow keys to navigate panes)...\n")
	if err := r.app.Tmux.AttachWindow(workspace.Target, teamWindowTarget); err != nil {
		fmt.Fprintf(r.stderr, "note: could not auto-attach: %v\n", err)
		fmt.Fprintf(r.stdout, "To view the team window manually: aom team view\n")
	}

	return nil
}
